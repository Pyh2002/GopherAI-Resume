package model

import "time"

type Message struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	SessionID uint      `gorm:"not null;index" json:"session_id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Role      string    `gorm:"size:16;not null;index" json:"role"`
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
