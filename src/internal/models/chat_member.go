package models

import (
	"time"

	"github.com/google/uuid"
)

type ChatMember struct {
	ChatID   uuid.UUID `gorm:"primaryKey" json:"chat_id"`
	UserID   uuid.UUID `gorm:"primaryKey" json:"user_id"`
	JoinedAt time.Time `json:"joined_at"`
}
