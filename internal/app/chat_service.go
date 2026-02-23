package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"gopherai-resume/internal/ai"
	"gopherai-resume/internal/model"
	"gopherai-resume/internal/repository"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrMessageEmpty    = errors.New("message content is empty")
	ErrLLMConfig       = errors.New("llm config is invalid")
	ErrMessageEnqueue  = errors.New("message enqueue failed")
)

type ChatService struct {
	sessionRepo  *repository.SessionRepository
	messageRepo  *repository.MessageRepository
	publisher    AsyncMessagePublisher
	historyCache HistoryCache
	llmClient    *ai.OpenAICompatibleClient
	defaultLLM   ai.ChatConfig
	maxContext   int
}

type AsyncMessagePublisher interface {
	Publish(ctx context.Context, msg model.Message) error
}

type HistoryCache interface {
	GetHistory(ctx context.Context, sessionID uint) ([]model.Message, bool, error)
	SetHistory(ctx context.Context, sessionID uint, messages []model.Message) error
	DeleteHistory(ctx context.Context, sessionID uint) error
	MarkDirty(ctx context.Context, sessionID uint) error
	IsDirty(ctx context.Context, sessionID uint) (bool, error)
}

type CreateSessionInput struct {
	UserID uint
	Title  string
}

type SendMessageInput struct {
	UserID    uint
	SessionID uint
	Content   string
	LLM       LLMOverride
}

type LLMRequestLog struct {
	BaseURL      string           `json:"base_url"`
	Model        string           `json:"model"`
	APIKeyMasked string           `json:"api_key_masked"`
	Messages     []ai.ChatMessage `json:"messages"`
}

type SendMessageResult struct {
	Messages   []model.Message `json:"messages"`
	LLMRequest LLMRequestLog   `json:"llm_request"`
}

type LLMOverride struct {
	BaseURL string
	APIKey  string
	Model   string
}

func NewChatService(
	sessionRepo *repository.SessionRepository,
	messageRepo *repository.MessageRepository,
	publisher AsyncMessagePublisher,
	historyCache HistoryCache,
	defaultLLM ai.ChatConfig,
	maxContext int,
) *ChatService {
	if maxContext <= 0 {
		maxContext = 20
	}
	return &ChatService{
		sessionRepo:  sessionRepo,
		messageRepo:  messageRepo,
		publisher:    publisher,
		historyCache: historyCache,
		llmClient:    ai.NewOpenAICompatibleClient(),
		defaultLLM:   defaultLLM,
		maxContext:   maxContext,
	}
}

