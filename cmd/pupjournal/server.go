package main

import (
	"github.com/spf13/cobra"
	"github.com/tyrm/mcp-wikipedia-local/cmd/pupjournal/action/server"
	"github.com/tyrm/mcp-wikipedia-local/cmd/pupjournal/flag"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
)

// serverCommands returns the 'server' subcommand.
func serverCommands() *cobra.Command {
	serverCmd := &cobra.Command{
		Use:   "server",
		Short: "start server",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), server.Start, args)
		},
	}
	flag.Server(serverCmd, config.Defaults)

	return serverCmd
}
