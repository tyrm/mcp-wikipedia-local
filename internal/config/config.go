package config

import (
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Init starts config collection.
func Init(flags *pflag.FlagSet) error {
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if err := viper.BindPFlags(flags); err != nil {
		return err
	}

	return nil
}

// ReadConfigFile reads the config file from disk if config path is sent.
func ReadConfigFile() error {
	if configPath := viper.GetString(Keys.ConfigPath); configPath != "" {
		viper.SetConfigFile(configPath)

		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}

	return nil
}
