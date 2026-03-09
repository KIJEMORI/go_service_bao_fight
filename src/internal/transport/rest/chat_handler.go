package rest

import (
	"context"
	"net/http"
	"project/internal/models"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *Handler) GetChatParticipants(ctx context.Context, chatID uuid.UUID) []string {
	cacheKey := "chat_members:" + chatID.String()

	// Пытаемся взять из Redis (используем Set для уникальности)
	// Команда SMEMBERS возвращает все элементы множества
	participants, err := h.Redis.SMembers(ctx, cacheKey).Result()
	if err == nil && len(participants) > 0 {
		return participants
	}

	// 2. Если в Redis пусто — идем в Postgres
	var members []uuid.UUID
	err = h.DB.WithContext(ctx).Model(&models.ChatMember{}).
		Where("chat_id = ?", chatID).
		Pluck("user_id", &members).Error // Pluck выбирает только одну колонку в слайс

	if err != nil {
		h.Logger.Error("Failed to fetch members from DB", zap.Error(err))
		return nil
	}

	// 3. Наполняем Redis, чтобы в следующий раз было быстро
	// Устанавливаем TTL (например, 1 час), чтобы данные не протухли навсегда
	strMembers := make([]interface{}, len(members))
	resultIds := make([]string, len(members))

	for i, m := range members {
		strMembers[i] = m.String()
		resultIds[i] = m.String()
	}

	if len(strMembers) > 0 {
		h.Redis.SAdd(ctx, cacheKey, strMembers...)
		h.Redis.Expire(ctx, cacheKey, time.Hour)
	}

	return resultIds
}

func (h *Handler) GetChatHistory(c echo.Context) error {
	ctx := c.Request().Context()

	// Получаем ID текущего пользователя из JWT (который положила мидлвара)
	userVal := c.Get("user")

	currentUserID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	// Получаем ID собеседника из параметров запроса
	otherUserID := c.QueryParam("with_user_id")
	if otherUserID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "with_user_id is required"})
	}

	// Параметры пагинации
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = min(100, max(50, limit))
	}

	beforeID := c.QueryParam("before_id")

	var messages []models.Message
	query := h.DB.WithContext(ctx).
		Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			currentUserID, otherUserID, otherUserID, currentUserID)

	if beforeID != "" {
		// Находим время создания сообщения-курсора
		var anchorMessage models.Message
		if err := h.DB.Select("created_at").Where("id = ?", beforeID).First(&anchorMessage).Error; err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid cursor"})
		}
		query = query.Where("created_at < ?", anchorMessage.CreatedAt)
	}

	if err := query.Order("created_at DESC").Limit(limit).Find(&messages).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	userIDs := make(map[string]struct{})
	for _, m := range messages {
		userIDs[m.SenderID.String()] = struct{}{}
	}

	uniqueIDs := make([]string, 0, len(userIDs))
	for id := range userIDs {
		uniqueIDs = append(uniqueIDs, id)
	}

	var profiles []models.Profile
	h.DB.Where("user_id IN ?", uniqueIDs).Find(&profiles)

	var nextCursor string
	if len(messages) > 0 {
		nextCursor = messages[len(messages)-1].ID.String()
	}

	profileMap := make(map[string]models.Profile)
	for _, p := range profiles {
		profileMap[p.UserID.String()] = p
	}

	var hasMore bool
	if len(messages) > 0 {
		// Проверяем, есть ли хоть одно сообщение старше самого старого в текущем батче
		lastMsg := messages[len(messages)-1]
		var count int64
		h.DB.Model(&models.Message{}).
			Where("((sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)) AND created_at < ?",
				currentUserID, otherUserID, otherUserID, currentUserID, lastMsg.CreatedAt).
			Limit(1).
			Count(&count)
		hasMore = count > 0
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"messages": messages,
		"meta": map[string]interface{}{
			"limit":       limit,
			"next_cursor": nextCursor,
			"has_more":    hasMore, // Подсказка фронту: "скроллить дальше?"
		},
	})
}

func (h *Handler) JoinChat(c echo.Context) error {
	// Получаем текущего пользователя
	userVal := c.Get("user")

	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	uID, _ := uuid.Parse(userID)

	// Получаем ID чата из запроса
	var input struct {
		ChatID string `json:"chat_id" validate:"required"`
	}
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	cID, err := uuid.Parse(input.ChatID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid chat_id"})
	}

	// Проверяем существование чата (опционально, но желательно)
	var chat models.Chat
	if err := h.DB.First(&chat, "id = ?", cID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "chat not found"})
	}

	// Записываем в БД
	member := models.ChatMember{
		ChatID: cID,
		UserID: uID,
	}

	if err := h.DB.Create(&member).Error; err != nil {
		// Если уже состоит в чате (ошибка уникальности ключа)
		return c.JSON(http.StatusConflict, map[string]string{"error": "already a member"})
	}

	// ИНВАЛИДАЦИЯ КЭША (Самый важный момент для Highload)
	// Удаляем список участников из Redis, чтобы GetChatParticipants пересобрал его
	cacheKey := "chat_members:" + cID.String()
	h.Redis.Del(c.Request().Context(), cacheKey)

	// Опционально: Отправляем системное сообщение в Kafka "User X joined the chat"

	return c.JSON(http.StatusOK, map[string]string{"status": "joined"})
}
