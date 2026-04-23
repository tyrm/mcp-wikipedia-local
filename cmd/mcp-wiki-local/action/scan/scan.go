package scan

import (
	"context"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tyrm/mcp-wikipedia-local/internal/archive"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
	"github.com/tyrm/mcp-wikipedia-local/internal/embed/ollama"
	"github.com/tyrm/mcp-wikipedia-local/internal/search/manticore"
	"github.com/tyrm/mcp-wikipedia-local/internal/worker"
)

func Scan(ctx context.Context, _ []string) error {
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
	zap.L().Info("archive index loaded", zap.Int("titles", len(arch.Titles())))

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

	if err := searchClient.CreateTable(ctx); err != nil {
		return err
	}

	count, err := worker.Run(ctx, arch, embedClient, searchClient, &worker.Config{
		NumWorkers:     viper.GetInt(config.Keys.ScanWorkers),
		BatchSize:      viper.GetInt(config.Keys.ScanBatchSize),
		CheckpointFile: viper.GetString(config.Keys.ScanCheckpointFile),
	})
	zap.L().Info("scan complete", zap.Int64("indexed", count))
	return err
}
