package rest

import (
	"encoding/json"
	"net/http"
	"project/internal/infrastructure/kafka"
	"project/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
	"golang.org/x/crypto/bcrypt"
)

type RegisterUserHandler interface {
	Process(users []models.User) error
}

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
	input.Password = string(hashedPassword)

	payload, err := json.Marshal(input)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to encode message"})
	}

	err = h.KafkaWriter.WriteMessages(ctx,
		kafka.Message{
			Topic: kafka.UserRegisterTopic.String(),
			Key:   []byte(input.Email), // Хорошая практика: использовать email как ключ для партиционирования
			Value: payload,
		},
	)

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"status": "sent",
		"email":  input.Email,
	})

}
