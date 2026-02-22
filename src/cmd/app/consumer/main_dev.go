package main

// import (
// 	"context"
// 	"fmt"
// 	"log"
// 	"os"
// 	"os/signal"
// 	"project/internal/config"
// 	"project/internal/models"
// 	"syscall"
// 	"time"

// 	"github.com/segmentio/kafka-go"
// 	"gorm.io/gorm"
// )

// func main() {

// 	config.LoadEnvironment()

// 	database := config.NewConfig()

// 	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
// 		database.Database.Host,
// 		database.Database.User,
// 		database.Database.Password,
// 		database.Database.DbName,
// 		database.Database.Port,
// 	)
// 	db, err := config.NewConnection(dsn)
// 	if err != nil {
// 		log.Fatal("DB connection failed:", err)
// 	}
// 	db.AutoMigrate(&models.Message{})

// 	reader := kafka.NewReader(kafka.ReaderConfig{
// 		Brokers: []string{os.Getenv("KAFKA_BROKER")},
// 		Topic:   "messages-topic",
// 		GroupID: "worker-v2",
// 	})

// 	// Канал для передачи сообщений от читателя к накопителю
// 	msgChan := make(chan string, 1000)

// 	// Каналы для сигналов и завершения
// 	sigChan := make(chan os.Signal, 1)
// 	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

// 	batchSize := 100                 // Сохраняем в БД по 100 сообщений
// 	flushInterval := 1 * time.Second // Или каждую секунду, если сообщений мало

// 	log.Println("Consumer started...")

// 	// Чтение в потоке: только забирает данные из Kafka и кидает в канал
// 	go func() {
// 		for {
// 			m, err := reader.ReadMessage(context.Background())
// 			if err != nil {
// 				log.Printf("read error: %v", err)
// 				return
// 			}
// 			msgChan <- string(m.Value)
// 		}
// 	}()

// 	var buffer = make([]models.Message, 0, batchSize)
// 	ticker := time.NewTicker(flushInterval)

// 	for {
// 		select {
// 		case msg := <-msgChan:
// 			// Добавляем сообщение в буфер сразу по приходу
// 			buffer = append(buffer, models.Message{
// 				Content:   msg,
// 				CreatedAt: time.Now(),
// 			})

// 			// Если накопили много — сбрасываем не дожидаясь таймера
// 			if len(buffer) >= 100 {
// 				flush(db, &buffer)
// 				ticker.Reset(1 * time.Second) // Сбрасываем таймер после записи
// 			}
// 		case <-sigChan:
// 			log.Println("Stopping consumer gracefully...")
// 			if len(buffer) > 0 {
// 				log.Printf("Flushing last %d messages before exit...", len(buffer))
// 				flush(db, &buffer)
// 			}
// 			reader.Close()
// 			return
// 		case <-ticker.C:
// 			// Сброс по таймеру
// 			if len(buffer) > 0 {
// 				flush(db, &buffer)
// 			}

// 			// default:
// 			// 	// Читаем сообщение с коротким таймаутом, чтобы не блокировать тикер
// 			// 	readCtx, timeout := context.WithTimeout(ctx, 1*time.Second)
// 			// 	m, err := reader.ReadMessage(readCtx)
// 			// 	timeout()

// 			// 	if err != nil {
// 			// 		if err == context.DeadlineExceeded {
// 			// 			continue // Просто таймаут чтения, идем на следующую итерацию
// 			// 		}
// 			// 		if err == context.Canceled {
// 			// 			break Loop
// 			// 		}
// 			// 		log.Printf("Read error: %v", err)
// 			// 		continue
// 			// 	}

// 			// 	// Добавляем в буфер
// 			// 	buffer = append(buffer, models.Message{
// 			// 		Content:   string(m.Value),
// 			// 		CreatedAt: time.Now(),
// 			// 	})

// 			// 	// Сброс по достижении размера пачки
// 			// 	if len(buffer) >= batchSize {
// 			// 		flush(db, &buffer)
// 			// 	}
// 		}
// 	}
// }

// // Вспомогательная функция для Bulk Insert
// func flush(db *gorm.DB, buffer *[]models.Message) {
// 	// Используем CreateInBatches для эффективной вставки
// 	err := db.CreateInBatches(*buffer, len(*buffer)).Error
// 	if err != nil {
// 		log.Printf("Failed to save batch: %v", err)
// 	} else {
// 		log.Printf("Successfully flushed %d messages to DB", len(*buffer))
// 	}
// 	// Очищаем буфер, сохраняя аллоцированную память
// 	*buffer = (*buffer)[:0]
// }
