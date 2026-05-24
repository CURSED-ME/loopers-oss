package main

import (
	"fmt"
	"os"

	"github.com/loopers-oss/loopers/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "loopers",
	Short: "Loopers is an airtight AI rate-limit and budget proxy",
	Long:  `An open-source, zero-storage proxy to enforce hard budget limits on LLM usage.`,
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./loopers.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose debug logging")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.loopers")
		viper.SetConfigType("yaml")
		viper.SetConfigName("loopers")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		// Log will be initialized properly in serve command, but let's initialize a basic logger here
		logging.InitLogger("info")
		logging.Logger.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())
	} else {
		logging.InitLogger("info")
	}

	if verbose {
		logging.InitLogger("debug")
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
