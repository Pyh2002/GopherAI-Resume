package model

import "time"

type RAGDocument struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"not null;index" json:"user_id"`
	SessionID  uint      `gorm:"index" json:"session_id"` // 0 = no session
	Name       string    `gorm:"size:256;not null" json:"name"`
	CreatedAt  time.Time `json:"created_at"`
}
