package server

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tyrm/mcp-wikipedia-local/cmd/mcp-wiki-local/action"
	"github.com/tyrm/mcp-wikipedia-local/internal/archive"
	"github.com/tyrm/mcp-wikipedia-local/internal/cache"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
	"github.com/tyrm/mcp-wikipedia-local/internal/embed/ollama"
	"github.com/tyrm/mcp-wikipedia-local/internal/logic"
	mcpserver "github.com/tyrm/mcp-wikipedia-local/internal/mcp"
	"github.com/tyrm/mcp-wikipedia-local/internal/obs"
	"github.com/tyrm/mcp-wikipedia-local/internal/search/manticore"
)

var Start action.Action = func(ctx context.Context, _ []string) error {
	arch, err := archive.New(&archive.Config{
		Path:      viper.GetString(config.Keys.ArchivePath),
		IndexPath: viper.GetString(config.Keys.ArchiveIndexPath),
	})
	if err != nil {
		return err
	}
	defer arch.Close()
	if err := arch.LoadIndex(); err != nil {
		return err
	}
	zap.L().Info("archive loaded", zap.Int("titles", len(arch.Titles())))

	embedClient := ollama.New(&ollama.Config{
		URL:       viper.GetString(config.Keys.EmbedURL),
		Model:     viper.GetString(config.Keys.EmbedModel),
		Dims:      viper.GetInt(config.Keys.EmbedDims),
		BatchSize: viper.GetInt(config.Keys.EmbedBatchSize),
	})

	searchClient, err := manticore.New(&manticore.Config{
		DSN:       viper.GetString(config.Keys.ManticoreDSN),
		Table:     viper.GetString(config.Keys.ManticoreTable),
		BatchSize: viper.GetInt(config.Keys.ManticoreBatchSize),
		EmbedDims: viper.GetInt(config.Keys.EmbedDims),
	})
	if err != nil {
		return err
	}

	pageCache, err := cache.New(&cache.Config{
		MaxSizeMB: viper.GetInt64(config.Keys.CacheMaxSizeMB),
	})
	if err != nil {
		return err
	}

	obsClient, err := obs.New(&obs.Config{
		PrometheusPort: viper.GetInt(config.Keys.ObsPrometheusPort),
		OTELEndpoint:   viper.GetString(config.Keys.ObsOTELEndpoint),
		ServiceName:    viper.GetString(config.Keys.ObsServiceName),
	})
	if err != nil {
		return err
	}
	defer obsClient.Shutdown(context.Background())
	go obsClient.ServeMetrics()

	logicClient := logic.New(&logic.Config{
		Archive:      arch,
		Search:       searchClient,
		Embed:        embedClient,
		Cache:        pageCache,
		DefaultLimit: viper.GetInt(config.Keys.SearchDefaultLimit),
	})

	srv, err := mcpserver.New(logicClient, &mcpserver.Config{
		Host: viper.GetString(config.Keys.ServerHost),
		Port: viper.GetInt(config.Keys.ServerPort),
	})
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	errCh := make(chan error, 1)
	go func() {
		zap.L().Info("MCP server starting",
			zap.String("host", viper.GetString(config.Keys.ServerHost)),
			zap.Int("port", viper.GetInt(config.Keys.ServerPort)),
		)
		errCh <- srv.Start()
	}()

	select {
	case sig := <-sigCh:
		zap.L().Info("received signal", zap.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-ctx.Done():
	}

	zap.L().Info("shutting down")
	return nil
}
