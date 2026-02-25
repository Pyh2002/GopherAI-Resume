package http

import (
	"time"

	"github.com/gin-gonic/gin"

	"gopherai-resume/internal/ai"
	appsvc "gopherai-resume/internal/app"
	"gopherai-resume/internal/bootstrap"
	"gopherai-resume/internal/cache"
	rabbitmqPlatform "gopherai-resume/internal/platform/rabbitmq"
	"gopherai-resume/internal/repository"
	"gopherai-resume/internal/transport/http/handler"
	"gopherai-resume/internal/transport/http/middleware"
	"gopherai-resume/internal/vision"
)

func NewRouter(app *bootstrap.App) *gin.Engine {
	gin.SetMode(app.Config.App.GinMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	healthHandler := handler.NewHealthHandler(app)
	router.StaticFile("/", "web/index.html")
	router.StaticFile("/login", "web/login.html")
	router.StaticFile("/register", "web/register.html")
	router.StaticFile("/app", "web/app.html")
	router.StaticFile("/chat", "web/chat.html")
	router.StaticFile("/rag", "web/rag.html")
	router.StaticFile("/vision", "web/vision.html")
	router.GET("/healthz", healthHandler.Check)

	userRepo := repository.NewUserRepository(app.MySQL)
	sessionRepo := repository.NewSessionRepository(app.MySQL)
	messageRepo := repository.NewMessageRepository(app.MySQL)
	authService := appsvc.NewAuthService(
		userRepo,
		app.Config.Auth.JWTSecret,
		time.Duration(app.Config.Auth.JWTExpireMinute)*time.Minute,
	)
	messagePublisher := rabbitmqPlatform.NewMessagePublisher(
		app.MQConn,
		app.Config.RabbitMQ.MessagePersistQueue,
	)
	historyCache := cache.NewHistoryCache(
		app.Redis,
		time.Duration(app.Config.Redis.HistoryTTLSeconds)*time.Second,
		time.Duration(app.Config.Redis.HistoryDirtyTTLSeconds)*time.Second,
	)
	chatService := appsvc.NewChatService(
		sessionRepo,
		messageRepo,
		messagePublisher,
		historyCache,
		ai.ChatConfig{
			BaseURL: app.Config.LLM.BaseURL,
			APIKey:  app.Config.LLM.APIKey,
			Model:   app.Config.LLM.Model,
		},
		app.Config.LLM.MaxContextMessage,
	)
	authHandler := handler.NewAuthHandler(authService)
	chatHandler := handler.NewChatHandler(chatService)

	embConfig := ai.EmbeddingConfig{
		BaseURL: app.Config.LLM.BaseURL,
		APIKey:  app.Config.LLM.APIKey,
		Model:   app.Config.LLM.EmbeddingModel,
	}
	chatConfig := ai.ChatConfig{
		BaseURL: app.Config.LLM.BaseURL,
		APIKey:  app.Config.LLM.APIKey,
		Model:   app.Config.LLM.Model,
	}
	ragSessionRepo := repository.NewRAGSessionRepository(app.MySQL)
	ragDocRepo := repository.NewRAGDocumentRepository(app.MySQL)
	ragChunkRepo := repository.NewRAGChunkRepository(app.MySQL)
	ragService := appsvc.NewRAGService(
		ragSessionRepo,
		ragDocRepo,
		ragChunkRepo,
		ai.NewOpenAICompatibleClient(),
		embConfig,
		chatConfig,
	)
	ragHandler := handler.NewRAGHandler(ragService)

	visionClassifier := vision.NewClassifier(
		app.Config.Vision.ModelPath,
		app.Config.Vision.LabelsPath,
		app.Config.Vision.ONNXSharedLibPath,
		app.Config.Vision.TopK,
	)
	visionHandler := handler.NewVisionHandler(visionClassifier)

	v1 := router.Group("/api/v1")
	authGroup := v1.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login", authHandler.Login)
	authGroup.GET("/me", middleware.AuthJWT(app.Config.Auth.JWTSecret), authHandler.Me)

	chatGroup := v1.Group("/chat")
	chatGroup.Use(middleware.AuthJWT(app.Config.Auth.JWTSecret))
	chatGroup.POST("/sessions", chatHandler.CreateSession)
	chatGroup.GET("/sessions", chatHandler.ListSessions)
	chatGroup.DELETE("/sessions/:id", chatHandler.DeleteSession)
	chatGroup.POST("/messages", chatHandler.SendMessage)
	chatGroup.POST("/stream", chatHandler.StreamMessage)
	chatGroup.GET("/history", chatHandler.GetHistory)

	ragGroup := v1.Group("/rag")
	ragGroup.Use(middleware.AuthJWT(app.Config.Auth.JWTSecret))
	ragGroup.POST("/sessions", ragHandler.CreateSession)
	ragGroup.GET("/sessions", ragHandler.ListSessions)
	ragGroup.DELETE("/sessions/:id", ragHandler.DeleteSession)
	ragGroup.POST("/documents", ragHandler.CreateDocument)
	ragGroup.POST("/documents/upload", ragHandler.UploadPDF)
	ragGroup.GET("/documents", ragHandler.ListDocuments)
	ragGroup.DELETE("/documents/:id", ragHandler.DeleteDocument)
	ragGroup.POST("/ask", ragHandler.Ask)

	visionGroup := v1.Group("/vision")
	visionGroup.Use(middleware.AuthJWT(app.Config.Auth.JWTSecret))
	visionGroup.POST("/classify", visionHandler.Classify)

	return router
}
