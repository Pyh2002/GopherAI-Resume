package repository

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	"gopherai-resume/internal/model"
)

type RAGDocumentRepository struct {
	db *gorm.DB
}

func NewRAGDocumentRepository(db *gorm.DB) *RAGDocumentRepository {
	return &RAGDocumentRepository{db: db}
}

func (r *RAGDocumentRepository) Create(doc *model.RAGDocument) error {
	if err := r.db.Create(doc).Error; err != nil {
		return fmt.Errorf("create rag document failed: %w", err)
	}
	return nil
}

func (r *RAGDocumentRepository) ListByUserID(userID uint) ([]model.RAGDocument, error) {
	var list []model.RAGDocument
	if err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list rag documents failed: %w", err)
	}
	return list, nil
}

// ListByUserIDAndSessionID lists documents for user; if sessionID is 0, lists all user's docs.
func (r *RAGDocumentRepository) ListByUserIDAndSessionID(userID, sessionID uint) ([]model.RAGDocument, error) {
	q := r.db.Where("user_id = ?", userID)
	if sessionID != 0 {
		q = q.Where("session_id = ?", sessionID)
	}
	var list []model.RAGDocument
	if err := q.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("list rag documents failed: %w", err)
	}
	return list, nil
}

// ListBySessionID returns document IDs for a session (for cascade delete).
func (r *RAGDocumentRepository) ListBySessionID(sessionID uint) ([]uint, error) {
	var ids []uint
	if err := r.db.Model(&model.RAGDocument{}).Where("session_id = ?", sessionID).Pluck("id", &ids).Error; err != nil {
		return nil, fmt.Errorf("list rag document ids by session failed: %w", err)
	}
	return ids, nil
}

// DeleteBySessionID deletes all documents in a session (caller must delete chunks first).
func (r *RAGDocumentRepository) DeleteBySessionID(sessionID uint) error {
	if err := r.db.Where("session_id = ?", sessionID).Delete(&model.RAGDocument{}).Error; err != nil {
		return fmt.Errorf("delete rag documents by session failed: %w", err)
	}
	return nil
}

func (r *RAGDocumentRepository) GetByIDAndUserID(id, userID uint) (*model.RAGDocument, error) {
	var doc model.RAGDocument
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&doc).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("get rag document failed: %w", err)
	}
	return &doc, nil
}

func (r *RAGDocumentRepository) DeleteByIDAndUserID(id, userID uint) error {
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.RAGDocument{}).Error; err != nil {
		return fmt.Errorf("delete rag document failed: %w", err)
	}
	return nil
}
