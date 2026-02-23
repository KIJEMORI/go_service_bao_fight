package worker

import (
	"context"
	"project/internal/models"
	"time"

	"github.com/segmentio/kafka-go"
)

type Handler interface {
	Process(ctx context.Context, batch [][]byte) error
}

type Worker struct {
	reader    *kafka.Reader
	handler   MessageHandler
	batchSize int
	interval  time.Duration
}

type BackgroundWorker interface {
	Run(ctx context.Context) error
}

// func (w *Worker) Start(ctx context.Context) {
// 	buffer := make([]models.Message, 0, w.batchSize)
// 	ticker := time.NewTicker(w.interval)

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			w.flush(&buffer)
// 			return
// 		case <-ticker.C:
// 			w.flush(&buffer)
// 		default:
// 			// Читаем сообщение и добавляем в буфер...
// 			// Если буфер полон -> w.flush(&buffer)
// 		}
// 	}
// }

// func (w *Worker) flush(buffer *[]models.Message) {
// 	if len(*buffer) == 0 {
// 		return
// 	}
// 	w.handler.Process(*buffer) // Вызываем конкретную бизнес-логику
// 	*buffer = (*buffer)[:0]
// }

type MessageHandler interface {
	Process(messages []models.Message) error
}
