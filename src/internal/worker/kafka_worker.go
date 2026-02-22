package worker

import (
	"context"
	"os"
	"time"

	"project/internal/models"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Config struct {
	Name      string
	Topic     string
	BatchSize int
	Interval  time.Duration
	Handler   MessageHandler
	Logger    *zap.Logger
}

type kafkaWorker struct {
	cfg Config
}

func NewKafkaWorker(cfg Config) BackgroundWorker {
	return &kafkaWorker{cfg: cfg}
}

func (w *kafkaWorker) Run(ctx context.Context) error {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{os.Getenv("KAFKA_BROKER")},
		Topic:   w.cfg.Topic,
		GroupID: "group-" + w.cfg.Name,
	})
	defer reader.Close()

	// Канал для передачи сообщений из горутины-читателя в основной цикл воркера
	msgChan := make(chan kafka.Message)
	// Канал для ошибок
	errChan := make(chan error)

	// Запускаем чтение в отдельном потоке (горутине)
	go func() {
		for {
			m, err := reader.ReadMessage(context.Background())
			if err != nil {
				errChan <- err
				return
			}
			msgChan <- m
		}
	}()

	buffer := make([]models.Message, 0, w.cfg.BatchSize)
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	w.cfg.Logger.Info("Worker started", zap.String("name", w.cfg.Name))

	for {
		select {
		case <-ctx.Done():
			// При выключении сбрасываем остатки
			return w.cfg.Handler.Process(buffer)

		case m := <-msgChan:
			// Сообщение пришло! Добавляем в буфер
			buffer = append(buffer, models.Message{
				Content:   string(m.Value),
				CreatedAt: time.Now(),
			})

			if len(buffer) >= w.cfg.BatchSize {
				w.cfg.Handler.Process(buffer)
				buffer = buffer[:0]
				ticker.Reset(w.cfg.Interval) // Сброс таймера, так как только что записали
			}

		case <-ticker.C:
			// Таймер сработал (например, прошла 1 сек), сбрасываем всё что есть
			if len(buffer) > 0 {
				w.cfg.Handler.Process(buffer)
				buffer = buffer[:0]
			}

		case err := <-errChan:
			// Если Kafka упала
			w.cfg.Logger.Error("Kafka read error", zap.Error(err))
			return err
		}
	}
}
