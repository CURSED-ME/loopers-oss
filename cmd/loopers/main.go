package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/loopers-oss/loopers/cmd/loopers/ui"
	"github.com/loopers-oss/loopers/internal/logging"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"strings"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "loopers",
	Short: "Loopers is an airtight AI rate-limit and budget proxy",
	Long:  `An open-source, zero-storage proxy to enforce hard budget limits on LLM usage.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !ui.IsInteractive() {
			cmd.Help()
			return
		}

		ui.PrintLogo()

		var action string
		err := huh.NewSelect[string]().
			Title("Main Menu").
			Options(
				huh.NewOption("◎  Run Diagnostics", "doctor"),
				huh.NewOption("⊙  Initialize Workspace", "init"),
				huh.NewOption("⊕  Create Key", "keys create"),
				huh.NewOption("≡  List Keys", "keys list"),
				huh.NewOption("⊖  Revoke Key", "keys revoke"),
				huh.NewOption("◈  Set Budget", "budget set"),
				huh.NewOption("▤  Budget Status", "budget status"),
				huh.NewOption("▶  Start Proxy Server", "serve"),
			).
			Value(&action).
			WithTheme(ui.GetHuhTheme()).
			Run()

		if err != nil {
			return // User aborted
		}

		// Dispatch to existing commands
		parts := strings.Split(action, " ")
		subCmd, _, err := cmd.Find(parts)
		if err != nil || subCmd == cmd {
			fmt.Fprintf(os.Stderr, "unknown action: %s\n", action)
			os.Exit(1)
		}

		if subCmd.Run != nil {
			subCmd.Run(subCmd, []string{})
		}
	},
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
