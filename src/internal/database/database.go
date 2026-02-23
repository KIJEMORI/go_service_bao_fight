package database

import (
	"fmt"
	"project/internal/config"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewConnection(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Настройка пула
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)           // Сколько свободных соединений держать
	sqlDB.SetMaxOpenConns(100)          // Максимум одновременных соединений
	sqlDB.SetConnMaxLifetime(time.Hour) // Время жизни соединения

	return db, nil
}

func GetDSN() string {
	cfg := config.NewConfig()
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		cfg.Database.Host,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DbName,
		cfg.Database.Port,
	)
}
