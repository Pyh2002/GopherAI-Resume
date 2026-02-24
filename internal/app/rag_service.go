package app

import (
	"context"
	"errors"
	"strings"

	"gopherai-resume/internal/ai"
	"gopherai-resume/internal/model"
	"gopherai-resume/internal/repository"
)

const (
	defaultChunkSize    = 512
	defaultChunkOverlap = 64
	defaultTopK         = 5
	embeddingBatchSize  = 10 // DashScope and similar APIs often limit batch size
)

var (
	ErrRAGNoDocuments   = errors.New("no documents to search")
	ErrRAGNoChunks      = errors.New("no chunks found for retrieval")
	ErrRAGSessionNotFound = errors.New("rag session not found")
)

type RAGService struct {
	sessionRepo *repository.RAGSessionRepository
	docRepo     *repository.RAGDocumentRepository
	chunkRepo   *repository.RAGChunkRepository
	llmClient   *ai.OpenAICompatibleClient
	embConfig   ai.EmbeddingConfig
	chatConfig  ai.ChatConfig
}

func NewRAGService(
	sessionRepo *repository.RAGSessionRepository,
	docRepo *repository.RAGDocumentRepository,
	chunkRepo *repository.RAGChunkRepository,
	llmClient *ai.OpenAICompatibleClient,
	embConfig ai.EmbeddingConfig,
	chatConfig ai.ChatConfig,
) *RAGService {
	return &RAGService{
		sessionRepo: sessionRepo,
		docRepo:     docRepo,
		chunkRepo:   chunkRepo,
		llmClient:   llmClient,
		embConfig:   embConfig,
		chatConfig:  chatConfig,
	}
}

// RAGCreateSessionInput for creating a RAG session.
type RAGCreateSessionInput struct {
	UserID uint
	Title  string
}

// CreateSession creates a new RAG session.
func (s *RAGService) CreateSession(input RAGCreateSessionInput) (*model.RAGSession, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "New RAG"
	}
	session := &model.RAGSession{UserID: input.UserID, Title: title}
	if err := s.sessionRepo.Create(session); err != nil {
		return nil, err
	}
	return session, nil
}

// ListSessions returns all RAG sessions for the user.
func (s *RAGService) ListSessions(userID uint) ([]model.RAGSession, error) {
	if userID == 0 {
		return nil, ErrInvalidInput
	}
	return s.sessionRepo.ListByUserID(userID)
}

// DeleteSession deletes a RAG session and all its documents (and chunks).
func (s *RAGService) DeleteSession(userID, sessionID uint) error {
	if userID == 0 || sessionID == 0 {
		return ErrInvalidInput
	}
	session, err := s.sessionRepo.GetByIDAndUserID(sessionID, userID)
	if err != nil || session == nil {
		return ErrRAGSessionNotFound
	}
	docIDs, err := s.docRepo.ListBySessionID(sessionID)
	if err != nil {
		return err
	}
	for _, docID := range docIDs {
		_ = s.chunkRepo.DeleteByDocumentID(docID)
	}
	if err := s.docRepo.DeleteBySessionID(sessionID); err != nil {
		return err
	}
	return s.sessionRepo.DeleteByIDAndUserID(sessionID, userID)
}

// DeleteDocument deletes a document and its chunks.
func (s *RAGService) DeleteDocument(userID, documentID uint) error {
	if userID == 0 || documentID == 0 {
		return ErrInvalidInput
	}
	doc, err := s.docRepo.GetByIDAndUserID(documentID, userID)
	if err != nil || doc == nil {
		return ErrInvalidInput
	}
	if err := s.chunkRepo.DeleteByDocumentID(doc.ID); err != nil {
		return err
	}
	return s.docRepo.DeleteByIDAndUserID(doc.ID, userID)
}

// IngestInput is the input for adding a document.
type IngestInput struct {
	UserID    uint
	SessionID uint // 0 = no session
	Name      string
	Content   string
}

// IngestResult is the result of document ingest.
type IngestResult struct {
	Document   model.RAGDocument `json:"document"`
	ChunkCount int              `json:"chunk_count"`
}

// ListDocuments returns RAG documents for the user; if sessionID is 0, returns all.
func (s *RAGService) ListDocuments(userID, sessionID uint) ([]model.RAGDocument, error) {
	if userID == 0 {
		return nil, ErrInvalidInput
	}
	return s.docRepo.ListByUserIDAndSessionID(userID, sessionID)
}

// Ingest chunks the content, embeds each chunk, and persists document + chunks.
func (s *RAGService) Ingest(ctx context.Context, input IngestInput) (*IngestResult, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, ErrInvalidInput
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = "Untitled"
	}

	chunks := chunkText(content, defaultChunkSize, defaultChunkOverlap)
	if len(chunks) == 0 {
		return nil, ErrInvalidInput
	}

	doc := &model.RAGDocument{
		UserID:    input.UserID,
		SessionID: input.SessionID,
		Name:      name,
	}
	if err := s.docRepo.Create(doc); err != nil {
		return nil, err
	}

	// Call embedding API in batches to avoid provider limits.
	var embeddings [][]float32
	for i := 0; i < len(chunks); i += embeddingBatchSize {
		end := i + embeddingBatchSize
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		batched, err := s.llmClient.EmbedBatch(ctx, s.embConfig, batch)
		if err != nil {
			return nil, err
		}
		embeddings = append(embeddings, batched...)
	}
	if len(embeddings) != len(chunks) {
		return nil, errors.New("embedding count mismatch")
	}

	ragChunks := make([]model.RAGChunk, len(chunks))
	for i := range chunks {
		ragChunks[i] = model.RAGChunk{
			DocumentID: doc.ID,
			Content:    chunks[i],
		}
		ragChunks[i].SetEmbedding(embeddings[i])
	}
	if err := s.chunkRepo.CreateBatch(ragChunks); err != nil {
		return nil, err
	}

	return &IngestResult{
		Document:   *doc,
		ChunkCount: len(ragChunks),
	}, nil
}

