package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"project/internal/config"
	"project/internal/database"
	"project/internal/service"
	"project/internal/worker"

	"go.uber.org/zap"
)

func main() {
	mode := flag.String("mode", "all", "Which workers to run: 'users', 'logs' or 'all'")
	flag.Parse()

	// Инициализация логгера и конфигов
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	config.LoadEnvironment()

	broker := os.Getenv("KAFKA_BROKER")
	if broker == "" {
		broker = "localhost:9092"
	}

	if err := database.RunMigrations(database.GetMigrationDSN()); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Подключение к БД
	db, err := database.NewConnection(database.GetDSN())
	if err != nil {
		logger.Fatal("Database connection failed", zap.Error(err))
	}

	// Настройка контекста для Graceful Shutdown
	// Слушает сигналы от Docker на остановку (SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	//  Инициализация хендлера (бизнес-логика)
	//dbHandler := service.NewSaveDBHandler(db, logger)

	var wg sync.WaitGroup

	start := func(name, topic string, h worker.Handler, routines int) {
		for i := 0; i < routines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				w := worker.NewKafkaWorker(worker.Config{
					Name:      name,
					Topic:     topic,
					Brokers:   []string{broker},
					BatchSize: 50,
					Interval:  2 * time.Second,
					Handler:   h,
					Logger:    logger.With(zap.String("worker", name), zap.Int("id", id)),
				})
				w.Run(ctx)
			}(i)
		}
	}

	if *mode == "users" || *mode == "all" {
		userHandler := service.NewSaveDBHandler(db, logger)
		start("user-saver", "registers-topic", userHandler, 2) // 2 горутины
	}
	if *mode == "logs" || *mode == "all" {
		// logHandler := service.NewLogHandler(db, logger)
		// start("log-processor", "system-logs", logHandler, 1)
	}

	logger.Info("All consumers are running...")
	// Ожидание завершения
	<-ctx.Done() // Блокируемся, пока не придет сигнал
	logger.Info("Shutting down consumer...")

	wg.Wait() // Ждем, пока Run() сделает финальный flush в базу
	logger.Info("Consumer exited cleanly")
}
