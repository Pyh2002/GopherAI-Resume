package bootstrap

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"gopherai-resume/internal/config"
	"gopherai-resume/internal/model"
	mysqlClient "gopherai-resume/internal/platform/mysql"
	rabbitmqClient "gopherai-resume/internal/platform/rabbitmq"
	redisClient "gopherai-resume/internal/platform/redis"
)

type App struct {
	Config *config.Config
	MySQL  *gorm.DB
	Redis  *redis.Client
	MQConn *amqp.Connection

	StartedAt time.Time
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load config failed: %w", err)
	}

	mysqlDB, err := mysqlClient.New(ctx, cfg.MySQLDSN())
	if err != nil {
		return nil, err
	}
	if err := mysqlDB.AutoMigrate(&model.User{}, &model.Session{}, &model.Message{}); err != nil {
		return nil, fmt.Errorf("auto migrate tables failed: %w", err)
	}

	redisCli, err := redisClient.New(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		return nil, err
	}

	mqConn, err := rabbitmqClient.New(ctx, cfg.RabbitMQ.URL)
	if err != nil {
		return nil, err
	}

	return &App{
		Config:    cfg,
		MySQL:     mysqlDB,
		Redis:     redisCli,
		MQConn:    mqConn,
		StartedAt: time.Now(),
	}, nil
}

func (a *App) Close() error {
	var closeErr error
	if a.Redis != nil {
		if err := a.Redis.Close(); err != nil {
			closeErr = err
		}
	}
	if a.MQConn != nil {
		if err := a.MQConn.Close(); err != nil {
			closeErr = err
		}
	}
	if a.MySQL != nil {
		sqlDB, err := a.MySQL.DB()
		if err == nil {
			if err := sqlDB.Close(); err != nil {
				closeErr = err
			}
		}
	}
	return closeErr
}
