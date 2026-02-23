package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
	SessionID uint       `json:"session_id" binding:"required,gt=0"`
	Content   string     `json:"content" binding:"required"`
	LLM       LLMRequest `json:"llm"`
}

type LLMRequest struct {
	BaseURL string `json:"base_url"`
	APIKey  string `json:"api_key"`
	Model   string `json:"model"`
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

func (h *ChatHandler) DeleteSession(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	sessionID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || sessionID64 == 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid session id")
		return
	}

	if err := h.chatService.DeleteSession(userID, uint(sessionID64)); err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrSessionNotFound):
			response.Error(c, http.StatusNotFound, response.CodeSessionNotFound, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "delete session failed")
		}
		return
	}

	response.OK(c, gin.H{"deleted_session_id": uint(sessionID64)})
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

	result, err := h.chatService.SendMessage(app.SendMessageInput{
		UserID:    userID,
		SessionID: req.SessionID,
		Content:   req.Content,
		LLM: app.LLMOverride{
			BaseURL: req.LLM.BaseURL,
			APIKey:  req.LLM.APIKey,
			Model:   req.LLM.Model,
		},
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput), errors.Is(err, app.ErrMessageEmpty):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrLLMConfig):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrMessageEnqueue):
			response.Error(c, http.StatusServiceUnavailable, response.CodeInternalServer, err.Error())
		case errors.Is(err, app.ErrSessionNotFound):
			response.Error(c, http.StatusNotFound, response.CodeSessionNotFound, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "send message failed")
		}
		return
	}

	response.OK(c, result)
}

func (h *ChatHandler) StreamMessage(c *gin.Context) {
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

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "stream not supported")
		return
	}

	full, err := h.chatService.StreamMessage(c.Request.Context(), app.SendMessageInput{
		UserID:    userID,
		SessionID: req.SessionID,
		Content:   req.Content,
		LLM: app.LLMOverride{
			BaseURL: req.LLM.BaseURL,
			APIKey:  req.LLM.APIKey,
			Model:   req.LLM.Model,
		},
	}, func(chunk string) error {
		if _, writeErr := c.Writer.Write([]byte("data: " + chunk + "\n\n")); writeErr != nil {
			return writeErr
		}
		flusher.Flush()
		return nil
	})
	if err != nil {
		if errors.Is(err, app.ErrMessageEnqueue) {
			if _, writeErr := c.Writer.Write([]byte("event: error\ndata: message enqueue failed\n\n")); writeErr == nil {
				flusher.Flush()
			}
			return
		}
		if _, writeErr := c.Writer.Write([]byte(fmt.Sprintf("event: error\ndata: %s\n\n", sanitizeSSE(err.Error())))); writeErr == nil {
			flusher.Flush()
		}
		return
	}

	if _, writeErr := c.Writer.Write([]byte("event: done\ndata: " + sanitizeSSE(full) + "\n\n")); writeErr == nil {
		flusher.Flush()
	}
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

func sanitizeSSE(input string) string {
	replaced := strings.ReplaceAll(input, "\r\n", "\\n")
	replaced = strings.ReplaceAll(replaced, "\n", "\\n")
	return replaced
}
