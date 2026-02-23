package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	redisv9 "github.com/redis/go-redis/v9"

	"gopherai-resume/internal/model"
)

type HistoryCache struct {
	client         *redisv9.Client
	historyTTL     time.Duration
	dirtyMarkerTTL time.Duration
}

func NewHistoryCache(client *redisv9.Client, historyTTL, dirtyMarkerTTL time.Duration) *HistoryCache {
	if historyTTL <= 0 {
		historyTTL = 60 * time.Second
	}
	if dirtyMarkerTTL <= 0 {
		dirtyMarkerTTL = 5 * time.Second
	}
	return &HistoryCache{
		client:         client,
		historyTTL:     historyTTL,
		dirtyMarkerTTL: dirtyMarkerTTL,
	}
}

func (c *HistoryCache) GetHistory(ctx context.Context, sessionID uint) ([]model.Message, bool, error) {
	key := c.historyKey(sessionID)
	raw, err := c.client.Get(ctx, key).Result()
	if err == redisv9.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("redis get history failed: %w", err)
	}

	var messages []model.Message
	if err := json.Unmarshal([]byte(raw), &messages); err != nil {
		return nil, false, fmt.Errorf("unmarshal cached history failed: %w", err)
	}
	return messages, true, nil
}

func (c *HistoryCache) SetHistory(ctx context.Context, sessionID uint, messages []model.Message) error {
	key := c.historyKey(sessionID)
	payload, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("marshal history cache failed: %w", err)
	}
	if err := c.client.Set(ctx, key, payload, c.historyTTL).Err(); err != nil {
		return fmt.Errorf("redis set history failed: %w", err)
	}
	return nil
}

func (c *HistoryCache) DeleteHistory(ctx context.Context, sessionID uint) error {
	key := c.historyKey(sessionID)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete history failed: %w", err)
	}
	return nil
}

func (c *HistoryCache) MarkDirty(ctx context.Context, sessionID uint) error {
	key := c.dirtyKey(sessionID)
	if err := c.client.Set(ctx, key, "1", c.dirtyMarkerTTL).Err(); err != nil {
		return fmt.Errorf("redis set dirty marker failed: %w", err)
	}
	return nil
}

func (c *HistoryCache) IsDirty(ctx context.Context, sessionID uint) (bool, error) {
	key := c.dirtyKey(sessionID)
	exists, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis check dirty marker failed: %w", err)
	}
	return exists > 0, nil
}

func (c *HistoryCache) historyKey(sessionID uint) string {
	return fmt.Sprintf("chat:history:%d", sessionID)
}

func (c *HistoryCache) dirtyKey(sessionID uint) string {
	return fmt.Sprintf("chat:history:dirty:%d", sessionID)
}
