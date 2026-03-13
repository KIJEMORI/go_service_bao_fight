package rest

import (
	"encoding/json"
	"net/http"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"
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
		h.logger.Error("JSON conversion failed", zap.Error(err))
		return "", c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal token error"})
	}

	// Ищем user_id в распарсенных данных
	senderID, _ := tokenData.Claims["user_id"].(string)

	if senderID == "" {
		h.logger.Error("CRITICAL: user_id NOT found after JSON sync", zap.String("raw", string(jsonBytes)))
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
		ChatID string `json:"chat_id" validate:"required"`
		Text   string `json:"text" validate:"required"`
	}

	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if input.ChatID == "" || input.Text == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "chat_id and text are required"})
	}

	// Парсим ID отправителя
	sID, err := uuid.Parse(senderID)
	if err != nil {
		h.logger.Error("Invalid sender UUID", zap.String("id", senderID))
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid sender id"})
	}

	// Парсим ID получателя
	cID, err := uuid.Parse(input.ChatID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid chat id format"})
	}

	var count int64
	h.db.Model(&models.ChatMember{}).Where("chat_id = ? AND user_id = ?", cID, sID).Count(&count)
	if count == 0 {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "you are not a member of this chat"})
	}

	// Формируем полную модель сообщения

	msg := models.Message{
		ID:        uuid.New(),
		ChatID:    cID,
		SenderID:  sID,
		Text:      input.Text,
		CreatedAt: time.Now(),
	}

	if err := h.db.WithContext(ctx).Create(&msg).Error; err != nil {
		h.logger.Error("DB error saving message", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save message"})
	}

	payload, _ := json.Marshal(msg)

	// Отправляем в Kafka
	err = h.kafkaWriter.WriteMessages(ctx,
		kafka.Message{
			Topic: kafka_topics.ChatMessagesTopic.String(), // Указываем топик чата
			Key:   []byte(input.ChatID),                    // Ключ по получателю для порядка сообщений
			Value: payload,
		},
	)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "kafka error: " + err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status": "sent",
		"id":     msg.ID,
		"seq":    msg.SequenceNumber,
	})
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
	h.hub.Mu.Lock()
	h.hub.Clients[userID] = append(h.hub.Clients[userID], ws)
	h.hub.Mu.Unlock()

	h.logger.Info("User connected to WS", zap.String("user_id", userID))

	// Ждем закрытия (важно очистить память!)
	defer func() {
		h.hub.Mu.Lock()
		// Удаляем конкретное соединение из слайса
		connections := h.hub.Clients[userID]
		if len(connections) == 0 {
			delete(h.hub.Clients, userID)
		}
		for i, conn := range connections {
			if conn == ws {
				h.hub.Clients[userID] = append(connections[:i], connections[i+1:]...)
				break
			}
		}
		h.hub.Mu.Unlock()
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
