package rest

import (
	"encoding/json"
	"net/http"
	"os"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/segmentio/kafka-go"
)

func (h *Handler) createSession(c echo.Context, userID uuid.UUID) (string, string, error) {
	// Генерируем Access Token (короткий)
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID.String(),
		"exp":     time.Now().Add(time.Minute * 15).Unix(), // 15 минут
	})
	at, _ := accessToken.SignedString([]byte(os.Getenv("JWT_SECRET")))

	// Refresh Token (длинный случайный ID)
	rt := uuid.New().String()

	// Сохраняем сессию в БД
	session := models.Session{
		ID:           uuid.New(),
		UserID:       userID,
		RefreshToken: rt,
		UserAgent:    c.Request().UserAgent(),
		ClientIP:     c.RealIP(),
		ExpiresAt:    time.Now().Add(time.Hour * 24 * 30), // 30 дней
	}

	if err := h.DB.Create(&session).Error; err != nil {
		return "", "", err
	}

	return at, rt, nil
}

func (h *Handler) Refresh(c echo.Context) error {
	var input struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Ищем сессию в БД
	var session models.Session
	err := h.DB.Where("refresh_token = ? AND expires_at > ?", input.RefreshToken, time.Now()).
		First(&session).Error

	if err != nil {
		h.DB.Where("refresh_token = ?", input.RefreshToken).Delete(&models.Session{})
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "session expired or invalid"})
	}

	// Удаляем старую сессию (Refresh Token Rotation - для безопасности)
	h.DB.Delete(&session)

	// Создаем новую пару токенов
	at, rt, err := h.createSession(c, session.UserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "could not refresh"})
	}

	return c.JSON(http.StatusOK, map[string]string{
		"access_token":  at,
		"refresh_token": rt,
	})
}

func (h *Handler) Logout(c echo.Context) error {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	c.Bind(&input)

	// Удаляем сессию по токену
	if err := h.DB.Where("refresh_token = ?", input.RefreshToken).Delete(&models.Session{}).Error; err != nil {
		return c.JSON(500, map[string]string{"error": "failed to logout"})
	}

	// Кидаем в Kafka факт того, что сессия закрыта
	payload, _ := json.Marshal(map[string]string{"event": "session_terminated", "token": input.RefreshToken})
	_ = h.KafkaWriter.WriteMessages(c.Request().Context(), kafka.Message{
		Topic: kafka_topics.LogoutToipic.String(),
		Value: payload,
	})

	return c.NoContent(http.StatusNoContent)
}
