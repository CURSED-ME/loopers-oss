package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/loopers-oss/loopers/internal/pricing"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose Loopers configuration and connectivity",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running Loopers Diagnostics...")
		fmt.Println()

		allPass := true

		// 1. Check loopers.yaml config
		hasConfigError := false
		if !viper.IsSet("redis.addr") {
			fmt.Println("❌ loopers.yaml missing field: redis.addr")
			hasConfigError = true
		}
		if !viper.IsSet("server.port") {
			fmt.Println("❌ loopers.yaml missing field: server.port")
			hasConfigError = true
		}
		if !viper.IsSet("pricing_path") {
			fmt.Println("❌ loopers.yaml missing field: pricing_path")
			hasConfigError = true
		}

		if hasConfigError {
			allPass = false
		} else {
			fmt.Println("✅ loopers.yaml schema valid")
		}

		// 2. Check Redis Connectivity
		start := time.Now()
		redisClient, err := getRedisClient()
		if err != nil {
			fmt.Printf("❌ Redis connection failed: %v\n", err)
			allPass = false
		} else {
			err = redisClient.Ping(context.Background())
			if err != nil {
				fmt.Printf("❌ Redis ping failed: %v\n", err)
				allPass = false
			} else {
				latency := time.Since(start).Milliseconds()
				fmt.Printf("✅ Redis connection OK (%dms)\n", latency)
			}
		}

		// 3. Validate pricing.yaml
		pricingPath := viper.GetString("pricing_path")
		if pricingPath == "" {
			pricingPath = "./pricing.yaml"
		}
		_, err = pricing.LoadStore(pricingPath)
		if err != nil {
			fmt.Printf("❌ pricing.yaml validation failed: %v\n", err)
			allPass = false
		} else {
			fmt.Println("✅ pricing.yaml loaded successfully")
		}

		// 4. Check that at least one key exists and has budgets
		if redisClient != nil && err == nil {
			rdb := redisClient.GetUnderlyingClient()
			ctx := context.Background()

			// Check if keys exist
			keys, _ := rdb.Keys(ctx, "loopers:key:*").Result()
			if len(keys) == 0 {
				fmt.Println("⚠️  No keys found. Run 'loopers keys create' to create one.")
			} else {
				// Check if any budgets exist
				budgets, _ := rdb.Keys(ctx, "loopers:budget:*:config").Result()
				if len(budgets) == 0 {
					fmt.Println("⚠️  Keys exist, but no budgets configured. Run 'loopers budget set' to configure limits.")
				} else {
					fmt.Printf("✅ %d keys found, budgets configured\n", len(keys))
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
			fmt.Printf("❌ Proxy not responding on localhost:%s — is it running?\n", port)
			allPass = false
		} else {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				fmt.Printf("✅ Proxy is running and healthy on port %s\n", port)
			} else {
				fmt.Printf("❌ Proxy returned status %d on /health\n", resp.StatusCode)
				allPass = false
			}
		}

		fmt.Println()
		if allPass {
			fmt.Println("All systems go! Loopers is correctly configured and running.")
		} else {
			fmt.Println("Diagnostics completed with errors. Please fix the issues above.")
		}
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
