package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"gopherai-resume/internal/app"
	"gopherai-resume/internal/transport/http/middleware"
	"gopherai-resume/internal/transport/http/response"
)

type ChatHandler struct {
	chatService *app.ChatService
}

type CreateSessionRequest struct {
	Title string `json:"title" binding:"max=128"`
}

type SendMessageRequest struct {
	SessionID uint   `json:"session_id" binding:"required,gt=0"`
	Content   string `json:"content" binding:"required"`
}

func NewChatHandler(chatService *app.ChatService) *ChatHandler {
	return &ChatHandler{chatService: chatService}
}

func (h *ChatHandler) CreateSession(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request payload")
		return
	}

	session, err := h.chatService.CreateSession(app.CreateSessionInput{
		UserID: userID,
		Title:  req.Title,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "create session failed")
		}
		return
	}

	response.OK(c, session)
}

func (h *ChatHandler) ListSessions(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	sessions, err := h.chatService.ListSessions(userID)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "list sessions failed")
		}
		return
	}

	response.OK(c, sessions)
}

func (h *ChatHandler) SendMessage(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request payload")
		return
	}

	messages, err := h.chatService.SendMessage(app.SendMessageInput{
		UserID:    userID,
		SessionID: req.SessionID,
		Content:   req.Content,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput), errors.Is(err, app.ErrMessageEmpty):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrSessionNotFound):
			response.Error(c, http.StatusNotFound, response.CodeSessionNotFound, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "send message failed")
		}
		return
	}

	response.OK(c, messages)
}

func (h *ChatHandler) GetHistory(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	sessionIDRaw := c.Query("session_id")
	sessionID64, err := strconv.ParseUint(sessionIDRaw, 10, 64)
	if err != nil || sessionID64 == 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid session_id")
		return
	}

	limit := 100
	if raw := c.Query("limit"); raw != "" {
		if parsed, parseErr := strconv.Atoi(raw); parseErr == nil {
			limit = parsed
		}
	}

	history, err := h.chatService.GetHistory(userID, uint(sessionID64), limit)
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrSessionNotFound):
			response.Error(c, http.StatusNotFound, response.CodeSessionNotFound, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "get history failed")
		}
		return
	}

	response.OK(c, history)
}

func getUserIDFromContext(c *gin.Context) (uint, bool) {
	userIDAny, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := userIDAny.(uint)
	return userID, ok
}
