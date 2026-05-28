package main

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

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

				activeStr := "❌"
				if meta.Active == "true" {
					activeStr = "✅"
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
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hash := args[0]

		redisClient, err := getRedisClient()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

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
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hash := args[0]

		redisClient, err := getRedisClient()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

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
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hash := args[0]

		redisClient, err := getRedisClient()
		if err != nil {
			logging.Logger.Fatal().Err(err).Msg("failed to connect to redis")
		}
		defer redisClient.Close()

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
		reader := bufio.NewReader(os.Stdin)

		ui.PrintHeader("🔒 Loopers Setup Wizard\nConfigure your AI cost firewall")
		fmt.Println()

		// 1. Providers
		fmt.Println("  Step 1 of 4 — Providers")
		fmt.Println("  Which AI providers will you use?")
		fmt.Println("  Available: openai, anthropic, gemini, bedrock, azure, mistral, groq, cohere, deepseek, together")
		fmt.Print("  › [openai, anthropic]: ")
		providersInput, _ := reader.ReadString('\n')
		providersInput = strings.TrimSpace(providersInput)
		if providersInput == "" {
			providersInput = "openai, anthropic"
		}
		fmt.Println()

		// 2. Daily Limit
		fmt.Println("  Step 2 of 4 — Daily Budget")
		fmt.Println("  Default daily spend limit in USD")
		fmt.Print("  › [10.00]: ")
		dailyInput, _ := reader.ReadString('\n')
		dailyInput = strings.TrimSpace(dailyInput)
		if dailyInput == "" {
			dailyInput = "10.00"
		}
		fmt.Println()

		// 3. Hourly Limit
		fmt.Println("  Step 3 of 4 — Hourly Budget")
		fmt.Println("  Default hourly spend limit in USD")
		fmt.Print("  › [2.00]: ")
		hourlyInput, _ := reader.ReadString('\n')
		hourlyInput = strings.TrimSpace(hourlyInput)
		if hourlyInput == "" {
			hourlyInput = "2.00"
		}
		fmt.Println()

		// 4. Redis URL
		fmt.Println("  Step 4 of 4 — Redis URL")
		fmt.Println("  Where is your Redis instance running?")
		fmt.Print("  › [localhost:6379]: ")
		redisInput, _ := reader.ReadString('\n')
		redisInput = strings.TrimSpace(redisInput)
		if redisInput == "" {
			redisInput = "localhost:6379"
		}
		fmt.Println()

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
