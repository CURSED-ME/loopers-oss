package main

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type FingerprintConfig struct {
	Threshold     int `mapstructure:"threshold"`
	WindowSeconds int `mapstructure:"window_seconds"`
}

type Config struct {
	Enabled     bool              `mapstructure:"enabled"`
	Fingerprint FingerprintConfig `mapstructure:"fingerprint"`
}

func main() {
	yamlStr := `
loop_detection:
  enabled: true
  fingerprint:
    threshold: 3
    window_seconds: 60
`
	viper.SetConfigType("yaml")
	if err := viper.ReadConfig(strings.NewReader(yamlStr)); err != nil {
		fmt.Printf("Read error: %v\n", err)
	}

	var cfg Config
	if err := viper.UnmarshalKey("loop_detection", &cfg); err != nil {
		fmt.Printf("Unmarshal error: %v\n", err)
	}

	fmt.Printf("Parsed config: %+v\n", cfg)
}
