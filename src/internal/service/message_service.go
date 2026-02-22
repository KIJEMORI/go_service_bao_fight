package service

import (
	"project/internal/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type saveDBHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func (h *saveDBHandler) Process(messages []models.Message) error {
	if len(messages) == 0 {
		return nil
	}

	// Выполняем массовую вставку в БД
	err := h.db.CreateInBatches(messages, len(messages)).Error
	if err != nil {
		h.logger.Error("Failed to save batch to DB", zap.Error(err))
		return err
	}

	h.logger.Info("Successfully saved messages to DB", zap.Int("count", len(messages)))
	return nil
}

func NewSaveDBHandler(db *gorm.DB, logger *zap.Logger) *saveDBHandler {
	return &saveDBHandler{
		db:     db,
		logger: logger,
	}
}
