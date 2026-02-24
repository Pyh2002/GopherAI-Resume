package model

import (
	"encoding/json"
	"time"
)

// RAGChunk stores a text chunk and its embedding for retrieval.
// Embedding is stored as JSON array of float32 for portability.
type RAGChunk struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DocumentID uint      `gorm:"not null;index" json:"document_id"`
	Content    string    `gorm:"type:text;not null" json:"content"`
	Embedding  string    `gorm:"type:text" json:"-"` // JSON array of float32
	CreatedAt  time.Time `json:"created_at"`
}

// EmbeddingVector returns the parsed embedding slice; empty on parse error.
func (c *RAGChunk) EmbeddingVector() []float32 {
	if c.Embedding == "" {
		return nil
	}
	var v []float32
	_ = json.Unmarshal([]byte(c.Embedding), &v)
	return v
}

// SetEmbedding stores the embedding as JSON.
func (c *RAGChunk) SetEmbedding(vec []float32) {
	if len(vec) == 0 {
		c.Embedding = "[]"
		return
	}
	b, _ := json.Marshal(vec)
	c.Embedding = string(b)
}
