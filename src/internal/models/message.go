package models

import "time"

type Message struct {
	ID        uint `gorm:"primaryKey"`
	Content   string
	CreatedAt time.Time
}
