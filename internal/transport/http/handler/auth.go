package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"gopherai-resume/internal/app"
	"gopherai-resume/internal/transport/http/middleware"
	"gopherai-resume/internal/transport/http/response"
)

type AuthHandler struct {
	authService *app.AuthService
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Email    string `json:"email" binding:"required,email,max=128"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=64"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

func NewAuthHandler(authService *app.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request payload")
		return
	}

	result, err := h.authService.Register(app.RegisterInput{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrUsernameExists):
			response.Error(c, http.StatusBadRequest, response.CodeUsernameExists, err.Error())
		case errors.Is(err, app.ErrEmailExists):
			response.Error(c, http.StatusBadRequest, response.CodeEmailExists, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "register failed")
		}
		return
	}

	response.OK(c, gin.H{
		"token": result.Token,
		"user": gin.H{
			"id":       result.User.ID,
			"username": result.User.Username,
			"email":    result.User.Email,
		},
	})
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "invalid request payload")
		return
	}

	result, err := h.authService.Login(app.LoginInput{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		switch {
		case errors.Is(err, app.ErrInvalidInput):
			response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		case errors.Is(err, app.ErrInvalidCredential):
			response.Error(c, http.StatusUnauthorized, response.CodeInvalidCredentials, err.Error())
		default:
			response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "login failed")
		}
		return
	}

	response.OK(c, gin.H{
		"token": result.Token,
		"user": gin.H{
			"id":       result.User.ID,
			"username": result.User.Username,
			"email":    result.User.Email,
		},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userIDAny, exists := c.Get(middleware.ContextUserIDKey)
	if !exists {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "user not found in token")
		return
	}

	userID, ok := userIDAny.(uint)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "invalid token payload")
		return
	}

	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalServer, "fetch current user failed")
		return
	}
	if user == nil {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "user not found")
		return
	}

	response.OK(c, gin.H{
		"id":       user.ID,
		"username": user.Username,
		"email":    user.Email,
	})
}
