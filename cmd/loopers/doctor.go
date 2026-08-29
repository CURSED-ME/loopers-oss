package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/try-loopers/loopers-oss/cmd/loopers/ui"
	"github.com/try-loopers/loopers-oss/internal/pricing"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose Loopers firewall configuration, Redis connectivity, and security engines",
	Run: func(cmd *cobra.Command, args []string) {
		ui.PrintLogo()
		ui.PrintHeader("◎ Loopers AI Firewall Diagnostics")
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
				budgets, _ := rdb.Keys(ctx, "loopers:budget:{*}:config").Result()
				if len(budgets) == 0 {
					ui.Warn("Keys exist, but no budgets configured. Run 'loopers budget set'.")
				} else {
					ui.Success(fmt.Sprintf("%d keys found, budgets configured", len(keys)))
				}
			}
		}

		// 5. Check Firewall Security Engines
		fmt.Println()
		fmt.Println("  Firewall Security Engines")

		// Loop Detection
		if viper.GetBool("loop_detection.enabled") {
			ui.Success("Loop Detection Engine: ENABLED")
		} else {
			ui.Warn("Loop Detection Engine: DISABLED")
		}

		// MCP Response Inspector
		if viper.GetBool("mcp.inspector.enabled") {
			ui.Success("MCP Tool Response Inspector: ENABLED")
		} else {
			ui.Warn("MCP Tool Response Inspector: DISABLED")
		}

		// Outbound DLP Gate
		if viper.GetBool("server.dlp.enabled") {
			ui.Success(fmt.Sprintf("Outbound Semantic DLP Gate: ENABLED (Action: %s)", viper.GetString("server.dlp.action")))
		} else {
			ui.Warn("Outbound Semantic DLP Gate: DISABLED")
		}

		// Risk Profile Engine
		if viper.GetBool("risk_profile.enabled") {
			ui.Success("Persistent Risk Profile Engine: ENABLED")
		} else {
			ui.Warn("Persistent Risk Profile Engine: DISABLED")
		}

		// 6. Check if firewall runtime is running
		port := viper.GetString("server.port")
		if port == "" {
			port = "8080"
		}
		resp, err := http.Get(fmt.Sprintf("http://localhost:%s/health", port))
		if err != nil {
			ui.Error(fmt.Sprintf("Firewall runtime: not responding on :%s", port))
			issues++
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ui.Success(fmt.Sprintf("Firewall runtime: running on :%s", port))
			} else {
				ui.Error(fmt.Sprintf("Firewall runtime: returned status %d on /health", resp.StatusCode))
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
