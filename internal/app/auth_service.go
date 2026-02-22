package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"gopherai-resume/internal/model"
	"gopherai-resume/internal/pkg/jwtutil"
	"gopherai-resume/internal/repository"
)

var (
	ErrInvalidInput      = errors.New("invalid input")
	ErrUsernameExists    = errors.New("username already exists")
	ErrEmailExists       = errors.New("email already exists")
	ErrInvalidCredential = errors.New("invalid username or password")
)

type AuthService struct {
	userRepo      *repository.UserRepository
	jwtSecret     string
	jwtExpiration time.Duration
}

type RegisterInput struct {
	Username string
	Email    string
	Password string
}

type LoginInput struct {
	Username string
	Password string
}

type AuthResult struct {
	Token string
	User  *model.User
}

func NewAuthService(userRepo *repository.UserRepository, jwtSecret string, jwtExpiration time.Duration) *AuthService {
	return &AuthService{
		userRepo:      userRepo,
		jwtSecret:     jwtSecret,
		jwtExpiration: jwtExpiration,
	}
}

func (s *AuthService) Register(input RegisterInput) (*AuthResult, error) {
	username := strings.TrimSpace(input.Username)
	email := strings.TrimSpace(strings.ToLower(input.Email))
	password := strings.TrimSpace(input.Password)

	if username == "" || email == "" || password == "" || len(password) < 8 {
		return nil, ErrInvalidInput
	}

	existingByName, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return nil, err
	}
	if existingByName != nil {
		return nil, ErrUsernameExists
	}

	existingByEmail, err := s.userRepo.GetByEmail(email)
	if err != nil {
		return nil, err
	}
	if existingByEmail != nil {
		return nil, ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password failed: %w", err)
	}

	user := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	token, err := jwtutil.GenerateToken(s.jwtSecret, s.jwtExpiration, user.ID, user.Username)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: user}, nil
}

func (s *AuthService) Login(input LoginInput) (*AuthResult, error) {
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" {
		return nil, ErrInvalidInput
	}

	user, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidCredential
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredential
	}

	token, err := jwtutil.GenerateToken(s.jwtSecret, s.jwtExpiration, user.ID, user.Username)
	if err != nil {
		return nil, err
	}
	return &AuthResult{Token: token, User: user}, nil
}

func (s *AuthService) GetUserByID(id uint) (*model.User, error) {
	if id == 0 {
		return nil, ErrInvalidInput
	}
	return s.userRepo.GetByID(id)
}
