package main

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	gob.Register(sessionkeys.SessionKey(0))

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
		Use:           "pupjournal [--config-path File]",
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

	// make config directory
	zap.L().Debug("Creating config directory", zap.String("path", viper.GetString(config.Keys.DataFolder)))
	if err := os.Mkdir(viper.GetString(config.Keys.DataFolder), os.FileMode(0750)); err != nil && !errors.Is(err, os.ErrExist) {
		log.Fatal(err)
	}

	return nil
}

func run(ctx context.Context, action action.Action, args []string) error {
	return action(ctx, args)
}
