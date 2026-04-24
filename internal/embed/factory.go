package embed

import (
	"fmt"

	"github.com/tyrm/mcp-wikipedia-local/internal/embed/ollama"
	"github.com/tyrm/mcp-wikipedia-local/internal/embed/openai"
)

type Config struct {
	Provider  string
	URL       string
	APIKey    string
	Model     string
	Dims      int
	BatchSize int
}

func NewClient(cfg *Config) (Client, error) {
	switch cfg.Provider {
	case "ollama", "":
		return ollama.New(&ollama.Config{
			URL:       cfg.URL,
			Model:     cfg.Model,
			Dims:      cfg.Dims,
			BatchSize: cfg.BatchSize,
		}), nil
	case "openai", "lmstudio":
		return openai.New(&openai.Config{
			URL:       cfg.URL,
			APIKey:    cfg.APIKey,
			Model:     cfg.Model,
			Dims:      cfg.Dims,
			BatchSize: cfg.BatchSize,
		}), nil
	default:
		return nil, fmt.Errorf("unknown embed provider %q (supported: ollama, openai, lmstudio)", cfg.Provider)
	}
}
