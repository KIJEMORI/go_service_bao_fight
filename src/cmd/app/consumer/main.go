package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"project/internal/database"
	"project/internal/infrastructure/kafka_topics"
	startflags "project/internal/infrastructure/start_flags"
	"project/internal/service"
	"project/internal/worker"

	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func main() {
	mode := flag.String("service", "all", "Which workers to run: 'users', 'logs' or 'all'")
	flag.Parse()

	services := strings.Split(*mode, ",")

	// Инициализация логгера и конфигов
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	//config.LoadEnvironment()

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

	enabled := func(name startflags.Flag) bool {
		ret := slices.Contains(services, string(name)) || slices.Contains(services, "all")
		return ret
	}

	if enabled(startflags.UserRegisterFlag) {
		userHandler := service.NewUserRegisterHandler(db, logger, &kafka.Writer{Addr: kafka.TCP(broker)})
		start("user-saver", kafka_topics.UserRegisterTopic.String(), userHandler, 2) // 2 горутины
	}
	if *mode == "logs" || *mode == "all" {
		// logHandler := service.NewLogHandler(db, logger)
		// start("log-processor", "system-logs", logHandler, 1)
	}
	if enabled(startflags.SendMessage) {
		msgHandler := service.NewMessageSaveHandler(db, logger)
		start("message-saver-v2", kafka_topics.ChatMessagesTopic.String(), msgHandler, 3)
	}
	if enabled(startflags.ReactionWorker) {
		writer := &kafka.Writer{
			Addr:     kafka.TCP(os.Getenv("KAFKA_BROKER")),
			Balancer: &kafka.LeastBytes{},
		}
		defer writer.Close()

		rdb := redis.NewClient(&redis.Options{
			Addr: os.Getenv("REDIS_HOST"),
		})

		// Проверка связи (Healthcheck)
		if err := rdb.Ping(context.Background()).Err(); err != nil {
			logger.Fatal("Redis not reachable", zap.Error(err))
		}

		reactionHandler := service.NewReactionHandler(db, writer, rdb, logger)
		// Запускаем воркер: читает топик user-reactions, сохраняет пачками по 50
		start("reaction-processor", kafka_topics.ActionLikeProfile.String(), reactionHandler, 2)
	}

	logger.Info("All consumers are running...")
	// Ожидание завершения
	<-ctx.Done() // Блокируемся, пока не придет сигнал
	logger.Info("Shutting down consumer...")

	wg.Wait() // Ждем, пока Run() сделает финальный flush в базу
	logger.Info("Consumer exited cleanly")
}
