package service

import (
	"context"
	"encoding/json"
	"project/internal/models"

	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

func (h *SaveDBHandler) ProcessRegisterUsers(users []models.User) error {
	if len(users) == 0 {
		return nil
	}

	// Выполняем массовую вставку в БД
	err := h.db.CreateInBatches(users, len(users)).Error
	if err != nil {
		h.logger.Error("Failed to save batch to DB", zap.Error(err))
		return err
	}

	h.logger.Info("Successfully register users ", zap.Int("count", len(users)))
	return nil
}
func (h *SaveDBHandler) Process(ctx context.Context, batch [][]byte) error {
	users := make([]models.User, 0, len(batch))

	for _, data := range batch {
		var user models.User
		if err := json.Unmarshal(data, &user); err != nil {
			h.logger.Error("Failed to unmarshal user data",
				zap.Error(err),
				zap.String("raw_data", string(data)),
			)
			continue
		}
		users = append(users, user)
	}

	if len(users) == 0 {
		return nil
	}

	// ОДИН вызов со встроенной логикой игнорирования дубликатов
	err := h.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "email"}}, // Указываем колонку с уникальным индексом
			DoNothing: true,                             // Если email существует — ничего не делать
		}).
		CreateInBatches(users, 100). // 100 — оптимальный размер для одного SQL запроса
		Error

	if err != nil {
		h.logger.Error("Failed to save users batch to DB", zap.Error(err))
		return err
	}

	h.logger.Info("Successfully processed users batch", zap.Int("count", len(users)))
	return nil
}
