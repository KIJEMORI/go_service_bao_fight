package rest

import (
	"github.com/segmentio/kafka-go"
)

type Handler struct {
	KafkaWriter *kafka.Writer
}
