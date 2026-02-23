package rest

import (
	"github.com/segmentio/kafka-go"
	"gorm.io/gorm"
)

type Handler struct {
	db          *gorm.DB
	KafkaWriter *kafka.Writer
}

func NewHandler(db *gorm.DB, writer *kafka.Writer) *Handler {
	return &Handler{
		db:          db,
		KafkaWriter: writer,
	}
}
