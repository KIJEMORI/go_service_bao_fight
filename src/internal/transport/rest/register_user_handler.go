package rest

import (
	"encoding/json"
	"net/http"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) RegisterUser(c echo.Context) error {

	ctx := c.Request().Context()

	var input struct {
		Email    string `json:"email" validate:"required"`
		Password string `json:"password" validate:"required"`
	}

	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to parse json"})
	}

	if input.Email == "" || input.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email and password are required"})
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to secure password"})
	}
	user := models.User{
		Email:        input.Email,
		PasswordHash: string(hashedPassword),
	}
	if err := h.db.WithContext(ctx).Create(&user).Error; err != nil {
		// Проверяем на дубликат (Postgres SQLState 23505)
		if strings.Contains(err.Error(), "duplicate key") {
			return c.JSON(http.StatusConflict, map[string]string{"error": "email already taken"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "database error"})
	}

	payload, err := json.Marshal(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to encode message"})
	}

	err = h.KafkaWriter.WriteMessages(ctx,
		kafka.Message{
			Topic: kafka_topics.UserRegisterTopic.String(),
			Key:   []byte(input.Email), // Хорошая практика: использовать email как ключ для партиционирования
			Value: payload,
		},
	)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"status": "success",
		"id":     user.ID.String(),
		"email":  input.Email,
	})

}
