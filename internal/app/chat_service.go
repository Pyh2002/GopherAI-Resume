package app

import (
	"errors"
	"fmt"
	"strings"

	"gopherai-resume/internal/model"
	"gopherai-resume/internal/repository"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrMessageEmpty    = errors.New("message content is empty")
)

type ChatService struct {
	sessionRepo *repository.SessionRepository
	messageRepo *repository.MessageRepository
}

type CreateSessionInput struct {
	UserID uint
	Title  string
}

type SendMessageInput struct {
	UserID    uint
	SessionID uint
	Content   string
}

func NewChatService(sessionRepo *repository.SessionRepository, messageRepo *repository.MessageRepository) *ChatService {
	return &ChatService{
		sessionRepo: sessionRepo,
		messageRepo: messageRepo,
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

func (s *ChatService) SendMessage(input SendMessageInput) ([]model.Message, error) {
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

	userMessage := &model.Message{
		SessionID: input.SessionID,
		UserID:    input.UserID,
		Role:      "user",
		Content:   content,
	}
	if err := s.messageRepo.Create(userMessage); err != nil {
		return nil, err
	}

	assistantMessage := &model.Message{
		SessionID: input.SessionID,
		UserID:    input.UserID,
		Role:      "assistant",
		Content:   fmt.Sprintf("You said: %s", content),
	}
	if err := s.messageRepo.Create(assistantMessage); err != nil {
		return nil, err
	}

	return []model.Message{*userMessage, *assistantMessage}, nil
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

	return s.messageRepo.ListBySessionID(sessionID, limit)
}
