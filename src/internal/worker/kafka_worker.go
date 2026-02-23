package worker

import (
	"context"
	"time"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

type Config struct {
	Name      string
	Brokers   []string
	Topic     string
	BatchSize int
	Interval  time.Duration
	Handler   Handler
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
		Brokers: w.cfg.Brokers,
		Topic:   w.cfg.Topic,
		GroupID: w.cfg.Name,
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

	batch := make([][]byte, 0, w.cfg.BatchSize)
	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	w.cfg.Logger.Info("Worker started",
		zap.String("topic", w.cfg.Topic),
		zap.String("group", w.cfg.Name),
	)
	for {
		select {
		case <-ctx.Done():
			// При выключении сбрасываем остатки
			w.flush(context.Background(), &batch)
			return nil

		case m := <-msgChan:
			// Сообщение пришло! Добавляем в буфер
			batch = append(batch, m.Value)
			if len(batch) >= w.cfg.BatchSize {
				w.flush(ctx, &batch)
				ticker.Reset(w.cfg.Interval)
			}

		case <-ticker.C:
			// Таймер сработал (например, прошла 1 сек), сбрасываем всё что есть
			if len(batch) > 0 {
				w.flush(ctx, &batch)
			}

		case err := <-errChan:
			// Если Kafka упала
			w.cfg.Logger.Error("Kafka read error", zap.Error(err))
			return err
		}
	}
}

func (w *kafkaWorker) flush(ctx context.Context, batch *[][]byte) {
	if len(*batch) == 0 {
		return
	}

	if err := w.cfg.Handler.Process(ctx, *batch); err != nil {
		w.cfg.Logger.Error("Failed to process batch", zap.Error(err))
		// Оффсеты в Kafka не закоммитятся автоматически, если ReadMessage не был завершен успешно,
		// но при пакетной обработке стоит продумать стратегию ошибок (Dead Letter Queue).
		return
	}

	// Очищаем слайс, сохраняя выделенную память
	*batch = (*batch)[:0]
}
