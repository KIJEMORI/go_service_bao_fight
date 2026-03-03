package models

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;index"`
	RefreshToken string    `gorm:"uniqueIndex"`
	UserAgent    string    // Инфо об устройстве (Chrome, iPhone и т.д.)
	ClientIP     string
	ExpiresAt    time.Time `gorm:"index"`
	CreatedAt    time.Time
}
