package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"gopherai-resume/internal/model"
)

type SessionRepository struct {
	db *gorm.DB
}

func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(session *model.Session) error {
	if err := r.db.Create(session).Error; err != nil {
		return fmt.Errorf("create session failed: %w", err)
	}
	return nil
}

func (r *SessionRepository) ListByUserID(userID uint) ([]model.Session, error) {
	var sessions []model.Session
	if err := r.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&sessions).Error; err != nil {
		return nil, fmt.Errorf("list sessions failed: %w", err)
	}
	return sessions, nil
}

func (r *SessionRepository) GetByIDAndUserID(sessionID, userID uint) (*model.Session, error) {
	var session model.Session
	if err := r.db.Where("id = ? AND user_id = ?", sessionID, userID).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get session failed: %w", err)
	}
	return &session, nil
}

func (r *SessionRepository) DeleteByIDAndUserID(sessionID, userID uint) error {
	if err := r.db.Where("id = ? AND user_id = ?", sessionID, userID).Delete(&model.Session{}).Error; err != nil {
		return fmt.Errorf("delete session failed: %w", err)
	}
	return nil
}
