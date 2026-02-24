package model

import "time"

type RAGSession struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"not null;index" json:"user_id"`
	Title     string    `gorm:"size:256;not null" json:"title"`
	CreatedAt time.Time `json:"created_at"`
}
