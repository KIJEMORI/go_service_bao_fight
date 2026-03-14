package main

import (
	"context"
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
	"project/internal/database"
	"project/internal/infrastructure/kafka_topics"
	startflags "project/internal/infrastructure/start_flags"
	"project/internal/transport/rest"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	"go.uber.org/zap"
)

func main() {
	mode := flag.String("service", "all", "Which services to enable: 'user_login','user_register','send_message','all'")
	flag.Parse()

	// Инициализация инфраструктуры
	logger, _ := zap.NewProduction()
	defer logger.Sync()
	//config.LoadEnvironment()

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

	// S3 Хранилище
	endpoint := os.Getenv("MINIO_ENDPOINT") // например, "minio:9000"
	accessKey := os.Getenv("MINIO_ROOT_USER")
	secretKey := os.Getenv("MINIO_ROOT_PASSWORD")
	bucketName := "profiles"

	var s3Client *minio.Client

	// Цикл ожидания доступности S3 (Retry logic)
	for i := 0; i < 10; i++ {
		s3Client, err = minio.New(endpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
			Secure: false, // Для локальной разработки без SSL
		})

		// Проверяем соединение через Health Check (просто список бакетов)
		if err == nil {
			_, err = s3Client.ListBuckets(context.Background())
		}

		if err == nil {
			logger.Info("Successfully connected to Minio S3")
			break
		}

		logger.Warn("S3 not ready, retrying in 2 seconds...", zap.Int("attempt", i+1))
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		logger.Fatal("Failed to connect to S3 after retries", zap.Error(err))
	}

	// Автоматическое создание бакета (папки) при старте
	exists, _ := s3Client.BucketExists(context.Background(), bucketName)
	if !exists {
		err = s3Client.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{})
		if err != nil {
			logger.Fatal("Failed to create bucket", zap.Error(err))
		}
		// Делаем бакет публичным на чтение (чтобы Nginx мог забирать фото)
		policy := fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucketName)
		s3Client.SetBucketPolicy(context.Background(), bucketName, policy)
		logger.Info("Bucket 'profiles' created with public-read policy")
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_HOST"),
	})

	// Проверка связи (Healthcheck)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		logger.Fatal("Redis not reachable", zap.Error(err))
	}

	// Инициализация сервера
	handler := rest.NewHandler(db, writer, s3Client, rdb, logger)

	enabled := func(name startflags.Flag) bool {
		ret := slices.Contains(services, string(name)) || slices.Contains(services, "all")
		return ret
	}

	if enabled(startflags.WebSocket) {
		go handler.StartChatWatcher(ctx, os.Getenv("KAFKA_BROKER"))
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
