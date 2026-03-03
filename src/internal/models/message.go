package models

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	SenderID   uuid.UUID `gorm:"type:uuid;index"`
	ReceiverID uuid.UUID `gorm:"type:uuid;index"`
	Text       string    `gorm:"not null"`
	CreatedAt  time.Time `json:"created_at"`
}
