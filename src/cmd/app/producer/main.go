package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"project/api/router"
	"project/internal/config"
	"project/internal/database"
	"project/internal/infrastructure/kafka_topics"
	startflags "project/internal/infrastructure/start_flags"
	"project/internal/models"
	"project/internal/transport/rest"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func main() {
	mode := flag.String("service", "all", "Which services to enable: 'user_login','user_register','send_message','all'")
	flag.Parse()

	// Инициализация инфраструктуры
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	config.LoadEnvironment()

	services := strings.Split(*mode, ",")

	if err := database.RunMigrations(database.GetMigrationDSN()); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Подключение к БД
	db, err := database.NewConnection(database.GetDSN())
	if err != nil {
		logger.Fatal("Database connection failed", zap.Error(err))
	}

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
	handler := rest.NewHandler(db, writer, logger)

	enabled := func(name startflags.Flag) bool {
		ret := slices.Contains(services, string(name)) || slices.Contains(services, "all")
		return ret
	}

	if enabled(startflags.SendMessage) {
		go StartChatWatcher(ctx, handler, os.Getenv("KAFKA_BROKER"))
	}

	e := router.NewRouter(handler, services)
	EnsureTopics(os.Getenv("KAFKA_BROKER"))

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

func StartChatWatcher(ctx context.Context, h *rest.Handler, broker string) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: []string{broker},
		Topic:   kafka_topics.ChatMessagesTopic.String(),
		GroupID: "ws-notifier-" + uuid.New().String(), // У каждого пода свой ID, чтобы все видели всё
	})

	for {
		m, err := reader.ReadMessage(ctx)
		if err != nil {
			return
		}

		var msg models.Message
		json.Unmarshal(m.Value, &msg)

		// Ищем получателя в нашем хабе
		h.Hub.Mu.RLock()
		if conns, ok := h.Hub.Clients[msg.ReceiverID.String()]; ok {
			for _, conn := range conns {
				conn.WriteJSON(msg) // МГНОВЕННАЯ ДОСТАВКА!
			}
		}
		h.Hub.Mu.RUnlock()
	}
}

func EnsureTopics(broker string) {
	conn, err := kafka.Dial("tcp", broker)
	if err != nil {
		return
	}
	defer conn.Close()

	topicConfigs := []kafka.TopicConfig{
		{
			Topic:             kafka_topics.ChatMessagesTopic.String(),
			NumPartitions:     3,
			ReplicationFactor: 1,
		},
	}

	err = conn.CreateTopics(topicConfigs...)
	if err != nil {
		fmt.Printf("Note: Topics might already exist: %v\n", err)
	}
}
