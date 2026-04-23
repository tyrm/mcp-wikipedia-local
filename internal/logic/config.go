package logic

import (
	"github.com/tyrm/mcp-wikipedia-local/internal/archive"
	"github.com/tyrm/mcp-wikipedia-local/internal/cache"
	"github.com/tyrm/mcp-wikipedia-local/internal/embed"
	"github.com/tyrm/mcp-wikipedia-local/internal/search"
)

type Config struct {
	Archive      *archive.Archive
	Search       search.Client
	Embed        embed.Client
	Cache        *cache.Cache
	DefaultLimit int
}
