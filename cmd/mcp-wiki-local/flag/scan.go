package flag

import (
	"github.com/spf13/cobra"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
)

// Scan adds flags for the scan subcommand.
func Scan(cmd *cobra.Command, values config.Values) {
	cmd.Flags().String(config.Keys.ArchivePath, values.ArchivePath, usage.ArchivePath)
	cmd.Flags().String(config.Keys.ArchiveIndexPath, values.ArchiveIndexPath, usage.ArchiveIndexPath)
	cmd.Flags().String(config.Keys.ManticoreDSN, values.ManticoreDSN, usage.ManticoreDSN)
	cmd.Flags().String(config.Keys.ManticoreTable, values.ManticoreTable, usage.ManticoreTable)
	cmd.Flags().Int(config.Keys.ManticoreBatchSize, values.ManticoreBatchSize, usage.ManticoreBatchSize)
	cmd.Flags().String(config.Keys.EmbedProvider, values.EmbedProvider, usage.EmbedProvider)
	cmd.Flags().String(config.Keys.EmbedURL, values.EmbedURL, usage.EmbedURL)
	cmd.Flags().String(config.Keys.EmbedAPIKey, values.EmbedAPIKey, usage.EmbedAPIKey)
	cmd.Flags().String(config.Keys.EmbedModel, values.EmbedModel, usage.EmbedModel)
	cmd.Flags().Int(config.Keys.EmbedDims, values.EmbedDims, usage.EmbedDims)
	cmd.Flags().Int(config.Keys.EmbedBatchSize, values.EmbedBatchSize, usage.EmbedBatchSize)
	cmd.Flags().Int(config.Keys.ScanWorkers, values.ScanWorkers, usage.ScanWorkers)
	cmd.Flags().Int(config.Keys.ScanBatchSize, values.ScanBatchSize, usage.ScanBatchSize)
	cmd.Flags().String(config.Keys.ScanCheckpointFile, values.ScanCheckpointFile, usage.ScanCheckpointFile)
	cmd.Flags().Bool(config.Keys.WikitextStripInfoboxes, values.WikitextStripInfoboxes, usage.WikitextStripInfoboxes)
	cmd.Flags().Bool(config.Keys.WikitextStripTables, values.WikitextStripTables, usage.WikitextStripTables)
}
