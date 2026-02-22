package config

import (
	"project/internal/config/database"
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

// Config - Главный конфиг приложения
type Config struct {
	Database *database.Config
}

func NewConfig() *Config {
	return &Config{
		Database: &database.Config{
			User:     getEnv("DB_CONFIG_USER", "root"),
			Password: getEnv("DB_CONFIG_PASSWORD", "root"),
			Host:     getEnv("DB_CONFIG_HOST", "localhost"),
			Port:     getEnvAsInt("DB_CONFIG_PORT", 3306),
			DbName:   getEnv("DB_CONFIG_DBNAME", ""),
		},
	}
}
