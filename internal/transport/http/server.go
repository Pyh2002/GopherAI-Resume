package http

import (
	"time"

	"github.com/gin-gonic/gin"

	appsvc "gopherai-resume/internal/app"
	"gopherai-resume/internal/bootstrap"
	"gopherai-resume/internal/repository"
	"gopherai-resume/internal/transport/http/handler"
	"gopherai-resume/internal/transport/http/middleware"
)

func NewRouter(app *bootstrap.App) *gin.Engine {
	gin.SetMode(app.Config.App.GinMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	healthHandler := handler.NewHealthHandler(app)
	router.StaticFile("/", "web/index.html")
	router.StaticFile("/login", "web/login.html")
	router.StaticFile("/register", "web/register.html")
	router.StaticFile("/chat", "web/chat.html")
	router.GET("/healthz", healthHandler.Check)

	userRepo := repository.NewUserRepository(app.MySQL)
	sessionRepo := repository.NewSessionRepository(app.MySQL)
	messageRepo := repository.NewMessageRepository(app.MySQL)
	authService := appsvc.NewAuthService(
		userRepo,
		app.Config.Auth.JWTSecret,
		time.Duration(app.Config.Auth.JWTExpireMinute)*time.Minute,
	)
	chatService := appsvc.NewChatService(sessionRepo, messageRepo)
	authHandler := handler.NewAuthHandler(authService)
	chatHandler := handler.NewChatHandler(chatService)

	v1 := router.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.GET("/me", middleware.AuthJWT(app.Config.Auth.JWTSecret), authHandler.Me)

	chatGroup := v1.Group("/chat")
	chatGroup.Use(middleware.AuthJWT(app.Config.Auth.JWTSecret))
	chatGroup.POST("/sessions", chatHandler.CreateSession)
	chatGroup.GET("/sessions", chatHandler.ListSessions)
	chatGroup.POST("/messages", chatHandler.SendMessage)
	chatGroup.GET("/history", chatHandler.GetHistory)

	return router
}