// AskInput is the input for RAG ask.
type AskInput struct {
	UserID      uint
	SessionID   uint   // if non-zero, search only docs in this session
	Question    string
	DocumentIDs []uint // empty = search by session or all user's documents
	TopK        int
}

// AskResult is the result of RAG ask (answer + used chunks).
type AskResult struct {
	Answer   string           `json:"answer"`
	Chunks   []model.RAGChunk `json:"chunks"`
}

// Ask retrieves top-k relevant chunks, builds a prompt with them, and calls the LLM.
func (s *RAGService) Ask(ctx context.Context, input AskInput) (*AskResult, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidInput
	}
	question := strings.TrimSpace(input.Question)
	if question == "" {
		return nil, ErrInvalidInput
	}

	topK := input.TopK
	if topK <= 0 {
		topK = defaultTopK
	}

	var docIDs []uint
	if len(input.DocumentIDs) > 0 {
		for _, id := range input.DocumentIDs {
			doc, err := s.docRepo.GetByIDAndUserID(id, input.UserID)
			if err != nil || doc == nil {
				continue
			}
			docIDs = append(docIDs, id)
		}
	} else {
		var docs []model.RAGDocument
		var err error
		if input.SessionID != 0 {
			docs, err = s.docRepo.ListByUserIDAndSessionID(input.UserID, input.SessionID)
		} else {
			docs, err = s.docRepo.ListByUserID(input.UserID)
		}
		if err != nil {
			return nil, err
		}
		if len(docs) == 0 {
			return nil, ErrRAGNoDocuments
		}
		for _, d := range docs {
			docIDs = append(docIDs, d.ID)
		}
	}
	if len(docIDs) == 0 {
		return nil, ErrRAGNoDocuments
	}

	allChunks, err := s.chunkRepo.ListByDocumentIDs(docIDs)
	if err != nil {
		return nil, err
	}
	if len(allChunks) == 0 {
		return nil, ErrRAGNoChunks
	}

	queryEmb, err := s.llmClient.Embed(ctx, s.embConfig, question)
	if err != nil {
		return nil, err
	}

	scored := make([]struct {
		chunk model.RAGChunk
		score float32
	}, len(allChunks))
	for i := range allChunks {
		vec := allChunks[i].EmbeddingVector()
		scored[i].chunk = allChunks[i]
		scored[i].score = cosineSimilarity(queryEmb, vec)
	}
	top := topKScored(scored, topK)

	selectedChunks := make([]model.RAGChunk, len(top))
	for i := range top {
		selectedChunks[i] = top[i].chunk
	}

	contextBlock := ""
	for i, c := range selectedChunks {
		contextBlock += "\n---\n" + c.Content
		if i == len(selectedChunks)-1 {
			contextBlock += "\n---"
		}
	}

	systemContent := "You are a helpful assistant. Answer the user's question based only on the following context. If the context does not contain enough information, say so. Do not make up facts."
	userContent := "Context:" + contextBlock + "\n\nQuestion: " + question + "\n\nAnswer:"

	messages := []ai.ChatMessage{
		{Role: "system", Content: systemContent},
		{Role: "user", Content: userContent},
	}
	answer, err := s.llmClient.Complete(ctx, s.chatConfig, messages)
	if err != nil {
		return nil, err
	}

	return &AskResult{
		Answer: strings.TrimSpace(answer),
		Chunks: selectedChunks,
	}, nil
}

// chunkText splits text into overlapping chunks by rune count.
func chunkText(text string, size, overlap int) []string {
	if size <= 0 {
		size = defaultChunkSize
	}
	if overlap >= size {
		overlap = size / 2
	}
	var chunks []string
	runes := []rune(text)
	for i := 0; i < len(runes); {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		chunks = append(chunks, chunk)
		i += size - overlap
		if i >= len(runes) {
			break
		}
	}
	return chunks
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA <= 0 || normB <= 0 {
		return 0
	}
	return dot / (float32(mathSqrt(float64(normA))) * float32(mathSqrt(float64(normB))))
}

func mathSqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Newton's method for sqrt
	t := x
	for i := 0; i < 20; i++ {
		next := 0.5 * (t + x/t)
		if next == t {
			return t
		}
		t = next
	}
	return t
}

func topKScored(scored []struct {
	chunk model.RAGChunk
	score float32
}, k int) []struct {
	chunk model.RAGChunk
	score float32
} {
	if k <= 0 || len(scored) == 0 {
		return nil
	}
	// simple sort descending by score
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].score > scored[i].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
	if k > len(scored) {
		k = len(scored)
	}
	return scored[:k]
}
