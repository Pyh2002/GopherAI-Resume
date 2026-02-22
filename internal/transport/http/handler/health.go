package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"gopherai-resume/internal/bootstrap"
)

type HealthHandler struct {
	app *bootstrap.App
}

type dependencyStatus struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

func NewHealthHandler(app *bootstrap.App) *HealthHandler {
	return &HealthHandler{app: app}
}

func (h *HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	mysqlStatus := h.checkMySQL(ctx)
	redisStatus := h.checkRedis(ctx)
	rmqStatus := h.checkRabbitMQ()

	allOK := mysqlStatus.OK && redisStatus.OK && rmqStatus.OK
	statusCode := http.StatusOK
	if !allOK {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"app":        h.app.Config.App.Name,
		"env":        h.app.Config.App.Env,
		"uptime_sec": int(time.Since(h.app.StartedAt).Seconds()),
		"dependencies": gin.H{
			"mysql":    mysqlStatus,
			"redis":    redisStatus,
			"rabbitmq": rmqStatus,
		},
	})
}

func (h *HealthHandler) checkMySQL(ctx context.Context) dependencyStatus {
	sqlDB, err := h.app.MySQL.DB()
	if err != nil {
		return dependencyStatus{OK: false, Message: err.Error()}
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return dependencyStatus{OK: false, Message: err.Error()}
	}
	return dependencyStatus{OK: true}
}

func (h *HealthHandler) checkRedis(ctx context.Context) dependencyStatus {
	if err := h.app.Redis.Ping(ctx).Err(); err != nil {
		return dependencyStatus{OK: false, Message: err.Error()}
	}
	return dependencyStatus{OK: true}
}

func (h *HealthHandler) checkRabbitMQ() dependencyStatus {
	if h.app.MQConn == nil || h.app.MQConn.IsClosed() {
		return dependencyStatus{OK: false, Message: "connection closed"}
	}
	return dependencyStatus{OK: true}
}
