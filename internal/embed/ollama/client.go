package ollama

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
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Dims() int {
	return c.cfg.Dims
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	batchSize := c.cfg.BatchSize
	if batchSize <= 0 {
		batchSize = len(texts)
	}

	var result [][]float32
	for i := 0; i < len(texts); i += batchSize {
		end := min(i+batchSize, len(texts))
		batch, err := c.embedBatch(ctx, texts[i:end])
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
	Embeddings [][]float32 `json:"embeddings"`
}

func (c *Client) embedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	body, err := json.Marshal(embedRequest{Model: c.cfg.Model, Input: texts})
	if err != nil {
		return nil, fmt.Errorf("marshal embed request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.cfg.URL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embed request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

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

	return er.Embeddings, nil
}
