package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	cfg  *Config
	http *http.Client
}

func New(cfg *Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Dims() int { return c.cfg.Dims }

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	batchSize := c.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = len(texts)
	}

	var result [][]float32
	for i := 0; i < len(texts); i += batchSize {
		batch, err := c.embedBatch(ctx, texts[i:min(i+batchSize, len(texts))])
		if err != nil {
			return nil, err
		}
		result = append(result, batch...)
	}
	return result, nil
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func (c *Client) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: c.cfg.Model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embed request status %d", resp.StatusCode)
	}

	var er embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, fmt.Errorf("decode embed response: %w", err)
	}

	// Response items may arrive out of order — sort by index.
	result := make([][]float32, len(er.Data))
	for _, item := range er.Data {
		if item.Index < len(result) {
			result[item.Index] = item.Embedding
		}
	}
	return result, nil
}
