package main

import (
	"context"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/tyrm/mcp-wikipedia-local/cmd/mcp-wiki-local/action"
	"github.com/tyrm/mcp-wikipedia-local/cmd/mcp-wiki-local/action/scan"
	"github.com/tyrm/mcp-wikipedia-local/cmd/mcp-wiki-local/action/server"
	"github.com/tyrm/mcp-wikipedia-local/cmd/mcp-wiki-local/flag"
	"github.com/tyrm/mcp-wikipedia-local/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Version is the software version.
var Version string

// Commit is the git commit.
var Commit string

// GitShortHashLength is the standard length of a Git short commit hash.
const GitShortHashLength = 7

func main() {
	// init logger
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	loggerConfig.DisableStacktrace = true
	zapLogger, err := loggerConfig.Build()
	if err != nil {
		log.Fatal(err)

		return
	}
	defer func() {
		_ = zapLogger.Sync()
	}()
	zap.ReplaceGlobals(zapLogger)

	// set software version
	ver := "dev"
	if len(Commit) < GitShortHashLength {
		ver = "v" + Version
	} else {
		ver = "v" + Version + "-" + Commit[:GitShortHashLength]
	}

	viper.Set(config.Keys.SoftwareVersion, ver)

	rootCmd := &cobra.Command{
		Use:           "mcp-wikipedia-local [--config-path File]",
		Short:         "", // TODO
		Version:       ver,
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	flag.Global(rootCmd, config.Defaults)

	err = viper.BindPFlag(config.Keys.ConfigPath, rootCmd.PersistentFlags().Lookup(config.Keys.ConfigPath))
	if err != nil {
		zap.L().Fatal("Error binding config flag", zap.Error(err))

		return
	}

	// add commands
	rootCmd.AddCommand(serverCommands())
	rootCmd.AddCommand(scanCommands())

	err = rootCmd.Execute()
	if err != nil {
		zap.L().Fatal("Error executing command", zap.Error(err))
	}
}

func preRun(cmd *cobra.Command) error {
	if err := config.Init(cmd.Flags()); err != nil {
		return fmt.Errorf("error initializing config: %s", err)
	}

	if err := config.ReadConfigFile(); err != nil {
		return fmt.Errorf("error reading config: %s", err)
	}

	return nil
}

func run(ctx context.Context, action action.Action, args []string) error {
	return action(ctx, args)
}

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

// scanCommands returns the 'scan' subcommand.
func scanCommands() *cobra.Command {
	scanCmd := &cobra.Command{
		Use:   "scan",
		Short: "scan the archive",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return preRun(cmd)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd.Context(), scan.Scan, args)
		},
	}
	flag.Scan(scanCmd, config.Defaults)

	return scanCmd
}
