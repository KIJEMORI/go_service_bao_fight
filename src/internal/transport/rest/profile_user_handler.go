package rest

import (
	"net/http"
	"project/internal/models"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

func (h *Handler) GetProfile(c echo.Context) error {

	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	uID, err := uuid.Parse(userID)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid sender id"})
	}

	var profile models.Profile
	if err := h.DB.WithContext(c.Request().Context()).Where("user_id = ?", uID).First(&profile).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "profile not found"})
	}

	return c.JSON(http.StatusOK, profile)
}

func (h *Handler) ChangeProfile(c echo.Context) error {
	// Берем ID из JWT (мидлвара уже проверила токен)
	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}

	var input struct {
		Nick       string `json:"nickname" validate:"required"`
		Name       string `json:"name" validate:"required"`
		Age        uint32 `json:"age" validate:"required"`
		Gender     string `json:"gender" validate:"required"`
		Sport      string `json:"sport"`
		Level      string `json:"level"`
		Experience string `json:"experience"`
		Weight     string `json:"weight" valide:"required"`
	}

	if err := c.Bind(&input); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if input.Nick == "" || input.Name == "" || input.Age == 0 || input.Gender == "" || input.Weight == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Name, age, gender and weight are required"})
	}

	uID, err := uuid.Parse(userID)
	if err != nil {
		h.Logger.Error("Invalid sender UUID", zap.String("id", userID))
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid sender id"})
	}

	profile := models.Profile{
		UserID:     uID,
		NickName:   input.Nick,
		UserName:   input.Name,
		Age:        input.Age,
		Gender:     input.Gender,
		Sport:      input.Sport,
		Level:      input.Level,
		Experience: input.Experience,
		Weight:     input.Weight,
	}

	ctx := c.Request().Context()

	err = h.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"user_name",
			"age",
			"gender",
			"sport",
			"level",
			"experience",
			"weight",
		}),
	}).Create(&profile).Error

	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "database error"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id": uID,
	})
}
