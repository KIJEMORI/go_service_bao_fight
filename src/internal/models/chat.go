package models

import (
	"time"

	"github.com/google/uuid"
)

type Chat struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Title     string    `json:"title"`
	IsGroup   bool      `json:"is_group"`
	CreatedAt time.Time `json:"created_at"`
}
