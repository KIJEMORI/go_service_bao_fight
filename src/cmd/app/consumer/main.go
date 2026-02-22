package main

import (
	"context"
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
	// Инициализация логгера и конфигов
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	config.LoadEnvironment()

	// Подключение к БД
	db, err := database.NewConnection()
	if err != nil {
		logger.Fatal("Database connection failed", zap.Error(err))
	}

	// Настройка контекста для Graceful Shutdown
	// Слушает сигналы от Docker на остановку (SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	//  Инициализация хендлера (бизнес-логика)
	dbHandler := service.NewSaveDBHandler(db, logger)

	// Создание воркера
	w := worker.NewKafkaWorker(worker.Config{
		Name:      "DB-Saver",
		Topic:     "messages-topic",
		BatchSize: 100,             // Сброс при накоплении 100 сообщений
		Interval:  1 * time.Second, // Или каждые 1с, если батч не полон
		Handler:   dbHandler,
		Logger:    logger,
	})

	var wg sync.WaitGroup

	// Запуск воркера в горутине
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Consumer worker started")
		if err := w.Run(ctx); err != nil {
			logger.Error("Worker error", zap.Error(err))
		}
	}()

	// Ожидание завершения
	<-ctx.Done() // Блокируемся, пока не придет сигнал
	logger.Info("Shutting down consumer...")

	wg.Wait() // Ждем, пока Run() сделает финальный flush в базу
	logger.Info("Consumer exited cleanly")
}
