package repository

import (
	"fmt"

	"gorm.io/gorm"

	"gopherai-resume/internal/model"
)

type MessageRepository struct {
	db *gorm.DB
}

func NewMessageRepository(db *gorm.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) Create(message *model.Message) error {
	if err := r.db.Create(message).Error; err != nil {
		return fmt.Errorf("create message failed: %w", err)
	}
	return nil
}

func (r *MessageRepository) ListBySessionID(sessionID uint, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	var messages []model.Message
	if err := r.db.Where("session_id = ?", sessionID).Order("created_at ASC").Limit(limit).Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list messages failed: %w", err)
	}
	return messages, nil
}
