package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/BurntSushi/toml"
)

type Config struct {
	App      AppConfig      `toml:"app"`
	Auth     AuthConfig     `toml:"auth"`
	LLM      LLMConfig      `toml:"llm"`
	MySQL    MySQLConfig    `toml:"mysql"`
	Redis    RedisConfig    `toml:"redis"`
	RabbitMQ RabbitMQConfig `toml:"rabbitmq"`
	Vision   VisionConfig   `toml:"vision"`
}

type AppConfig struct {
	Name    string `toml:"name"`
	Env     string `toml:"env"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
	GinMode string `toml:"gin_mode"`
}

type MySQLConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	User     string `toml:"user"`
	Password string `toml:"password"`
	DB       string `toml:"db"`
	Params   string `toml:"params"`
}

type RedisConfig struct {
	Addr                   string `toml:"addr"`
	Password               string `toml:"password"`
	DB                     int    `toml:"db"`
	HistoryTTLSeconds      int    `toml:"history_ttl_seconds"`
	HistoryDirtyTTLSeconds int    `toml:"history_dirty_ttl_seconds"`
}

type RabbitMQConfig struct {
	URL                 string `toml:"url"`
	MessagePersistQueue string `toml:"message_persist_queue"`
}

type AuthConfig struct {
	JWTSecret       string `toml:"jwt_secret"`
	JWTExpireMinute int    `toml:"jwt_expire_minute"`
}

type LLMConfig struct {
	BaseURL           string `toml:"base_url"`
	APIKey            string `toml:"api_key"`
	Model             string `toml:"model"`
	MaxContextMessage int    `toml:"max_context_message"`
	EmbeddingModel    string `toml:"embedding_model"`
}

type VisionConfig struct {
	ModelPath         string `toml:"model_path"`
	LabelsPath        string `toml:"labels_path"`
	TopK              int    `toml:"top_k"`
	ONNXSharedLibPath string `toml:"onnx_shared_lib_path"`
}

func Load() (*Config, error) {
	cfg := defaultConfig()

	configPath := getEnv("CONFIG_FILE", "configs/config.toml")
	if _, err := os.Stat(configPath); err == nil {
		if _, err := toml.DecodeFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("decode config file failed: %w", err)
		}
	}

	overrideByEnv(cfg)
	return cfg, nil
}

func (c *Config) HTTPAddr() string {
	return fmt.Sprintf("%s:%d", c.App.Host, c.App.Port)
}

func (c *Config) MySQLDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?%s",
		c.MySQL.User,
		c.MySQL.Password,
		c.MySQL.Host,
		c.MySQL.Port,
		c.MySQL.DB,
		c.MySQL.Params,
	)
}

func defaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:    "gopherai-resume",
			Env:     "dev",
			Host:    "0.0.0.0",
			Port:    8080,
			GinMode: "debug",
		},
		Auth: AuthConfig{
			JWTSecret:       "change-me-in-production",
			JWTExpireMinute: 120,
		},
		LLM: LLMConfig{
			BaseURL:           "https://dashscope.aliyuncs.com/compatible-mode/v1",
			APIKey:            "sk-f35af11a2d4a4e819e1137bff10e36d3",
			Model:             "qwen3-max",
			MaxContextMessage: 20,
			EmbeddingModel:    "text-embedding-v3",
		},
		MySQL: MySQLConfig{
			Host:     "127.0.0.1",
			Port:     3306,
			User:     "root",
			Password: "",
			DB:       "gopherai_resume",
			Params:   "parseTime=true&loc=Local&charset=utf8mb4",
		},
		Redis: RedisConfig{
			Addr:                   "127.0.0.1:6379",
			Password:               "",
			DB:                     0,
			HistoryTTLSeconds:      60,
			HistoryDirtyTTLSeconds: 5,
		},
		RabbitMQ: RabbitMQConfig{
			URL:                 "amqp://guest:guest@127.0.0.1:5672/",
			MessagePersistQueue: "chat.message.persist",
		},
		Vision: VisionConfig{
			ModelPath:         "assets/mobilenetv2-7.onnx",
			LabelsPath:        "assets/labels.txt",
			TopK:              5,
			ONNXSharedLibPath: "", // use default or set via VISION_ONNX_LIB
		},
	}
}

func overrideByEnv(cfg *Config) {
	cfg.App.Name = getEnv("APP_NAME", cfg.App.Name)
	cfg.App.Env = getEnv("APP_ENV", cfg.App.Env)
	cfg.App.Host = getEnv("APP_HOST", cfg.App.Host)
	cfg.App.Port = getEnvAsInt("APP_PORT", cfg.App.Port)
	cfg.App.GinMode = getEnv("GIN_MODE", cfg.App.GinMode)
	cfg.Auth.JWTSecret = getEnv("JWT_SECRET", cfg.Auth.JWTSecret)
	cfg.Auth.JWTExpireMinute = getEnvAsInt("JWT_EXPIRE_MINUTE", cfg.Auth.JWTExpireMinute)
	cfg.LLM.BaseURL = getEnv("LLM_BASE_URL", cfg.LLM.BaseURL)
	cfg.LLM.APIKey = getEnv("LLM_API_KEY", cfg.LLM.APIKey)
	cfg.LLM.Model = getEnv("LLM_MODEL", cfg.LLM.Model)
	cfg.LLM.MaxContextMessage = getEnvAsInt("LLM_MAX_CONTEXT_MESSAGE", cfg.LLM.MaxContextMessage)
	cfg.LLM.EmbeddingModel = getEnv("LLM_EMBEDDING_MODEL", cfg.LLM.EmbeddingModel)

	cfg.MySQL.Host = getEnv("MYSQL_HOST", cfg.MySQL.Host)
	cfg.MySQL.Port = getEnvAsInt("MYSQL_PORT", cfg.MySQL.Port)
	cfg.MySQL.User = getEnv("MYSQL_USER", cfg.MySQL.User)
	cfg.MySQL.Password = getEnv("MYSQL_PASSWORD", cfg.MySQL.Password)
	cfg.MySQL.DB = getEnv("MYSQL_DB", cfg.MySQL.DB)
	cfg.MySQL.Params = getEnv("MYSQL_PARAMS", cfg.MySQL.Params)

	cfg.Redis.Addr = getEnv("REDIS_ADDR", cfg.Redis.Addr)
	cfg.Redis.Password = getEnv("REDIS_PASSWORD", cfg.Redis.Password)
	cfg.Redis.DB = getEnvAsInt("REDIS_DB", cfg.Redis.DB)
	cfg.Redis.HistoryTTLSeconds = getEnvAsInt("REDIS_HISTORY_TTL_SECONDS", cfg.Redis.HistoryTTLSeconds)
	cfg.Redis.HistoryDirtyTTLSeconds = getEnvAsInt("REDIS_HISTORY_DIRTY_TTL_SECONDS", cfg.Redis.HistoryDirtyTTLSeconds)

	cfg.RabbitMQ.URL = getEnv("RABBITMQ_URL", cfg.RabbitMQ.URL)
	cfg.RabbitMQ.MessagePersistQueue = getEnv("RABBITMQ_MESSAGE_PERSIST_QUEUE", cfg.RabbitMQ.MessagePersistQueue)

	cfg.Vision.ModelPath = getEnv("VISION_MODEL_PATH", cfg.Vision.ModelPath)
	cfg.Vision.LabelsPath = getEnv("VISION_LABELS_PATH", cfg.Vision.LabelsPath)
	cfg.Vision.TopK = getEnvAsInt("VISION_TOP_K", cfg.Vision.TopK)
	cfg.Vision.ONNXSharedLibPath = getEnv("VISION_ONNX_LIB", cfg.Vision.ONNXSharedLibPath)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}
