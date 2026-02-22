package main

// import (
// 	"os"
// 	"project/api/router"
// 	"project/internal/transport/rest"
// 	"time"

// 	"github.com/segmentio/kafka-go"
// )

// func main() {
// 	writer := &kafka.Writer{
// 		Addr:     kafka.TCP(os.Getenv("KAFKA_BROKER")),
// 		Topic:    "messages-topic",
// 		Balancer: &kafka.LeastBytes{}, // Балансировщик: выбирает партицию с наименьшей нагрузкой

// 		// --- Настройки производительности (Батчинг) ---

// 		// Максимальное количество сообщений в одной пачке перед отправкой
// 		BatchSize: 100,

// 		// Максимальный объем пачки (например, 1 МБ)
// 		BatchBytes: 1048576,

// 		// Как долго ждать накопления пачки, если BatchSize еще не достигнут.
// 		// Очень важный параметр: 10-50ms дают огромный прирост пропускной способности.
// 		BatchTimeout: 10 * time.Millisecond,

// 		// --- Надежность ---

// 		// Подтверждение записи:
// 		// RequireNone (не ждать), RequireOne (ждать лидера), RequireAll (ждать все реплики)
// 		RequiredAcks: kafka.RequireOne,

// 		// Количество попыток переотправки при временных ошибках
// 		MaxAttempts: 3,

// 		// Асинхронная отправка. Если true, WriteMessages не будет блокироваться,
// 		Async: false,
// 	}
// 	defer writer.Close()

// 	handler := &rest.Handler{KafkaWriter: writer}

// 	// Инициализируем Echo роутер
// 	e := router.NewRouter(handler)

// 	// Запуск на порту 8080
// 	if err := e.Start(":8080"); err != nil {
// 		e.Logger.Fatal("shutting down the server")
// 	}
// }
