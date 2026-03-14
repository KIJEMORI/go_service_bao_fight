package rest

import (
	"context"
	"encoding/json"
	"net/http"
	"project/internal/infrastructure/kafka_topics"
	"project/internal/models"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
)

func (h *Handler) GetDiscovery(c echo.Context) error {
	ctx := c.Request().Context()
	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}
	cacheKey := "discovery_queue:" + userID

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	ids, _ := h.redis.LRange(ctx, cacheKey, 0, int64(limit)).Result()

	if len(ids) == 0 {
		// Если в redis пусто — один раз тяжело ищем в Postgres
		var candidateIDs []string
		h.db.Table("profiles").
			Joins("LEFT JOIN reactions ON reactions.to_user_id = profiles.user_id AND reactions.from_user_id = ?", userID).
			Where("reactions.id IS NULL AND profiles.user_id != ?", userID).
			Limit(100).Pluck("user_id", &candidateIDs).
			Order("RANDOM()")

		if len(candidateIDs) == 0 {
			return c.JSON(http.StatusOK, map[string]string{"empty": "no more profiles"})
		}
		// Кладем их в redis List
		h.redis.RPush(ctx, cacheKey, candidateIDs)
		h.redis.Expire(ctx, cacheKey, time.Hour)
		ids = candidateIDs[:min(10, len(candidateIDs))]
	}

	// Достаем полные профили по этим 10 ID
	var profiles []models.Profile
	h.db.Where("user_id IN ?", ids).Find(&profiles)

	// Удаляем выданные ID из очереди в redis
	h.redis.LTrim(ctx, cacheKey, int64(len(ids)), -1)
	return c.JSON(http.StatusOK, profiles)
}

func (h *Handler) SetReaction(c echo.Context) error {
	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}
	uID, _ := uuid.Parse(userID)

	var input struct {
		ToUserID string `json:"to_user_id" validate:"required"`
		Action   string `json:"action" validate:"required"` // 'like' or 'dislike'
	}
	if err := c.Bind(&input); err != nil {
		return err
	}

	notification := map[string]string{
		"type":    input.Action,
		"from_id": uID.String(),
		"to_id":   input.ToUserID,
	}
	payload, _ := json.Marshal(notification)

	ctx := c.Request().Context()

	h.kafkaWriter.WriteMessages(ctx, kafka.Message{
		Topic: kafka_topics.NewActionProfile.String(),
		Key:   []byte(input.ToUserID), // Ключ — получатель лайка
		Value: payload,
	})

	return c.JSON(http.StatusAccepted, map[string]string{"status": "processing"})
}

func (h *Handler) GetLikingProfiles(c echo.Context) error {
	userVal := c.Get("user")
	userID, err := getSenderID(c, h, userVal)
	if err != nil {
		return err
	}
	//uID, _ := uuid.Parse(userID)

	ctx := c.Request().Context()
	cacheKey := "fans:" + userID

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	beforeTS := c.QueryParam("before_ts")
	maxScore := "+inf"
	if beforeTS != "" {
		maxScore = "(" + beforeTS // Строго меньше (исключая само граничное значение)
	}

	fanIDs, err := h.redis.ZRevRangeByScore(ctx, cacheKey, &redis.ZRangeBy{
		Max:    maxScore,
		Min:    "-inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()

	if err != nil || len(fanIDs) == 0 {
		var dbReactions []models.Reaction
		h.db.Where("to_user_id = ? AND action = 'like'", userID).
			Where("from_user_id NOT IN (SELECT to_user_id FROM reactions WHERE from_user_id = ?)", userID).
			Order("created_at DESC").Limit(100).Find(&dbReactions)

		// Сразу закидываем в Redis, чтобы следующий запрос был быстрым
		if len(dbReactions) > 0 {
			var zMembers []redis.Z
			for _, r := range dbReactions {
				zMembers = append(zMembers, redis.Z{
					Score:  float64(r.CreatedAt.Unix()), // Время как оценка для сортировки
					Member: r.FromUserID.String(),
				})
				fanIDs = append(fanIDs, r.FromUserID.String())
			}
			h.redis.ZAdd(ctx, cacheKey, zMembers...)
			h.redis.Expire(ctx, cacheKey, time.Hour*24)
			if len(fanIDs) > limit {
				fanIDs = fanIDs[:limit]
			}
		} else {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"profiles": []models.Profile{},
				"next_ts":  0,
			})
		}
	}

	var profiles []models.Profile
	// Находим профили тех, кто лайкнул меня, но кого я еще не оценивал
	if err := h.db.Where("user_id IN ?", fanIDs).Find(&profiles).Error; err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to fetch fans"})
	}

	var nextTS int64
	if len(profiles) > 0 {
		// Берем Score (время) последнего элемента
		lastScore, _ := h.redis.ZScore(ctx, cacheKey, fanIDs[len(fanIDs)-1]).Result()
		nextTS = int64(lastScore)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"profiles": profiles,
		"next_ts":  nextTS, // Фронт пришлет это в следующем запросе как before_ts
	})
}

func (h *Handler) StartNotificationWatcher(ctx context.Context, broker string, topics []string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     []string{broker},
		GroupTopics: topics,
		// Группа должна быть уникальной для каждого инстанса API!
		// Чтобы ВСЕ инстансы получили сообщение и проверили своих клиентов.
		GroupID: "notif-group-" + uuid.New().String(),
	})

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			return
		}

		var notif struct {
			UserID string `json:"user_id"`
			Type   string `json:"type"`
		}
		json.Unmarshal(m.Value, &notif)

		// Ищем юзера в локальном Hub этого инстанса
		h.hub.Mu.RLock()
		if conns, ok := h.hub.Clients[notif.UserID]; ok {
			for _, conn := range conns {
				// Шлем в сокет
				conn.WriteJSON(map[string]any{
					"event":   "notification",
					"type":    m.Topic,
					"payload": string(m.Value),
				})
			}
		}
		h.hub.Mu.RUnlock()
	}
}
