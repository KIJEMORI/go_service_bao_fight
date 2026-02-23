package database

import (
	"errors"
	"fmt"
	"log"
	"project/internal/config"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func RunMigrations(dsn string) error {
	// Формируем URL для мигратора (библиотека требует префикс postgres://)
	m, err := migrate.New(
		"file://migrations", // Путь к папке с .sql файлами
		dsn,
	)
	if err != nil {
		return fmt.Errorf("could not create migrate instance: %w", err)
	}

	// Применяем все новые миграции
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("could not run up migrations: %w", err)
	}

	log.Println("Migrations applied successfully!")
	return nil
}

func GetMigrationDSN() string {
	cfg := config.NewConfig()
	// Используем формат postgres://user:password@host:port/dbname
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DbName,
	)
}
