package rest

import (
	"encoding/json"
	"net/http"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type MessageHandler interface {
	Process(messages []models.Message) error
}

func getSenderID(c echo.Context, h *Handler, userVal any) (string, error) {
	if userVal == nil {
		return "", c.JSON(http.StatusUnauthorized, map[string]string{"error": "token missing"})
	}

	// ПРИНУДИТЕЛЬНАЯ КОНВЕРТАЦИЯ ЧЕРЕЗ JSON
	// Это достанет данные из ЛЮБОГО типа (map или struct)
	jsonBytes, _ := json.Marshal(userVal)

	var tokenData struct {
		Claims map[string]interface{} `json:"claims"`
	}

	if err := json.Unmarshal(jsonBytes, &tokenData); err != nil {
		h.Logger.Error("JSON conversion failed", zap.Error(err))
		return "", c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal token error"})
	}

	// Ищем user_id в распарсенных данных
	senderID, _ := tokenData.Claims["user_id"].(string)

	if senderID == "" {
		h.Logger.Error("CRITICAL: user_id NOT found after JSON sync", zap.String("raw", string(jsonBytes)))
		return "", c.JSON(http.StatusUnauthorized, map[string]string{"error": "user_id missing"})
	}

	return senderID, nil
}

func (h *Handler) SendMessage(c echo.Context) error {
	userVal := c.Get("user")

	senderID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	ctx := c.Request().Context()

	// Добавляем receiver_id во входящую структуру
	var input struct {
		ReceiverID string `json:"receiver_id" validate:"required"`
		Text       string `json:"text" validate:"required"`
	}

	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if input.ReceiverID == "" || input.Text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "receiver_id and text are required"})
	}

	// Парсим ID отправителя
	sID, err := uuid.Parse(senderID)
	if err != nil {
		h.Logger.Error("Invalid sender UUID", zap.String("id", senderID))
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid sender id"})
	}

	// Парсим ID получателя
	rID, err := uuid.Parse(input.ReceiverID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid receiver id format"})
	}

	// Формируем полную модель сообщения
	msg := models.Message{
		ID:         uuid.New(),
		SenderID:   sID,
		ReceiverID: rID,
		Text:       input.Text,
		CreatedAt:  time.Now(),
	}

	payload, _ := json.Marshal(msg)

	// Отправляем в Kafka
	err = h.KafkaWriter.WriteMessages(ctx,
		kafka.Message{
			Topic: kafka_topics.ChatMessagesTopic.String(), // Указываем топик чата
			Key:   []byte(input.ReceiverID),                // Ключ по получателю для порядка сообщений
			Value: payload,
		},
	)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "kafka error: " + err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "sent",
		"id":     msg.ID,
	})
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
	if limit == 0 {
		limit = 50
	} // По умолчанию 50 сообщений

	var messages []models.Message

	offset, _ := strconv.Atoi(c.QueryParam("offset"))

	err = h.DB.WithContext(ctx).
		Where("(sender_id = ? AND receiver_id = ?) OR (sender_id = ? AND receiver_id = ?)",
			currentUserID, otherUserID, otherUserID, currentUserID).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset). // Пропускаем уже загруженные сообщения
		Find(&messages).Error

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	return c.JSON(http.StatusOK, messages)
}

func (h *Handler) HandleWS(c echo.Context) error {
	// Берем ID из JWT (мидлвара уже проверила токен)
	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}

	// Регистрируем клиента
	h.Hub.Mu.Lock()
	h.Hub.Clients[userID] = append(h.Hub.Clients[userID], ws)
	h.Hub.Mu.Unlock()

	h.Logger.Info("User connected to WS", zap.String("user_id", userID))

	// Ждем закрытия (важно очистить память!)
	defer func() {
		h.Hub.Mu.Lock()
		// Удаляем конкретное соединение из слайса
		connections := h.Hub.Clients[userID]
		if len(connections) == 0 {
			delete(h.Hub.Clients, userID)
		}
		for i, conn := range connections {
			if conn == ws {
				h.Hub.Clients[userID] = append(connections[:i], connections[i+1:]...)
				break
			}
		}
		h.Hub.Mu.Unlock()
		ws.Close()
	}()

	// Блокируем горутину, пока сокет жив
	for {
		if _, _, err := ws.ReadMessage(); err != nil {
			break
		}
	}
	return nil
}
