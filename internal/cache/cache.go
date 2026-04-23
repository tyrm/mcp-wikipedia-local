package cache

import (
	"fmt"

	"github.com/outcaste-io/ristretto"

	"github.com/tyrm/mcp-wikipedia-local/internal/parser"
)

type Cache struct {
	store *ristretto.Cache
}

func New(cfg *Config) (*Cache, error) {
	store, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,
		MaxCost:     cfg.MaxSizeMB * 1024 * 1024,
		BufferItems: 64,
	})
	if err != nil {
		return nil, fmt.Errorf("new ristretto cache: %w", err)
	}
	return &Cache{store: store}, nil
}

func (c *Cache) Get(title string) (*parser.ParsedPage, bool) {
	val, ok := c.store.Get(title)
	if !ok {
		return nil, false
	}
	return val.(*parser.ParsedPage), true
}

func (c *Cache) Set(title string, page *parser.ParsedPage) {
	cost := estimateCost(page)
	c.store.Set(title, page, cost)
}

func estimateCost(page *parser.ParsedPage) int64 {
	size := int64(len(page.Title) + len(page.Lead) + len(page.IsRedirect))
	for _, s := range page.Sections {
		size += int64(len(s.Title) + len(s.Body))
	}
	for _, o := range page.Outline {
		size += int64(len(o.Title))
	}
	return size
}
