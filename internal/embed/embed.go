package embed

import "context"

type Client interface {
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	Dims() int
}
