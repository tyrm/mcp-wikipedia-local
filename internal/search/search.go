package search

import "context"

type Client interface {
	CreateTable(ctx context.Context) error
	BulkInsert(ctx context.Context, articles []Article) error
	SearchBM25(ctx context.Context, query string, limit int) ([]Result, error)
	SearchKNN(ctx context.Context, embedding []float32, limit int) ([]Result, error)
	SearchHybrid(ctx context.Context, query string, embedding []float32, limit int) ([]Result, error)
	TruncateTable(ctx context.Context) error
}

type Article struct {
	ID         int64
	Title      string
	LeadText   string
	Lang       string
	IsDisambig bool
	Embedding  []float32
}

type Result struct {
	ID          int64
	Title       string
	LeadSnippet string
	Score       float64
}
