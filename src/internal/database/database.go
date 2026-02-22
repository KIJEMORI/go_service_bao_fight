package database

import (
	"fmt"
	"project/internal/config"

	"gorm.io/gorm"
)

func NewConnection() (*gorm.DB, error) {
	database := config.NewConfig()

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=disable",
		database.Database.Host,
		database.Database.User,
		database.Database.Password,
		database.Database.DbName,
		database.Database.Port,
	)
	db, err := config.NewConnection(dsn)

	return db, err
}
