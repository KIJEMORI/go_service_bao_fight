package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"project/api/router"
	"project/internal/config"
	"project/internal/transport/rest"

	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func main() {
	mode := flag.String("service", "all", "Which services to enable: 'users', 'billing' or 'all'")
	flag.Parse()

	// Инициализация инфраструктуры
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	config.LoadEnvironment()

	// Настройка Kafka Writer
	writer := &kafka.Writer{
		Addr:     kafka.TCP(os.Getenv("KAFKA_BROKER")),
		Balancer: &kafka.LeastBytes{},
	}
	defer writer.Close()

	// Graceful Shutdown контекст
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Инициализация сервера
	handler := &rest.Handler{KafkaWriter: writer}
	e := router.NewRouter(handler, *mode)

	var wg sync.WaitGroup

	// Запуск HTTP-сервера в горутине
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("Starting Producer API", zap.String("port", "8080"))

		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server failed", zap.Error(err))
		}
	}()

	// Ожидание сигнала завершения
	<-ctx.Done()
	logger.Info("Shutdown signal received, closing API...")

	// Graceful Shutdown для Echo
	// Даем серверу 10 секунд на то, чтобы завершить текущие HTTP-запросы
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("Graceful shutdown failed", zap.Error(err))
	}

	wg.Wait()
	logger.Info("Producer API exited cleanly")
}
