package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/CURSED-ME/loopers-oss/cmd/loopers/ui"
	"github.com/CURSED-ME/loopers-oss/internal/logging"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "loopers",
	Short: "Loopers is the AI Firewall for the Agentic Era",
	Long: `Loopers is an open-source, bare-metal AI firewall designed for autonomous agents.
It provides real-time pre-call budget governance, runaway loop termination,
stateful FSM policy enforcement, MCP tool response inspection, persistent agent risk scoring,
and outbound semantic DLP protection across 500+ AI models.`,
	Run: func(cmd *cobra.Command, args []string) {
		if !ui.IsInteractive() {
			cmd.Help()
			return
		}

		for {
			app := ui.NewApp()
			p := tea.NewProgram(app, tea.WithAltScreen())
			m, err := p.Run()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error running UI: %v\n", err)
				return
			}

			appModel, ok := m.(ui.AppModel)
			if !ok || appModel.Action == "" || appModel.Action == "quit" {
				return // User aborted or exited
			}

			action := appModel.Action

			if strings.HasPrefix(action, "screen_") {
				runScreen(action)
				continue
			}

			// Dispatch to existing commands (like serve)
			parts := strings.Split(action, " ")
			subCmd, _, err := cmd.Find(parts)
			if err != nil || subCmd == cmd {
				fmt.Fprintf(os.Stderr, "unknown action: %s\n", action)
				os.Exit(1)
			}

			if subCmd.Run != nil || subCmd.RunE != nil {
				if action == "serve" {
					if _, err := os.Stat("loopers.yaml"); os.IsNotExist(err) {
						var runSetup bool
						huh.NewConfirm().
							Title("No loopers.yaml found. Run Quick Start Setup now?").
							Value(&runSetup).
							Run()

						if runSetup {
							runScreen("screen_init")
						}
						continue
					}
				}
			}

			if subCmd.Run != nil {
				viper.ReadInConfig()
				subCmd.Run(subCmd, []string{})
			} else if subCmd.RunE != nil {
				viper.ReadInConfig()
				if err := subCmd.RunE(subCmd, []string{}); err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
			}

			break
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

	viper.SetEnvPrefix("LOOPERS")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	viper.BindEnv("redis.addr", "REDIS_ADDR")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("server.port", "SERVER_PORT")
	viper.BindEnv("server.insecure_dev", "SERVER_INSECURE_DEV")
	viper.BindEnv("pricing_path", "PRICING_PATH")
	viper.BindEnv("log.level", "LOG_LEVEL")
	viper.BindEnv("zsp.escalation_secret", "LOOPERS_ZSP_ESCALATION_SECRET")

	if err := viper.ReadInConfig(); err == nil {
		// Log will be initialized properly in serve command, but let's initialize a basic logger here
		logging.InitLogger("info")
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
