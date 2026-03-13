package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func (h *Handler) GetChatParticipants(ctx context.Context, chatID uuid.UUID) []string {
	cacheKey := "chat_members:" + chatID.String()

	// Пытаемся взять из redis (используем Set для уникальности)
	// Команда SMEMBERS возвращает все элементы множества
	participants, err := h.redis.SMembers(ctx, cacheKey).Result()
	if err == nil && len(participants) > 0 {
		return participants
	}

	// Если в redis пусто — идем в Postgres
	var members []uuid.UUID
	err = h.db.WithContext(ctx).Model(&models.ChatMember{}).
		Where("chat_id = ?", chatID).
		Pluck("user_id", &members).Error // Pluck выбирает только одну колонку в слайс

	if err != nil {
		h.logger.Error("Failed to fetch members from db", zap.Error(err))
		return nil
	}

	// Наполняем redis, чтобы в следующий раз было быстро
	// Устанавливаем TTL (например, 1 час), чтобы данные не протухли навсегда
	strMembers := make([]interface{}, len(members))
	resultIds := make([]string, len(members))

	for i, m := range members {
		strMembers[i] = m.String()
		resultIds[i] = m.String()
	}

	if len(strMembers) > 0 {
		h.redis.SAdd(ctx, cacheKey, strMembers...)
		h.redis.Expire(ctx, cacheKey, time.Hour)
	}

	return resultIds
}
func (h *Handler) StartChatWatcher(ctx context.Context, broker string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   kafka_topics.ChatMessagesTopic.String(),
		GroupID: "ws-notifier-" + uuid.New().String(), // У каждого пода свой ID, чтобы все видели всё
	})

	defer reader.Close()

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			return
		}

		var msg models.Message
		if err := json.Unmarshal(m.Value, &msg); err != nil {
			continue // Пропускаем битые сообщения
		}
		participants := h.GetChatParticipants(ctx, msg.ChatID)

		event := map[string]interface{}{
			"type":    "chat_activity", // Универсальный тип
			"chat_id": msg.ChatID,
			"message": msg, // Все данные сообщения для отрисовки в чате
			"inbox": map[string]any{ // Краткие данные для обновления списка чатов
				"preview":    msg.Text,
				"updated_at": msg.CreatedAt,
			},
		}
		// Ищем чат в нашем хабе
		for _, userID := range participants {
			if conns, ok := h.hub.Clients[userID]; ok {
				for _, conn := range conns {
					conn.WriteJSON(event)
				}
			}
		}
		h.hub.Mu.RUnlock()
	}
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
	chatID := c.QueryParam("chat_id")
	if chatID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "chat_id is required"})
	}

	// Параметры пагинации
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = min(100, max(50, limit))
	}

	beforeID := c.QueryParam("before_id")

	var messages []models.Message
	query := h.db.WithContext(ctx).Where("chat_id = ?", chatID)
	if beforeID != "" {
		var anchorMessage models.Message
		if err := h.db.Select("created_at").Where("id = ?", beforeID).First(&anchorMessage).Error; err != nil {
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
	h.db.Where("user_id IN ?", uniqueIDs).Find(&profiles)

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
		lastMsg := messages[len(messages)-1]
		var count int64
		h.db.Model(&models.Message{}).
			Where("chat_id = ? AND created_at < ?", chatID, lastMsg.CreatedAt).
			Limit(1).Count(&count)
		hasMore = count > 0
	}

	var count int64
	h.db.Model(&models.ChatMember{}).
		Where("chat_id = ? AND user_id = ?", chatID, currentUserID).
		Count(&count)

	if count == 0 {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "access denied to this chat"})
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
	if err := h.db.First(&chat, "id = ?", cID).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "chat not found"})
	}

	if !chat.IsGroup {
		return c.JSON(400, "cannot join private chat")
	}

	// Записываем в БД
	member := models.ChatMember{
		ChatID: cID,
		UserID: uID,
	}

	if err := h.db.Create(&member).Error; err != nil {
		// Если уже состоит в чате (ошибка уникальности ключа)
		return c.JSON(http.StatusConflict, map[string]string{"error": "already a member"})
	}

	// ИНВАЛИДАЦИЯ КЭША (Самый важный момент для Highload)
	// Удаляем список участников из redis, чтобы GetChatParticipants пересобрал его
	cacheKey := "chat_members:" + cID.String()
	h.redis.Del(c.Request().Context(), cacheKey)

	// Опционально: Отправляем системное сообщение в Kafka "User X joined the chat"

	return c.JSON(http.StatusOK, map[string]string{"status": "joined"})
}
func (h *Handler) GetChats(c echo.Context) error {
	ctx := c.Request().Context()

	userVal := c.Get("user")

	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}
	uID, _ := uuid.Parse(userID)
	cacheKey := "user_chats:" + userID

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 20
	}
	beforeTS := c.QueryParam("before_ts")
	maxScore := "+inf" // По умолчанию берем самые свежие
	if beforeTS != "" {
		maxScore = "(" + beforeTS // "(" означает "строго меньше чем"
	}

	// Структура для ответа (Чат + данные собеседника)
	type ChatResponse struct {
		models.Chat
		Partner models.Profile `json:"partner,omitempty"`
	}

	chatIDs, _ := h.redis.ZRevRangeByScore(ctx, cacheKey, &redis.ZRangeBy{
		Max:    maxScore,
		Min:    "-inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()

	if len(chatIDs) == 0 {
		// WARM-UP: Если в Redis пусто, идем в Postgres
		var dbChatIDs []string
		h.db.Table("chat_members").Where("user_id = ?", userID).Pluck("chat_id", &dbChatIDs)

		if len(dbChatIDs) > 0 {
			// Наполняем ZSET. Score = текущее время (потом заменим на время сообщения)
			var zMembers []redis.Z
			now := float64(time.Now().Unix())
			for _, id := range dbChatIDs {
				zMembers = append(zMembers, redis.Z{Score: now, Member: id})
			}
			h.redis.ZAdd(ctx, cacheKey, zMembers...)
			h.redis.Expire(ctx, cacheKey, time.Hour*24)
			chatIDs = dbChatIDs
		} else {
			return c.JSON(http.StatusOK, []interface{}{})
		}
	}

	// Получаем сами чаты
	var chats []models.Chat
	h.db.Where("id IN ?", chatIDs).Find(&chats)

	// Собираем профили собеседников (для диалогов 1-на-1)
	var partners []models.ChatMember
	h.db.WithContext(ctx).
		Where("chat_id IN ? AND user_id != ?", chatIDs, uID).
		Find(&partners)

	partnerIDs := make([]uuid.UUID, len(partners))
	chatToPartner := make(map[uuid.UUID]uuid.UUID)
	for i, p := range partners {
		partnerIDs[i] = p.UserID
		chatToPartner[p.ChatID] = p.UserID
	}

	var profiles []models.Profile
	h.db.WithContext(ctx).Where("user_id IN ?", partnerIDs).Find(&profiles)

	profileMap := make(map[uuid.UUID]models.Profile)
	for _, p := range profiles {
		profileMap[p.UserID] = p
	}

	// Склеиваем всё в один ответ
	response := make([]ChatResponse, len(chats))
	for i, chat := range chats {
		response[i].Chat = chat
		if partnerID, ok := chatToPartner[chat.ID]; ok {
			response[i].Partner = profileMap[partnerID]
		}
	}

	var lastTS int64
	if len(chats) > 0 {
		// Мы берем Score из Redis для этого чата или время его создания
		lastTS = chats[len(chats)-1].CreatedAt.Unix()
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"chats":   response,
		"next_ts": lastTS,
	})
}
