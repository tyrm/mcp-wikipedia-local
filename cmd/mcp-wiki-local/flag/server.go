package flag

import (
	"github.com/spf13/cobra"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
)

// Server adds flags for the server subcommand.
func Server(cmd *cobra.Command, values config.Values) {
	cmd.Flags().String(config.Keys.ServerHost, values.ServerHost, usage.ServerHost)
	cmd.Flags().Int(config.Keys.ServerPort, values.ServerPort, usage.ServerPort)
	cmd.Flags().String(config.Keys.AuthToken, values.AuthToken, usage.AuthToken)
	cmd.Flags().String(config.Keys.ArchivePath, values.ArchivePath, usage.ArchivePath)
	cmd.Flags().String(config.Keys.ArchiveIndexPath, values.ArchiveIndexPath, usage.ArchiveIndexPath)
	cmd.Flags().String(config.Keys.ManticoreDSN, values.ManticoreDSN, usage.ManticoreDSN)
	cmd.Flags().String(config.Keys.ManticoreTable, values.ManticoreTable, usage.ManticoreTable)
	cmd.Flags().String(config.Keys.EmbedProvider, values.EmbedProvider, usage.EmbedProvider)
	cmd.Flags().String(config.Keys.EmbedURL, values.EmbedURL, usage.EmbedURL)
	cmd.Flags().String(config.Keys.EmbedAPIKey, values.EmbedAPIKey, usage.EmbedAPIKey)
	cmd.Flags().String(config.Keys.EmbedModel, values.EmbedModel, usage.EmbedModel)
	cmd.Flags().Int(config.Keys.EmbedDims, values.EmbedDims, usage.EmbedDims)
	cmd.Flags().Int64(config.Keys.CacheMaxSizeMB, values.CacheMaxSizeMB, usage.CacheMaxSizeMB)
	cmd.Flags().Float64(config.Keys.SearchBM25Weight, values.SearchBM25Weight, usage.SearchBM25Weight)
	cmd.Flags().Float64(config.Keys.SearchKNNWeight, values.SearchKNNWeight, usage.SearchKNNWeight)
	cmd.Flags().Int(config.Keys.SearchRRFK, values.SearchRRFK, usage.SearchRRFK)
	cmd.Flags().Int(config.Keys.SearchDefaultLimit, values.SearchDefaultLimit, usage.SearchDefaultLimit)
	cmd.Flags().Int(config.Keys.ObsPrometheusPort, values.ObsPrometheusPort, usage.ObsPrometheusPort)
	cmd.Flags().String(config.Keys.ObsOTELEndpoint, values.ObsOTELEndpoint, usage.ObsOTELEndpoint)
	cmd.Flags().String(config.Keys.ObsServiceName, values.ObsServiceName, usage.ObsServiceName)
}
