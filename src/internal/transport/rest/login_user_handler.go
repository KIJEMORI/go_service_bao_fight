package rest

import (
	"net/http"
	"project/internal/models"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) LoginUser(c echo.Context) error {
	var input struct {
		Email    string `json:"email" validate:"required"`
		Password string `json:"password" validate:"required"`
	}
	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	}
	if err := c.Validate(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "bad request"})
	}

	// Ищем юзера (Прямой запрос)
	var user models.User
	if err := h.db.Where("email = ?", input.Email).First(&user).Error; err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	// Сверяем хеш
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}

	// Генерируем JWT
	at, rt, err := h.createSession(c, user.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "session error"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"access_token":  at,
		"refresh_token": rt,
		"user_id":       user.ID.String(),
	})
}
