package scan

import (
	"context"
	"os"

	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/tyrm/mcp-wikipedia-local/internal/archive"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
	"github.com/tyrm/mcp-wikipedia-local/internal/embed"
	"github.com/tyrm/mcp-wikipedia-local/internal/search/manticore"
	"github.com/tyrm/mcp-wikipedia-local/internal/worker"
)

func Scan(ctx context.Context, _ []string) error {
	zap.L().Info("opening archive",
		zap.String("path", viper.GetString(config.Keys.ArchivePath)),
		zap.String("index_path", viper.GetString(config.Keys.ArchiveIndexPath)),
	)
	arch, err := archive.New(&archive.Config{
		Path:      viper.GetString(config.Keys.ArchivePath),
		IndexPath: viper.GetString(config.Keys.ArchiveIndexPath),
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := arch.Close(); err != nil {
			zap.L().Error("close archive", zap.Error(err))
		}
	}()

	zap.L().Info("loading archive index into memory")
	if err := arch.LoadIndex(); err != nil {
		return err
	}
	zap.L().Info("archive index loaded", zap.Int("titles", len(arch.Titles())))

	zap.L().Info("connecting to embed service",
		zap.String("provider", viper.GetString(config.Keys.EmbedProvider)),
		zap.String("url", viper.GetString(config.Keys.EmbedURL)),
		zap.String("model", viper.GetString(config.Keys.EmbedModel)),
		zap.Int("dims", viper.GetInt(config.Keys.EmbedDims)),
	)
	embedClient, err := embed.NewClient(&embed.Config{
		Provider:  viper.GetString(config.Keys.EmbedProvider),
		URL:       viper.GetString(config.Keys.EmbedURL),
		APIKey:    viper.GetString(config.Keys.EmbedAPIKey),
		Model:     viper.GetString(config.Keys.EmbedModel),
		Dims:      viper.GetInt(config.Keys.EmbedDims),
		BatchSize: viper.GetInt(config.Keys.EmbedBatchSize),
	})
	if err != nil {
		return err
	}

	zap.L().Info("connecting to manticore",
		zap.String("dsn", viper.GetString(config.Keys.ManticoreDSN)),
		zap.String("table", viper.GetString(config.Keys.ManticoreTable)),
	)
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
	zap.L().Info("manticore table ready", zap.String("table", viper.GetString(config.Keys.ManticoreTable)))

	// Fresh scan (no checkpoint): truncate any leftover data so IDs restart cleanly.
	checkpointFile := viper.GetString(config.Keys.ScanCheckpointFile)
	_, statErr := os.Stat(checkpointFile)
	isFresh := checkpointFile == "" || os.IsNotExist(statErr)
	if isFresh {
		if err := searchClient.TruncateTable(ctx); err != nil {
			return err
		}
		zap.L().Info("table truncated for fresh scan")
	} else {
		zap.L().Info("resuming from checkpoint", zap.String("checkpoint", checkpointFile))
	}

	zap.L().Info("starting scan",
		zap.Int("workers", viper.GetInt(config.Keys.ScanWorkers)),
		zap.Int("batch_size", viper.GetInt(config.Keys.ScanBatchSize)),
	)
	count, err := worker.Run(ctx, arch, embedClient, searchClient, &worker.Config{
		NumWorkers:     viper.GetInt(config.Keys.ScanWorkers),
		BatchSize:      viper.GetInt(config.Keys.ScanBatchSize),
		CheckpointFile: checkpointFile,
	})
	zap.L().Info("scan complete", zap.Int64("indexed", count))
	return err
}
