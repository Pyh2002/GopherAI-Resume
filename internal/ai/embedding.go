package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EmbeddingConfig holds API settings for text-embedding (OpenAI-compatible).
type EmbeddingConfig struct {
	BaseURL string
	APIKey  string
	Model   string
}

// Embed returns the embedding vector for the given text.
func (c *OpenAICompatibleClient) Embed(ctx context.Context, cfg EmbeddingConfig, text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("embedding input is empty")
	}

	reqBody := map[string]interface{}{
		"model": cfg.Model,
		"input": text,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request failed: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build embedding request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response failed: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding response status %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse embedding json failed: %w", err)
	}
	if len(parsed.Data) == 0 || len(parsed.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("empty embedding in response")
	}
	return parsed.Data[0].Embedding, nil
}

// EmbedBatch returns embeddings for multiple texts (if the API supports array input).
func (c *OpenAICompatibleClient) EmbedBatch(ctx context.Context, cfg EmbeddingConfig, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	trimmed := make([]string, 0, len(texts))
	for _, t := range texts {
		if s := strings.TrimSpace(t); s != "" {
			trimmed = append(trimmed, s)
		}
	}
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("no non-empty texts for embedding")
	}

	reqBody := map[string]interface{}{
		"model": cfg.Model,
		"input": trimmed,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal embedding batch request failed: %w", err)
	}

	url := strings.TrimRight(cfg.BaseURL, "/") + "/embeddings"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build embedding batch request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := c.httpClient
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding batch request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding batch response failed: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding batch response status %d: %s", resp.StatusCode, string(raw))
	}

	var parsed struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse embedding batch json failed: %w", err)
	}
	result := make([][]float32, len(parsed.Data))
	for i := range parsed.Data {
		result[i] = parsed.Data[i].Embedding
	}
	return result, nil
}
