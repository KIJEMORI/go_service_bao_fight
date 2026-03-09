package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Profile struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID     uuid.UUID `gorm:"type:uuid;uniqueIndex" json:"user_id"`
	NickName   string    `gorm:"uniqueIndex;not null" json:"nick_name"`
	UserName   string    `gorm:"not null" json:"user_name"`
	Age        uint32    `gorm:"not null"`
	Gender     string    `gorm:"not null"`
	Sport      string    ``
	Level      string    ``
	Experience string    ``
	Weight     string    `gorm:"not null"`
	PhotoURL   string    ``
	CreatedAt  time.Time ``
	UpdatedAt  time.Time ``
}

func (u *Profile) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return
}
