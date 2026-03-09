package rest

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHub struct {
	// Карта: ID пользователя -> список его активных вкладок/соединений
	Clients map[string][]*websocket.Conn
	Mu      sync.RWMutex
}

type Handler struct {
	DB          *gorm.DB
	KafkaWriter *kafka.Writer
	Logger      *zap.Logger
	Hub         *WSHub
	S3Client    *minio.Client
	Redis       *redis.Client
}

func NewHandler(db *gorm.DB, writer *kafka.Writer, s3 *minio.Client, rdb *redis.Client, logger *zap.Logger) *Handler {

	return &Handler{
		DB:          db,
		KafkaWriter: writer,
		Logger:      logger,
		Hub: &WSHub{
			Clients: make(map[string][]*websocket.Conn),
		},
		S3Client: s3,
		Redis:    rdb,
	}
}
