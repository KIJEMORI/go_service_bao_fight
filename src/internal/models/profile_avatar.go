package models

import (
	"time"

	"github.com/google/uuid"
)

type ProfileAvatar struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;index" json:"user_id"`
	URL       string    `json:"url"`
	IsCurrent bool      `gorm:"default:false" json:"is_current"`
	CreatedAt time.Time `json:"created_at"`
}
