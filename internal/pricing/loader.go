package pricing

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// ModelPrice holds the cost parameters for a specific model.
type ModelPrice struct {
	InputPer1M  float64 `mapstructure:"input_per_1m"`
	OutputPer1M float64 `mapstructure:"output_per_1m"`
	Fallback    string  `mapstructure:"fallback"`
}

// ProviderConfig holds the pricing config for a specific provider.
type ProviderConfig struct {
	DefaultMaxOutputTokens int                   `mapstructure:"default_max_output_tokens"`
	Models                 map[string]ModelPrice `mapstructure:"models"`
}

// Config represents the schema of pricing.yaml.
type Config struct {
	Providers map[string]ProviderConfig `mapstructure:"providers"`
}

// Store is a thread-safe in-memory store for LLM pricing.
type Store struct {
	mu     sync.RWMutex
	config Config
}

// LoadStore loads the pricing config from the specified YAML path.
func LoadStore(path string) (*Store, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read pricing config: %w", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pricing config: %w", err)
	}

	return &Store{config: config}, nil
}

// GetModelPrice returns the pricing rates (input & output per 1M tokens) and default max output tokens for a model.
// Falls back to provider's "_fallback" if model not found.
func (s *Store) GetModelPrice(provider, model string) (inputPer1M, outputPer1M float64, defaultMaxOutputTokens int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provConf, hasProv := s.config.Providers[provider]
	if !hasProv {
		return 0, 0, 0
	}

	defaultMaxOutputTokens = provConf.DefaultMaxOutputTokens

	modelPrice, hasModel := provConf.Models[model]
	if !hasModel {
		fallback, hasFallback := provConf.Models["_fallback"]
		if hasFallback {
			return fallback.InputPer1M, fallback.OutputPer1M, defaultMaxOutputTokens
		}
		return 0, 0, defaultMaxOutputTokens
	}

	return modelPrice.InputPer1M, modelPrice.OutputPer1M, defaultMaxOutputTokens
}

// GetFallback returns the configured fallback model for the given provider and model, or empty string if none configured.
func (s *Store) GetFallback(provider, model string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	provConf, hasProv := s.config.Providers[provider]
	if !hasProv {
		return ""
	}

	modelPrice, hasModel := provConf.Models[model]
	if !hasModel {
		return ""
	}

	return modelPrice.Fallback
}
