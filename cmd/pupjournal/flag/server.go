package flag

import (
	"github.com/spf13/cobra"
)

func Server(cmd *cobra.Command, values config.Values) {
	Database(cmd, values)
	Valkey(cmd, values)

	cmd.PersistentFlags().String(config.Keys.CookieSecret, values.CookieSecret, usage.CookieSecret)
	cmd.PersistentFlags().String(config.Keys.HTTPBind, values.HTTPBind, usage.HTTPBind)
	cmd.PersistentFlags().String(config.Keys.UptraceDSN, values.UptraceDSN, usage.UptraceDSN)
}
