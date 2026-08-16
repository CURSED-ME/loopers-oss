package riskprofile

// Config holds persistent agent risk profile configuration.
// Loaded by server.go via viper.UnmarshalKey("risk_profile", &cfg).
type Config struct {
	Enabled                 bool   `mapstructure:"enabled"`
	TTL                     string `mapstructure:"ttl"`                       // e.g. "720h", "0" for permanent (no expiry)
	AutoQuarantineThreshold int    `mapstructure:"auto_quarantine_threshold"` // default 75
	PermanentBlockThreshold int    `mapstructure:"permanent_block_threshold"` // default 90
}

// DefaultConfig returns safe, production-grade defaults for risk profiling.
func DefaultConfig() Config {
	return Config{
		Enabled:                 true,
		TTL:                     "0",
		AutoQuarantineThreshold: 75,
		PermanentBlockThreshold: 90,
	}
}
