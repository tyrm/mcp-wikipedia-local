package logic

import (
	"context"

	"github.com/tyrm/mcp-wikipedia-local/internal/search"
)

func (l *Logic) Search(ctx context.Context, query string, limit int) ([]search.Result, error) {
	if limit <= 0 {
		limit = l.cfg.DefaultLimit
	}

	embedding, err := l.cfg.Embed.Embed(ctx, []string{query})
	if err == nil && len(embedding) > 0 {
		results, err := l.cfg.Search.SearchHybrid(ctx, query, embedding[0], limit)
		if err == nil {
			return results, nil
		}
	}

	return l.cfg.Search.SearchBM25(ctx, query, limit)
}
