package models

import (
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	ChatID         uuid.UUID      `gorm:"type:uuid;index" json:"chat_id"`
	SenderID       uuid.UUID      `gorm:"type:uuid;index" json:"sender_id"`
	Text           string         `gorm:"not null" json:"text"`
	Payload        map[string]any `gorm:"type:jsonb" json:"payload,omitempty"`
	SequenceNumber uint64         `gorm:"autoIncrement" json:"seq"`
	CreatedAt      time.Time      `json:"created_at"`
}
