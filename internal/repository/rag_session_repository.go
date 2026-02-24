package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"gopherai-resume/internal/model"
)

type RAGSessionRepository struct {
	db *gorm.DB
}

func NewRAGSessionRepository(db *gorm.DB) *RAGSessionRepository {
	return &RAGSessionRepository{db: db}
}

func (r *RAGSessionRepository) Create(session *model.RAGSession) error {
	if err := r.db.Create(session).Error; err != nil {
		return fmt.Errorf("create rag session failed: %w", err)
	}
	return nil
}

func (r *RAGSessionRepository) ListByUserID(userID uint) ([]model.RAGSession, error) {
	var list []model.RAGSession
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list rag sessions failed: %w", err)
	}
	return list, nil
}

func (r *RAGSessionRepository) GetByIDAndUserID(id, userID uint) (*model.RAGSession, error) {
	var session model.RAGSession
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get rag session failed: %w", err)
	}
	return &session, nil
}

func (r *RAGSessionRepository) DeleteByIDAndUserID(id, userID uint) error {
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.RAGSession{}).Error; err != nil {
		return fmt.Errorf("delete rag session failed: %w", err)
	}
	return nil
}
