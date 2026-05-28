package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/loopers-oss/loopers/cmd/loopers/ui"
	"github.com/loopers-oss/loopers/internal/budget"
	"github.com/loopers-oss/loopers/internal/keyring"
	"github.com/loopers-oss/loopers/internal/logging"
	"github.com/loopers-oss/loopers/internal/pricing"
	"github.com/loopers-oss/loopers/internal/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	keyName      string
	keyProvider  string
	minuteLimit  string
	hourlyLimit  string
	dailyLimit   string
	weeklyLimit  string
	monthlyLimit string
)

func getRedisClient() (*budget.Client, error) {
	addr := viper.GetString("redis.addr")
	if addr == "" {
		addr = "localhost:6379"
	}
	password := viper.GetString("redis.password")
	db := viper.GetInt("redis.db")

	return budget.NewClient(addr, password, db)
}

func fetchActiveKeys(rdb *budget.Client) ([]huh.Option[string], error) {
	ctx := context.Background()
	iter := rdb.GetUnderlyingClient().Scan(ctx, 0, "loopers:key:*", 0).Iterator()
	var options []huh.Option[string]
	for iter.Next(ctx) {
		redisKey := iter.Val()
		h := redisKey[len("loopers:key:"):]
		var meta keyring.KeyMetadata
		if err := rdb.GetUnderlyingClient().HGetAll(ctx, redisKey).Scan(&meta); err == nil && meta.Active == "true" {
			displayHash := h
			if len(displayHash) > 12 {
				displayHash = displayHash[:12] + "..."
			}
			label := fmt.Sprintf("%s · %s", meta.Name, displayHash)
			options = append(options, huh.NewOption(label, h))
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	return options, nil
}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the loopers proxy server",
	Run: func(cmd *cobra.Command, args []string) {
		logLevel := viper.GetString("log.level")
		if logLevel == "" {
			logLevel = "info"
		}
		logging.InitLogger(logLevel)

		logging.Logger.Info().Msg("Starting Loopers server...")

		pricingPath := viper.GetString("pricing_path")
		if pricingPath == "" {
			pricingPath = "./pricing.yaml"
		}
		pricingStore, err := pricing.LoadStore(pricingPath)
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("Failed to load pricing file")
		}

		redisClient, err := getRedisClient()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("Failed to connect to Redis")
		}

		s := server.NewServer(redisClient, pricingStore)

		port := viper.GetString("server.port")
		if port == "" {
			port = "8080"
		}

		srv := &http.Server{
			Addr:    ":" + port,
			Handler: s.GetRouter(),
		}

		server.ListenAndServeWithGracefulShutdown(srv, redisClient)
	},
}

var keysCmd = &cobra.Command{
	Use:   "keys",
	Short: "Manage proxy keys",
}

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new proxy key",
	Run: func(cmd *cobra.Command, args []string) {
		if (keyName == "" || keyProvider == "") && ui.IsInteractive() {
			ui.PrintLogo()
			err := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Key Name").
						Description("A descriptive name for this key (e.g. 'my-app')").
						Value(&keyName).
						Validate(func(s string) error {
							if s == "" {
								return fmt.Errorf("name is required")
							}
							return nil
						}),
					huh.NewSelect[string]().
						Title("Provider").
						Options(
							huh.NewOption("OpenAI", "openai"),
							huh.NewOption("Anthropic", "anthropic"),
							huh.NewOption("Gemini", "gemini"),
							huh.NewOption("Bedrock", "bedrock"),
							huh.NewOption("Azure", "azure"),
							huh.NewOption("Mistral", "mistral"),
							huh.NewOption("Groq", "groq"),
							huh.NewOption("Cohere", "cohere"),
							huh.NewOption("DeepSeek", "deepseek"),
							huh.NewOption("Together", "together"),
						).
						Value(&keyProvider),
				),
			).WithTheme(ui.GetHuhTheme()).Run()
			if err != nil {
				return
			}
		}

		if keyName == "" || keyProvider == "" {
			logging.Logger.Fatal().Msg("flags --name and --provider are required")
		}
		validProviders := map[string]bool{
			"openai": true, "anthropic": true, "gemini": true, "bedrock": true, "azure": true, "mistral": true,
			"groq": true, "cohere": true, "deepseek": true, "together": true,
		}
		if !validProviders[keyProvider] {
			logging.Logger.Fatal().Msg("provider must be one of: openai, anthropic, gemini, bedrock, azure, mistral, groq, cohere, deepseek, together")
		}

		rawKey, err := keyring.GenerateRawKey()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to generate key")
		}
		hash := keyring.HashKey(rawKey)

		redisClient, err := getRedisClient()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

		ctx := context.Background()
		rdb := redisClient.GetUnderlyingClient()

		key := fmt.Sprintf("loopers:key:%s", hash)
		err = rdb.HSet(ctx, key, map[string]interface{}{
			"name":       keyName,
			"provider":   keyProvider,
			"created_at": time.Now().UTC().Format(time.RFC3339),
			"active":     "true",
		}).Err()

		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to store key in redis")
		}

		ui.PrintKeyCard(keyName, keyProvider, rawKey, hash)
	},
}

var keysListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all proxy keys",
	Run: func(cmd *cobra.Command, args []string) {
		redisClient, err := getRedisClient()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

		ctx := context.Background()
		rdb := redisClient.GetUnderlyingClient()

		iter := rdb.Scan(ctx, 0, "loopers:key:*", 0).Iterator()

		headers := []string{"Hash", "Name", "Provider", "Created At", "Active"}
		var rows [][]string

		for iter.Next(ctx) {
			redisKey := iter.Val()
			hash := redisKey[len("loopers:key:"):]

			displayHash := hash
			if len(displayHash) > 12 {
				displayHash = displayHash[:12] + "..."
			}

			var meta keyring.KeyMetadata
			if err := rdb.HGetAll(ctx, redisKey).Scan(&meta); err == nil {
				t, _ := time.Parse(time.RFC3339, meta.CreatedAt)
				timeStr := t.Format("2006-01-02 15:04 UTC")
				if meta.CreatedAt == "" {
					timeStr = "Unknown"
				}

				activeStr := "✗"
				if meta.Active == "true" {
					activeStr = "✓"
				}

				rows = append(rows, []string{displayHash, meta.Name, meta.Provider, timeStr, activeStr})
			}
		}

		if err := iter.Err(); err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed scanning keys")
		}

		fmt.Printf("  Keys (%d total)\n", len(rows))
		if len(rows) > 0 {
			ui.PrintTable(headers, rows)
		}
	},
}

var keysRevokeCmd = &cobra.Command{
	Use:   "revoke [hash]",
	Short: "Revoke a proxy key",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var hash string
		if len(args) > 0 {
			hash = args[0]
		}

		redisClient, err := getRedisClient()
		if err != nil {
			if ui.IsInteractive() && len(args) == 0 {
				ui.Error("Failed to connect to Redis. Ensure it is running.")
				return
			}
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

		if hash == "" && ui.IsInteractive() {
			ui.PrintLogo()
			options, err := fetchActiveKeys(redisClient)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to fetch keys: %v", err))
				return
			}
			if len(options) == 0 {
				ui.Error("No active keys found to revoke")
				return
			}

			err = huh.NewSelect[string]().
				Title("Select Key to Revoke").
				Options(options...).
				Value(&hash).
				WithTheme(ui.GetHuhTheme()).
				Run()

			if err != nil {
				return
			}
		}

		if hash == "" {
			logging.Logger.Fatal().Msg("hash argument is required")
		}

		ctx := context.Background()
		rdb := redisClient.GetUnderlyingClient()
		key := fmt.Sprintf("loopers:key:%s", hash)

		exists, err := rdb.Exists(ctx, key).Result()
		if err != nil || exists == 0 {
			logging.Logger.Fatal().Msg("Key hash not found")
		}

		err = rdb.HSet(ctx, key, "active", "false").Err()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to revoke key")
		}

		ui.Success("Key revoked successfully.")
	},
}

var budgetCmd = &cobra.Command{
	Use:   "budget",
	Short: "Manage budgets",
}

