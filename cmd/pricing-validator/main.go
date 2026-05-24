package main

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type ModelPrice struct {
	InputPer1M  *float64 `yaml:"input_per_1m"`
	OutputPer1M *float64 `yaml:"output_per_1m"`
}

type ProviderConfig struct {
	DefaultMaxOutputTokens *int                  `yaml:"default_max_output_tokens"`
	Models                 map[string]*ModelPrice `yaml:"models"`
}

type PricingConfig struct {
	Providers map[string]*ProviderConfig `yaml:"providers"`
}

func main() {
	pricingPath := "pricing.yaml"
	if len(os.Args) > 1 {
		pricingPath = os.Args[1]
	}

	fmt.Printf("Validating pricing configuration at: %s\n", pricingPath)

	data, err := os.ReadFile(pricingPath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		os.Exit(1)
	}

	var config PricingConfig
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		fmt.Printf("Error parsing YAML: %v\n", err)
		os.Exit(1)
	}

	if len(config.Providers) == 0 {
		fmt.Println("Error: No providers found in configuration under root 'providers' key.")
		os.Exit(1)
	}

	// List of expected providers to double check
	expectedProviders := map[string]bool{
		"openai":    true,
		"anthropic": true,
		"gemini":    true,
		"bedrock":   true,
		"mistral":   true,
		"azure":     true,
	}

	for name, provider := range config.Providers {
		fmt.Printf("Checking provider: %s\n", name)

		if provider == nil {
			fmt.Printf("Error: Provider '%s' config is empty.\n", name)
			os.Exit(1)
		}

		if provider.DefaultMaxOutputTokens == nil {
			fmt.Printf("Error: Provider '%s' is missing 'default_max_output_tokens'.\n", name)
			os.Exit(1)
		}

		if *provider.DefaultMaxOutputTokens <= 0 {
			fmt.Printf("Error: Provider '%s' has invalid 'default_max_output_tokens' (%d). Must be greater than 0.\n", name, *provider.DefaultMaxOutputTokens)
			os.Exit(1)
		}

		if len(provider.Models) == 0 {
			fmt.Printf("Error: Provider '%s' has no models configured.\n", name)
			os.Exit(1)
		}

		// Ensure fallback is configured
		fallbackModel, hasFallback := provider.Models["_fallback"]
		if !hasFallback || fallbackModel == nil {
			fmt.Printf("Error: Provider '%s' is missing '_fallback' model configuration.\n", name)
			os.Exit(1)
		}

		for modelName, pricing := range provider.Models {
			if pricing == nil {
				fmt.Printf("Error: Model '%s' under provider '%s' has empty config.\n", modelName, name)
				os.Exit(1)
			}
			if pricing.InputPer1M == nil {
				fmt.Printf("Error: Model '%s' under provider '%s' is missing 'input_per_1m'.\n", modelName, name)
				os.Exit(1)
			}
			if *pricing.InputPer1M < 0 {
				fmt.Printf("Error: Model '%s' under provider '%s' has negative 'input_per_1m' value: %f.\n", modelName, name, *pricing.InputPer1M)
				os.Exit(1)
			}
			if pricing.OutputPer1M == nil {
				fmt.Printf("Error: Model '%s' under provider '%s' is missing 'output_per_1m'.\n", modelName, name)
				os.Exit(1)
			}
			if *pricing.OutputPer1M < 0 {
				fmt.Printf("Error: Model '%s' under provider '%s' has negative 'output_per_1m' value: %f.\n", modelName, name, *pricing.OutputPer1M)
				os.Exit(1)
			}
		}

		delete(expectedProviders, name)
	}

	// Warn if some core providers are missing
	if len(expectedProviders) > 0 {
		for missing := range expectedProviders {
			fmt.Printf("Warning: Expected core provider '%s' is missing from the configuration file.\n", missing)
		}
	}

	absolutePath, _ := filepath.Abs(pricingPath)
	fmt.Printf("✅ Pricing configuration is valid! (Path: %s)\n", absolutePath)
}
