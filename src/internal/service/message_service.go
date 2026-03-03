package service

import (
	"context"
	"encoding/json"
	"project/internal/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type MessageSaveHandler struct {
	db     *gorm.DB
	logger *zap.Logger
}

func NewMessageSaveHandler(db *gorm.DB, logger *zap.Logger) *MessageSaveHandler {
	return &MessageSaveHandler{
		db:     db,
		logger: logger,
	}
}

func (h *MessageSaveHandler) Process(ctx context.Context, batch [][]byte) error {
	var msgs []models.Message
	for _, data := range batch {
		var m models.Message
		if err := json.Unmarshal(data, &m); err != nil {
			h.logger.Error("Unmarshal error", zap.Error(err), zap.String("raw", string(data)))
			continue
		}
		msgs = append(msgs, m)
	}

	if len(msgs) > 0 {
		return h.db.WithContext(ctx).CreateInBatches(msgs, 100).Error
	}
	return nil
}
