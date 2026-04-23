package logic

import (
	"context"
	"fmt"
	"strings"
)

func (l *Logic) resolveTitle(ctx context.Context, title string) (string, error) {
	if _, ok := l.cfg.Archive.OffsetForTitle(title); ok {
		return title, nil
	}

	normalized := toTitleCase(title)
	if _, ok := l.cfg.Archive.OffsetForTitle(normalized); ok {
		return normalized, nil
	}

	results, err := l.cfg.Search.SearchBM25(ctx, title, 1)
	if err != nil || len(results) == 0 {
		return "", fmt.Errorf("title not found: %s", title)
	}
	return results[0].Title, nil
}

func toTitleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}
