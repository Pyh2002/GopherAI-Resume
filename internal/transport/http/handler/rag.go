package handler

import (
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"gopherai-resume/internal/app"
	"gopherai-resume/internal/pkg/pdfextract"
	"gopherai-resume/internal/transport/http/response"
)

const maxPDFSize = 10 << 20 // 10 MB

type RAGHandler struct {
	ragService *app.RAGService
}

type CreateRAGSessionRequest struct {
	Title string `json:"title" binding:"max=128"`
}

type CreateRAGDocumentRequest struct {
	Name      string `json:"name"`
	Content   string `json:"content" binding:"required"`
	SessionID uint   `json:"session_id"`
}

type AskRAGRequest struct {
	Question    string  `json:"question" binding:"required"`
	SessionID   uint    `json:"session_id"`
	DocumentIDs []uint  `json:"document_ids"`
	TopK        int     `json:"top_k"`
}

func NewRAGHandler(ragService *app.RAGService) *RAGHandler {
	return &RAGHandler{ragService: ragService}
}

func (h *RAGHandler) CreateSession(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}
	var req CreateRAGSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request payload")
		return
	}
	session, err := h.ragService.CreateSession(app.RAGCreateSessionInput{
		UserID: userID,
		Title:  req.Title,
	})
	if err != nil {
		if errors.Is(err, app.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		} else {
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "create session failed")
		}
		return
	}
	response.OK(c, session)
}

func (h *RAGHandler) ListSessions(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}
	sessions, err := h.ragService.ListSessions(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "list sessions failed")
		return
	}
	response.OK(c, sessions)
}

func (h *RAGHandler) DeleteSession(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}
	sessionID, err := parseUintParam(c, "id")
	if err != nil || sessionID == 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid session id")
		return
	}
	if err := h.ragService.DeleteSession(userID, sessionID); err != nil {
		switch {
		case errors.Is(err, app.ErrRAGSessionNotFound):
			response.Error(c, http.StatusNotFound, response.CodeSessionNotFound, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "delete session failed")
		}
		return
	}
	response.OK(c, gin.H{"deleted_session_id": sessionID})
}

func parseUintParam(c *gin.Context, key string) (uint, error) {
	s := c.Param(key)
	u, err := strconv.ParseUint(s, 10, 64)
	return uint(u), err
}

func (h *RAGHandler) CreateDocument(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	var req CreateRAGDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request payload")
		return
	}

	result, err := h.ragService.Ingest(c.Request.Context(), app.IngestInput{
		UserID:    userID,
		SessionID: req.SessionID,
		Name:      req.Name,
		Content:   req.Content,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "ingest failed: "+err.Error())
		}
		return
	}

	response.OK(c, result)
}

// UploadPDF accepts a multipart form with "file" (PDF) and optional "name", extracts text and ingests.
func (h *RAGHandler) UploadPDF(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "missing file")
		return
	}
	if file.Size > maxPDFSize {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "file too large (max 10MB)")
		return
	}
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".pdf" {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "only PDF files are allowed")
		return
	}

	f, err := file.Open()
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "failed to read file")
		return
	}
	defer f.Close()

	text, err := pdfextract.ExtractText(f)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "failed to extract text from PDF: "+err.Error())
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "PDF contains no extractable text")
		return
	}

	name := strings.TrimSpace(c.PostForm("name"))
	if name == "" {
		name = strings.TrimSuffix(file.Filename, filepath.Ext(file.Filename))
		if name == "" {
			name = "Untitled"
		}
	}

	sessionID := parseUintForm(c, "session_id")

	result, err := h.ragService.Ingest(c.Request.Context(), app.IngestInput{
		UserID:    userID,
		SessionID: sessionID,
		Name:      name,
		Content:   text,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "ingest failed: "+err.Error())
		}
		return
	}

	response.OK(c, result)
}

func parseUintForm(c *gin.Context, key string) uint {
	s := c.PostForm(key)
	if s == "" {
		return 0
	}
	u, _ := strconv.ParseUint(s, 10, 64)
	return uint(u)
}

func (h *RAGHandler) ListDocuments(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}
	sessionID := uint(0)
	if s := c.Query("session_id"); s != "" {
		if u, err := strconv.ParseUint(s, 10, 64); err == nil {
			sessionID = uint(u)
		}
	}

	docs, err := h.ragService.ListDocuments(userID, sessionID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "list documents failed")
		return
	}

	response.OK(c, docs)
}

func (h *RAGHandler) DeleteDocument(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}
	docID, err := parseUintParam(c, "id")
	if err != nil || docID == 0 {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid document id")
		return
	}
	if err := h.ragService.DeleteDocument(userID, docID); err != nil {
		if errors.Is(err, app.ErrInvalidInput) {
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		} else {
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "delete document failed")
		}
		return
	}
	response.OK(c, gin.H{"deleted_document_id": docID})
}

func (h *RAGHandler) Ask(c *gin.Context) {
	userID, ok := getUserIDFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	var req AskRAGRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request payload")
		return
	}

	result, err := h.ragService.Ask(c.Request.Context(), app.AskInput{
		UserID:      userID,
		SessionID:   req.SessionID,
		Question:    req.Question,
		DocumentIDs: req.DocumentIDs,
		TopK:        req.TopK,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrRAGNoDocuments):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrRAGNoChunks):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "ask failed")
		}
		return
	}

	response.OK(c, result)
}
