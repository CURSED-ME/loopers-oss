package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loopers-oss/loopers/cmd/loopers/ui"
	"github.com/loopers-oss/loopers/internal/pricing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose Loopers configuration and connectivity",
	Run: func(cmd *cobra.Command, args []string) {
		ui.PrintHeader("🔍 Loopers Diagnostics")
		fmt.Println()

		issues := 0

		// 1. Check loopers.yaml config
		fmt.Println("  Config")
		if !viper.IsSet("redis.addr") {
			ui.Error("redis.addr missing")
			issues++
		} else {
			ui.Success(fmt.Sprintf("redis.addr: %s", viper.GetString("redis.addr")))
		}

		if !viper.IsSet("server.port") {
			ui.Error("server.port missing")
			issues++
		} else {
			ui.Success(fmt.Sprintf("server.port: %s", viper.GetString("server.port")))
		}

		pricingPath := viper.GetString("pricing_path")
		if pricingPath == "" {
			pricingPath = "./pricing.yaml"
		}
		if !viper.IsSet("pricing_path") {
			ui.Error("pricing_path missing")
			issues++
		} else {
			ui.Success(fmt.Sprintf("pricing_path: %s", pricingPath))
		}

		fmt.Println()
		fmt.Println("  Connectivity")

		// 2. Check Redis Connectivity
		start := time.Now()
		redisClient, err := getRedisClient()
		if err != nil {
			ui.Error(fmt.Sprintf("Redis: connection failed (%v)", err))
			issues++
		} else {
			err = redisClient.Ping(context.Background())
			if err != nil {
				ui.Error(fmt.Sprintf("Redis: ping failed (%v)", err))
				issues++
			} else {
				latency := time.Since(start).Milliseconds()
				ui.Success(fmt.Sprintf("Redis: OK (%dms)", latency))
			}
		}

		// 3. Validate pricing.yaml
		fmt.Println()
		fmt.Println("  Data")
		_, err = pricing.LoadStore(pricingPath)
		if err != nil {
			ui.Error(fmt.Sprintf("Pricing: validation failed (%v)", err))
			issues++
		} else {
			ui.Success("Pricing: loaded successfully")
		}

		// 4. Check that at least one key exists and has budgets
		if redisClient != nil && err == nil {
			rdb := redisClient.GetUnderlyingClient()
			ctx := context.Background()

			// Check if keys exist
			keys, _ := rdb.Keys(ctx, "loopers:key:*").Result()
			if len(keys) == 0 {
				ui.Warn("No keys found. Run 'loopers keys create' to create one.")
			} else {
				// Check if any budgets exist
				budgets, _ := rdb.Keys(ctx, "loopers:budget:*:config").Result()
				if len(budgets) == 0 {
					ui.Warn("Keys exist, but no budgets configured. Run 'loopers budget set'.")
				} else {
					ui.Success(fmt.Sprintf("%d keys found, budgets configured", len(keys)))
				}
			}
		}

		// 5. Check if proxy is running
		port := viper.GetString("server.port")
		if port == "" {
			port = "8080"
		}
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", port))
		if err != nil {
			ui.Error(fmt.Sprintf("Proxy: not responding on :%s", port))
			issues++
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ui.Success(fmt.Sprintf("Proxy: running on :%s", port))
			} else {
				ui.Error(fmt.Sprintf("Proxy: returned status %d on /health", resp.StatusCode))
				issues++
			}
		}

		fmt.Println()
		ui.PrintSummary(issues == 0, issues)
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
