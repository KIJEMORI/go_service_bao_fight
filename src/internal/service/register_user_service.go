package service

import (
	"context"
	"encoding/json"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// func (h *SaveDBHandler) ProcessRegisterUsers(users []models.User) error {
// 	if len(users) == 0 {
// 		return nil
// 	}

// 	// Выполняем массовую вставку в БД
// 	err := h.db.CreateInBatches(users, len(users)).Error
// 	if err != nil {
// 		h.logger.Error("Failed to save batch to DB", zap.Error(err))
// 		return err
// 	}

//		h.logger.Info("Successfully register users ", zap.Int("count", len(users)))
//		return nil
//	}

type UserRegisterHandler struct {
	db             *gorm.DB
	logger         *zap.Logger
	notifierWriter *kafka.Writer // Писатель в топик ошибок
}

func NewUserRegisterHandler(db *gorm.DB, logger *zap.Logger, writer *kafka.Writer) *UserRegisterHandler {
	return &UserRegisterHandler{
		db:             db,
		logger:         logger,
		notifierWriter: writer,
	}
}

func (h *UserRegisterHandler) Process(ctx context.Context, batch [][]byte) error {
	// users := make([]models.User, 0, len(batch))

	// for _, data := range batch {
	// 	var user models.User
	// 	if err := json.Unmarshal(data, &user); err != nil {
	// 		h.logger.Error("Failed to unmarshal user data",
	// 			zap.Error(err),
	// 			zap.String("raw_data", string(data)),
	// 		)
	// 		continue
	// 	}
	// 	users = append(users, user)
	// }

	// if len(users) == 0 {
	// 	return nil
	// }

	// var insertedIDs []uuid.UUID

	// // ОДИН вызов со встроенной логикой игнорирования дубликатов
	// err := h.db.WithContext(ctx).
	// 	Model(&models.User{}).
	// 	Clauses(
	// 		clause.OnConflict{
	// 			Columns:   []clause.Column{{Name: "email"}},
	// 			DoNothing: true,
	// 		},
	// 		clause.Returning{Columns: []clause.Column{{Name: "id"}}},
	// 	).
	// 	// Возвращаем ID только тех, кто реально создался
	// 	CreateInBatches(users, 100).
	// 	Scan(&insertedIDs). // Записываем ID в наш слайс
	// 	Error

	// if err != nil {
	// 	h.logger.Error("Failed to save users batch to DB", zap.Error(err))
	// 	return err
	// }

	// if len(insertedIDs) < len(users) {
	// 	h.handleDuplicates(ctx, users, insertedIDs)
	// }

	// h.logger.Info("Successfully processed users batch", zap.Int("count", len(insertedIDs)))
	return nil
}

func (h *UserRegisterHandler) handleDuplicates(ctx context.Context, original []models.User, inserted []uuid.UUID) {
	insertedMap := make(map[uuid.UUID]bool)
	for _, id := range inserted {
		insertedMap[id] = true
	}

	for _, user := range original {
		if !insertedMap[user.ID] {
			h.logger.Warn("Registration conflict: email already exists", zap.String("email", user.Email))

			// Отправляем уведомление об ошибке в Kafka для фронтенда
			msg := map[string]string{
				"email":  user.Email,
				"error":  "email_already_taken",
				"status": "failed",
			}
			payload, _ := json.Marshal(msg)

			h.notifierWriter.WriteMessages(ctx, kafka.Message{
				Topic: kafka_topics.UserRegisterErrorTopic.String(),
				Key:   []byte(user.Email),
				Value: payload,
			})
		}
	}
}
