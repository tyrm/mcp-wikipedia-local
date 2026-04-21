package flag

import (
	"github.com/spf13/cobra"
)

// Global adds flags that are common to all commands.
func Global(cmd *cobra.Command, values config.Values) {
	cmd.PersistentFlags().String(config.Keys.ConfigPath, values.ConfigPath, usage.ConfigPath)
	cmd.PersistentFlags().String(config.Keys.DataFolder, values.DataFolder, usage.DataFolder)
	cmd.PersistentFlags().String(config.Keys.LogLevel, values.LogLevel, usage.LogLevel)

	// application
	cmd.PersistentFlags().String(config.Keys.ApplicationName, values.ApplicationName, usage.ApplicationName)
	cmd.PersistentFlags().String(config.Keys.ApplicationWebsite, values.ApplicationWebsite, usage.ApplicationWebsite)
}
