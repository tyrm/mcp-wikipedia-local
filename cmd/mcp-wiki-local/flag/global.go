package flag

import (
	"github.com/spf13/cobra"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
)

// Global adds flags that are common to all commands.
func Global(cmd *cobra.Command, values config.Values) {
	cmd.PersistentFlags().String(config.Keys.ConfigPath, values.ConfigPath, usage.ConfigPath)
}