var budgetSetCmd = &cobra.Command{
	Use:   "set [hash]",
	Short: "Set daily, hourly, weekly, monthly, and minute budgets for a key hash",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var hash string
		if len(args) > 0 {
			hash = args[0]
		}

		redisClient, err := getRedisClient()
		if err != nil {
			if ui.IsInteractive() && len(args) == 0 {
				ui.Error("Failed to connect to Redis. Ensure it is running.")
				return
			}
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

		if hash == "" && ui.IsInteractive() {
			ui.PrintLogo()
			options, err := fetchActiveKeys(redisClient)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to fetch keys: %v", err))
				return
			}
			if len(options) == 0 {
				ui.Error("No active keys found")
				return
			}

			err = huh.NewSelect[string]().
				Title("Select Key").
				Options(options...).
				Value(&hash).
				WithTheme(ui.GetHuhTheme()).
				Run()

			if err != nil {
				return
			}
		}

		if hash == "" {
			logging.Logger.Fatal().Msg("hash argument is required")
		}

		if (minuteLimit == "" && hourlyLimit == "" && dailyLimit == "" && weeklyLimit == "" && monthlyLimit == "") && ui.IsInteractive() {
			validateFloat := func(s string) error {
				if s == "" {
					return nil
				}
				v, err := strconv.ParseFloat(s, 64)
				if err != nil || v < 0 {
					return fmt.Errorf("must be a valid positive number")
				}
				return nil
			}

			err := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().Title("Minute Limit (USD)").Value(&minuteLimit).Validate(validateFloat),
					huh.NewInput().Title("Hourly Limit (USD)").Value(&hourlyLimit).Validate(validateFloat),
					huh.NewInput().Title("Daily Limit (USD)").Value(&dailyLimit).Validate(validateFloat),
					huh.NewInput().Title("Weekly Limit (USD)").Value(&weeklyLimit).Validate(validateFloat),
					huh.NewInput().Title("Monthly Limit (USD)").Value(&monthlyLimit).Validate(validateFloat),
				),
			).WithTheme(ui.GetHuhTheme()).Run()

			if err != nil {
				return
			}
		}

		ctx := context.Background()
		rdb := redisClient.GetUnderlyingClient()

		keyExist, err := rdb.Exists(ctx, fmt.Sprintf("loopers:key:%s", hash)).Result()
		if err != nil || keyExist == 0 {
			logging.Logger.Fatal().Msg("key not found in registry")
		}

		configKey := fmt.Sprintf("loopers:budget:%s:config", hash)

		fields := make(map[string]interface{})
		if minuteLimit != "" {
			fields["minute"] = minuteLimit
		}
		if hourlyLimit != "" {
			fields["hourly"] = hourlyLimit
		}
		if dailyLimit != "" {
			fields["daily"] = dailyLimit
		}
		if weeklyLimit != "" {
			fields["weekly"] = weeklyLimit
		}
		if monthlyLimit != "" {
			fields["monthly"] = monthlyLimit
		}

		if len(fields) == 0 {
			logging.Logger.Fatal().Msg("either --minute, --hourly, --daily, --weekly, or --monthly must be provided")
		}

		err = rdb.HSet(ctx, configKey, fields).Err()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to set budget")
		}

		ui.Success("Budget set successfully.")
	},
}

