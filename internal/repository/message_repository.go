package repository

import (
	"fmt"
	"slices"

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

func (r *MessageRepository) ListRecentBySessionID(sessionID uint, limit int) ([]model.Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 20
	}

	var messages []model.Message
	if err := r.db.Where("session_id = ?", sessionID).Order("created_at DESC").Limit(limit).Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("list recent messages failed: %w", err)
	}
	slices.Reverse(messages)
	return messages, nil
}

func (r *MessageRepository) DeleteBySessionID(sessionID uint) error {
	if err := r.db.Where("session_id = ?", sessionID).Delete(&model.Message{}).Error; err != nil {
		return fmt.Errorf("delete messages by session failed: %w", err)
	}
	return nil
}
