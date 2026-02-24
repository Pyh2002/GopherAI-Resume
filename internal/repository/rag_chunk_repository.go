package repository

import (
	"fmt"

	"gorm.io/gorm"

	"gopherai-resume/internal/model"
)

type RAGChunkRepository struct {
	db *gorm.DB
}

func NewRAGChunkRepository(db *gorm.DB) *RAGChunkRepository {
	return &RAGChunkRepository{db: db}
}

func (r *RAGChunkRepository) Create(chunk *model.RAGChunk) error {
	if err := r.db.Create(chunk).Error; err != nil {
		return fmt.Errorf("create rag chunk failed: %w", err)
	}
	return nil
}

func (r *RAGChunkRepository) CreateBatch(chunks []model.RAGChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	if err := r.db.Create(&chunks).Error; err != nil {
		return fmt.Errorf("create rag chunks batch failed: %w", err)
	}
	return nil
}

// ListByDocumentIDs returns all chunks for the given document IDs (for a user's docs).
// Caller should filter document IDs by user ownership.
func (r *RAGChunkRepository) ListByDocumentIDs(documentIDs []uint) ([]model.RAGChunk, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}
	var chunks []model.RAGChunk
	if err := r.db.Where("document_id IN ?", documentIDs).Find(&chunks).Error; err != nil {
		return nil, fmt.Errorf("list rag chunks by document ids failed: %w", err)
	}
	return chunks, nil
}

func (r *RAGChunkRepository) DeleteByDocumentID(documentID uint) error {
	if err := r.db.Where("document_id = ?", documentID).Delete(&model.RAGChunk{}).Error; err != nil {
		return fmt.Errorf("delete rag chunks by document failed: %w", err)
	}
	return nil
}
