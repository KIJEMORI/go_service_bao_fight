package service

import (
	"context"
	"encoding/json"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ReactionHandler struct {
	db     *gorm.DB
	rdb    *redis.Client
	writer *kafka.Writer
	logger *zap.Logger
}

func NewReactionHandler(db *gorm.DB, writer *kafka.Writer, rdb *redis.Client, logger *zap.Logger) *ReactionHandler {
	return &ReactionHandler{db: db, writer: writer, logger: logger}
}

func (h *ReactionHandler) Process(ctx context.Context, messages [][]byte) error {
	var reactions []models.Reaction

	for _, data := range messages {
		var r models.Reaction
		if err := json.Unmarshal(data, &r); err != nil {
			h.logger.Error("Failed to unmarshal reaction", zap.Error(err))
			continue
		}
		if r.ID == uuid.Nil {
			r.ID = uuid.New()
		}
		reactions = append(reactions, r)
	}

	if len(reactions) == 0 {
		return nil
	}

	// Пакетное сохранение в БД
	err := h.db.WithContext(ctx).CreateInBatches(reactions, len(reactions)).Error
	if err != nil {
		h.logger.Error("Failed to batch insert reactions", zap.Error(err))
		return err
	}

	// Проверка матчей
	for _, r := range reactions {
		h.checkMatch(r)
	}

	return nil
}

func (h *ReactionHandler) checkMatch(r models.Reaction) {

	ctx := context.Background()
	// Ключ: кто лайкнул этого пользователя
	fansKey := "fans:" + r.ToUserID.String()
	// Ключ: кого лайкнул я
	myLikesKey := "my_likes:" + r.FromUserID.String()

	if r.Action == models.ActionDislike {
		// Если я дизлайкнул того, кто меня лайкнул — я должен исчезнуть из его списка поклонников
		h.rdb.SRem(ctx, "fans:"+r.ToUserID.String(), r.FromUserID.String())
		return
	}

	if r.Action != models.ActionLike {
		return
	}

	// Добавляем в список получателя
	h.rdb.SAdd(ctx, fansKey, r.FromUserID.String())
	// Запоминаем свой лайк
	h.rdb.SAdd(ctx, myLikesKey, r.ToUserID.String())

	// Проверка матча в Redis
	isMatch, _ := h.rdb.SIsMember(ctx, "my_likes:"+r.ToUserID.String(), r.FromUserID.String()).Result()

	if isMatch {
		// МЭТЧ!
		h.rdb.SRem(ctx, fansKey, r.FromUserID.String())
		h.rdb.SRem(ctx, "fans:"+r.FromUserID.String(), r.ToUserID.String())

		h.rdb.SRem(ctx, myLikesKey, r.ToUserID.String())
		h.rdb.SRem(ctx, "my_likes:"+r.ToUserID.String(), r.FromUserID.String())

		h.db.Transaction(func(tx *gorm.DB) error {
			var existingChatID uuid.UUID
			// Ищем чат, где есть оба юзера (диалог)
			tx.Raw(`
	            SELECT chat_id FROM chat_members
	            WHERE user_id IN (?, ?)
	            GROUP BY chat_id HAVING COUNT(chat_id) = 2
	        `, r.FromUserID, r.ToUserID).Scan(&existingChatID)

			if existingChatID != uuid.Nil {
				return nil // Чат уже есть, ничего не делаем
			}

			// Создаем новый чат
			newID := uuid.New()
			if err := tx.Create(&models.Chat{ID: newID, IsGroup: false}).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.ChatMember{ChatID: newID, UserID: r.FromUserID}).Error; err != nil {
				return err
			}
			if err := tx.Create(&models.ChatMember{ChatID: newID, UserID: r.ToUserID}).Error; err != nil {
				return err
			}

			// Добавляем новый чат в "голову" списка обоим юзерам
			h.rdb.ZAdd(ctx, "user_chats:"+r.FromUserID.String(), redis.Z{Score: float64(time.Now().Unix()), Member: newID})
			h.rdb.ZAdd(ctx, "user_chats:"+r.ToUserID.String(), redis.Z{Score: float64(time.Now().Unix()), Member: newID})

			// Отправляем уведомления только если чат был создан только что
			h.sendWSNotification(r.FromUserID, "match", newID, kafka_topics.MatchFindNotification.String())
			h.sendWSNotification(r.ToUserID, "match", newID, kafka_topics.LikeNotification.String())
			return nil
		})
	} else {
		h.sendWSNotification(r.ToUserID, "new_like", nil, kafka_topics.LikeNotification.String())
	}
}

func (h *ReactionHandler) sendWSNotification(userID uuid.UUID, nType string, chatID interface{}, topic string) {
	payload, _ := json.Marshal(map[string]any{
		"user_id": userID.String(),
		"type":    nType,
		"chat_id": chatID,
		"ts":      time.Now().Unix(),
	})

	// Пушим в топик, который слушают все API-инстансы
	h.writer.WriteMessages(context.Background(), kafka.Message{
		Topic: topic,
		Key:   []byte(userID.String()),
		Value: payload,
	})
}