var budgetStatusCmd = &cobra.Command{
	Use:   "status [hash]",
	Short: "Get budget status for a key hash",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var hash string
		if len(args) > 0 {
			hash = args[0]
		}

		redisClient, err := getRedisClient()
		if err != nil {
			if ui.IsInteractive() && len(args) == 0 {
				ui.Error("Failed to connect to Redis. Ensure it is running.")
				return
			}
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

		if hash == "" && ui.IsInteractive() {
			ui.PrintLogo()
			options, err := fetchActiveKeys(redisClient)
			if err != nil {
				ui.Error(fmt.Sprintf("Failed to fetch keys: %v", err))
				return
			}
			if len(options) == 0 {
				ui.Error("No active keys found")
				return
			}

			err = huh.NewSelect[string]().
				Title("Select Key to View").
				Options(options...).
				Value(&hash).
				WithTheme(ui.GetHuhTheme()).
				Run()

			if err != nil {
				return
			}
		}

		if hash == "" {
			logging.Logger.Fatal().Msg("hash argument is required")
		}

		ctx := context.Background()
		rdb := redisClient.GetUnderlyingClient()

		keyExist, err := rdb.Exists(ctx, fmt.Sprintf("loopers:key:%s", hash)).Result()
		if err != nil || keyExist == 0 {
			logging.Logger.Fatal().Msg("key not found in registry")
		}

		status, err := redisClient.GetBudgetStatus(ctx, hash)
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to get budget status")
		}

		displayHash := hash
		if len(displayHash) > 12 {
			displayHash = displayHash[:12] + "..."
		}

		keyConfig, _ := rdb.HGetAll(ctx, fmt.Sprintf("loopers:key:%s", hash)).Result()
		name := keyConfig["name"]
		provider := keyConfig["provider"]

		fmt.Printf("  Budget Status  ›  %s  (%s / %s)\n\n", displayHash, name, provider)

		windowsOrdered := []string{"minute", "hourly", "daily", "weekly", "monthly"}
		for _, w := range windowsOrdered {
			s := status[w]
			ui.PrintBudgetBar(w, s.CurrentSpend, s.Limit)
		}
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Interactively initialize Loopers configuration",
	Run: func(cmd *cobra.Command, args []string) {
		ui.PrintLogo()
		ui.PrintHeader("Loopers Setup Wizard\nConfigure your AI cost firewall")
		fmt.Println()

		providersInput := "openai, anthropic"
		dailyInput := "10.00"
		hourlyInput := "2.00"
		redisInput := "localhost:6379"

		if ui.IsInteractive() {
			err := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Providers").
						Description("Available: openai, anthropic, gemini, bedrock, azure, mistral, groq, cohere, deepseek, together").
						Value(&providersInput),
					huh.NewInput().
						Title("Daily Budget (USD)").
						Description("Default daily spend limit").
						Value(&dailyInput).
						Validate(func(s string) error {
							v, err := strconv.ParseFloat(s, 64)
							if err != nil || v <= 0 {
								return fmt.Errorf("must be a valid number > 0")
							}
							return nil
						}),
					huh.NewInput().
						Title("Hourly Budget (USD)").
						Description("Default hourly spend limit").
						Value(&hourlyInput).
						Validate(func(s string) error {
							v, err := strconv.ParseFloat(s, 64)
							if err != nil || v <= 0 {
								return fmt.Errorf("must be a valid number > 0")
							}
							return nil
						}),
					huh.NewInput().
						Title("Redis URL").
						Description("Where is your Redis instance running?").
						Value(&redisInput).
						Validate(func(s string) error {
							if !strings.Contains(s, ":") {
								return fmt.Errorf("must match host:port format")
							}
							return nil
						}),
				),
			).WithTheme(ui.GetHuhTheme()).Run()

			if err != nil {
				return
			}
		}

		// Generate loopers.yaml
		yamlContent := fmt.Sprintf(`server:
  port: 8080
redis:
  addr: "%s"
  password: ""
  db: 0
log:
  level: "info"
pricing_path: "./pricing.yaml"

alerting:
  webhook_url: "https://example.com/webhook"
  thresholds:
    - percent: 50
      message: "Budget 50%% consumed"
    - percent: 80
      message: "Budget 80%% consumed — approaching limit"
    - percent: 95
      message: "Budget 95%% consumed — imminent cutoff"
`, redisInput)

		err := os.WriteFile("loopers.yaml", []byte(yamlContent), 0644)
		if err != nil {
			ui.Error(fmt.Sprintf("Error writing loopers.yaml: %v", err))
			return
		}
		ui.Success("loopers.yaml written")

		// Generate docker-compose.yml
		composeContent := `version: '3.8'

services:
  redis:
    image: redis:7-alpine
    container_name: loopers-redis
    ports:
      - "6379:6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 2s
      timeout: 2s
      retries: 5
    networks:
      - loopers-net

  loopers:
    build: .
    container_name: loopers-proxy
    ports:
      - "8080:8080"
    environment:
      - REDIS_ADDR=redis:6379
      - SERVER_PORT=8080
      - PRICING_PATH=/app/pricing.yaml
    depends_on:
      redis:
        condition: service_healthy
    networks:
      - loopers-net

networks:
  loopers-net:
    driver: bridge
`
		err = os.WriteFile("docker-compose.yml", []byte(composeContent), 0644)
		if err != nil {
			ui.Error(fmt.Sprintf("Error writing docker-compose.yml: %v", err))
			return
		}
		ui.Success("docker-compose.yml written")

		fmt.Println()
		fmt.Println("  Next steps:")
		fmt.Println("    1. docker-compose up -d")
		fmt.Println("    2. loopers keys create --name my-app --provider openai")
		fmt.Printf("    3. loopers budget set <hash> --daily %s --hourly %s\n", dailyInput, hourlyInput)
	},
}

func init() {
	// Root flags
	rootCmd.AddCommand(serveCmd)
	rootCmd.AddCommand(initCmd)

	// Keys commands
	keysCreateCmd.Flags().StringVar(&keyName, "name", "", "Name of the proxy key (required)")
	keysCreateCmd.Flags().StringVar(&keyProvider, "provider", "", "Provider for the key (openai|anthropic|gemini|bedrock|azure|mistral) (required)")
	keysCmd.AddCommand(keysCreateCmd)
	keysCmd.AddCommand(keysListCmd)
	keysCmd.AddCommand(keysRevokeCmd)
	rootCmd.AddCommand(keysCmd)

	// Budget commands
	budgetSetCmd.Flags().StringVar(&minuteLimit, "minute", "", "Minute budget in USD")
	budgetSetCmd.Flags().StringVar(&hourlyLimit, "hourly", "", "Hourly budget in USD")
	budgetSetCmd.Flags().StringVar(&dailyLimit, "daily", "", "Daily budget in USD")
	budgetSetCmd.Flags().StringVar(&weeklyLimit, "weekly", "", "Weekly budget in USD")
	budgetSetCmd.Flags().StringVar(&monthlyLimit, "monthly", "", "Monthly budget in USD")
	budgetCmd.AddCommand(budgetSetCmd)
	budgetCmd.AddCommand(budgetStatusCmd)
	rootCmd.AddCommand(budgetCmd)
}
