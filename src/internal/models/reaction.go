package models

import (
	"time"

	"github.com/google/uuid"
)

const (
	ActionLike    = "like"
	ActionDislike = "dislike"
	ActionSuper   = "superlike"
	Match         = "match"
)

type Reaction struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	FromUserID uuid.UUID `gorm:"type:uuid;index" json:"from_user_id"`
	ToUserID   uuid.UUID `gorm:"type:uuid;index" json:"to_user_id"`
	Action     string    `gorm:"type:varchar(20);not null" json:"action"`
	CreatedAt  time.Time `json:"created_at"`
}