func (s *ChatService) CreateSession(input CreateSessionInput) (*model.Session, error) {
	if input.UserID == 0 {
		return nil, ErrInvalidInput
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		title = "New Chat"
	}

	session := &model.Session{
		UserID: input.UserID,
		Title:  title,
	}
	if err := s.sessionRepo.Create(session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *ChatService) ListSessions(userID uint) ([]model.Session, error) {
	if userID == 0 {
		return nil, ErrInvalidInput
	}
	return s.sessionRepo.ListByUserID(userID)
}

func (s *ChatService) DeleteSession(userID, sessionID uint) error {
	if userID == 0 || sessionID == 0 {
		return ErrInvalidInput
	}
	session, err := s.sessionRepo.GetByIDAndUserID(sessionID, userID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrSessionNotFound
	}
	if err := s.messageRepo.DeleteBySessionID(sessionID); err != nil {
		return err
	}
	if err := s.sessionRepo.DeleteByIDAndUserID(sessionID, userID); err != nil {
		return err
	}
	if s.historyCache != nil {
		_ = s.historyCache.DeleteHistory(context.Background(), sessionID)
	}
	return nil
}

func (s *ChatService) SendMessage(input SendMessageInput) (*SendMessageResult, error) {
	if input.UserID == 0 || input.SessionID == 0 {
		return nil, ErrInvalidInput
	}

	content := strings.TrimSpace(input.Content)
	if content == "" {
		return nil, ErrMessageEmpty
	}

	session, err := s.sessionRepo.GetByIDAndUserID(input.SessionID, input.UserID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	cfg, err := s.resolveLLM(input.LLM)
	if err != nil {
		return nil, err
	}
	promptMessages, err := s.buildPromptMessages(input.SessionID, content)
	if err != nil {
		return nil, err
	}

	userMessage := &model.Message{
		SessionID: input.SessionID,
		UserID:    input.UserID,
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}
	if s.publisher == nil {
		return nil, ErrMessageEnqueue
	}
	if s.historyCache != nil {
		_ = s.historyCache.MarkDirty(context.Background(), input.SessionID)
		_ = s.historyCache.DeleteHistory(context.Background(), input.SessionID)
	}
	if err := s.publisher.Publish(context.Background(), *userMessage); err != nil {
		return nil, ErrMessageEnqueue
	}
	assistantContent, err := s.llmClient.Complete(context.Background(), cfg, promptMessages)
	if err != nil {
		return nil, err
	}
	assistantContent = strings.TrimSpace(assistantContent)
	if assistantContent == "" {
		assistantContent = "The model returned an empty response."
	}

	assistantMessage := &model.Message{
		SessionID: input.SessionID,
		UserID:    input.UserID,
		Role:      "assistant",
		Content:   assistantContent,
		CreatedAt: time.Now(),
	}
	if err := s.publisher.Publish(context.Background(), *assistantMessage); err != nil {
		return nil, ErrMessageEnqueue
	}

	return &SendMessageResult{
		Messages: []model.Message{*userMessage, *assistantMessage},
		LLMRequest: LLMRequestLog{
			BaseURL:      cfg.BaseURL,
			Model:        cfg.Model,
			APIKeyMasked: maskSecret(cfg.APIKey),
			Messages:     promptMessages,
		},
	}, nil
}

func (s *ChatService) GetHistory(userID, sessionID uint, limit int) ([]model.Message, error) {
	if userID == 0 || sessionID == 0 {
		return nil, ErrInvalidInput
	}

	session, err := s.sessionRepo.GetByIDAndUserID(sessionID, userID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}

	ctx := context.Background()
	if s.historyCache != nil {
		dirty, err := s.historyCache.IsDirty(ctx, sessionID)
		if err == nil && !dirty {
			if cached, hit, cacheErr := s.historyCache.GetHistory(ctx, sessionID); cacheErr == nil && hit {
				return trimMessages(cached, limit), nil
			}
		}
	}

	messages, err := s.messageRepo.ListBySessionID(sessionID, limit)
	if err != nil {
		return nil, err
	}
	if s.historyCache != nil {
		if dirty, dirtyErr := s.historyCache.IsDirty(ctx, sessionID); dirtyErr == nil && !dirty {
			_ = s.historyCache.SetHistory(ctx, sessionID, messages)
		}
	}
	return messages, nil
}

func (s *ChatService) StreamMessage(
	ctx context.Context,
	input SendMessageInput,
	onChunk func(string) error,
) (string, error) {
	if input.UserID == 0 || input.SessionID == 0 {
		return "", ErrInvalidInput
	}
	content := strings.TrimSpace(input.Content)
	if content == "" {
		return "", ErrMessageEmpty
	}

	session, err := s.sessionRepo.GetByIDAndUserID(input.SessionID, input.UserID)
	if err != nil {
		return "", err
	}
	if session == nil {
		return "", ErrSessionNotFound
	}

	cfg, err := s.resolveLLM(input.LLM)
	if err != nil {
		return "", err
	}
	promptMessages, err := s.buildPromptMessages(input.SessionID, content)
	if err != nil {
		return "", err
	}

	userMessage := &model.Message{
		SessionID: input.SessionID,
		UserID:    input.UserID,
		Role:      "user",
		Content:   content,
		CreatedAt: time.Now(),
	}
	if s.publisher == nil {
		return "", ErrMessageEnqueue
	}
	if s.historyCache != nil {
		_ = s.historyCache.MarkDirty(ctx, input.SessionID)
		_ = s.historyCache.DeleteHistory(ctx, input.SessionID)
	}
	if err := s.publisher.Publish(ctx, *userMessage); err != nil {
		return "", ErrMessageEnqueue
	}

	full, err := s.llmClient.StreamComplete(ctx, cfg, promptMessages, onChunk)
	if err != nil {
		return "", err
	}
	full = strings.TrimSpace(full)
	if full == "" {
		full = "The model returned an empty response."
	}

	assistantMessage := &model.Message{
		SessionID: input.SessionID,
		UserID:    input.UserID,
		Role:      "assistant",
		Content:   full,
		CreatedAt: time.Now(),
	}
	if err := s.publisher.Publish(ctx, *assistantMessage); err != nil {
		return "", ErrMessageEnqueue
	}

	return full, nil
}

func trimMessages(messages []model.Message, limit int) []model.Message {
	if limit <= 0 || limit >= len(messages) {
		return messages
	}
	return messages[len(messages)-limit:]
}

func (s *ChatService) resolveLLM(override LLMOverride) (ai.ChatConfig, error) {
	cfg := s.defaultLLM
	if strings.TrimSpace(override.BaseURL) != "" {
		cfg.BaseURL = strings.TrimSpace(override.BaseURL)
	}
	if strings.TrimSpace(override.APIKey) != "" {
		cfg.APIKey = strings.TrimSpace(override.APIKey)
	}
	if strings.TrimSpace(override.Model) != "" {
		cfg.Model = strings.TrimSpace(override.Model)
	}
	if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
		return ai.ChatConfig{}, ErrLLMConfig
	}
	return cfg, nil
}

func maskSecret(secret string) string {
	if len(secret) <= 8 {
		return "****"
	}
	return secret[:4] + strings.Repeat("*", len(secret)-8) + secret[len(secret)-4:]
}

func (s *ChatService) buildPromptMessages(sessionID uint, currentUserInput string) ([]ai.ChatMessage, error) {
	recent, err := s.messageRepo.ListRecentBySessionID(sessionID, s.maxContext)
	if err != nil {
		return nil, err
	}

	messages := make([]ai.ChatMessage, 0, len(recent)+1)
	messages = append(messages, ai.ChatMessage{
		Role:    "system",
		Content: "You are a concise and helpful AI assistant.",
	})
	for _, item := range recent {
		role := item.Role
		if role == "" {
			role = "user"
		}
		messages = append(messages, ai.ChatMessage{
			Role:    role,
			Content: item.Content,
		})
	}
	if strings.TrimSpace(currentUserInput) != "" {
		messages = append(messages, ai.ChatMessage{
			Role:    "user",
			Content: strings.TrimSpace(currentUserInput),
		})
	}
	return messages, nil
}
