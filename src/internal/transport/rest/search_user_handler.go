package rest

import (
	"net/http"
	"project/internal/models"

	"github.com/labstack/echo/v4"
)

func (h *Handler) SearchUser(c echo.Context) error {
	query := c.QueryParam("user") // Например: /api/v1/prot/search?q=ivan
	if len(query) < 3 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "min 3 characters"})
	}

	var profiles []models.Profile
	// Поиск по нику (ровное начало) или по отображаемому имени (везде)
	// ILIKE — регистронезависимый поиск в Postgres
	err := h.DB.Where("nick_name ILIKE ? OR user_name ILIKE ?", query+"%", "%"+query+"%").
		Limit(20).
		Find(&profiles).Error

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "search failed"})
	}

	return c.JSON(http.StatusOK, profiles)
}
