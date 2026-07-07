package loop

type Config struct {
	Enabled     bool              `mapstructure:"enabled"`
	Fingerprint FingerprintConfig `mapstructure:"fingerprint"`
	Velocity    VelocityConfig    `mapstructure:"velocity"`
	Stall       StallConfig       `mapstructure:"stall"`
}

type FingerprintConfig struct {
	// Threshold is the number of identical requests within WindowSeconds to trigger a block.
	// NOTE: JSON map key ordering in arbitrary structs may cause identical requests to hash differently.
	// Default: 3
	Threshold int `mapstructure:"threshold"`

	// WindowSeconds is the sliding window in seconds for the fingerprint ring.
	// Default: 60 for rapid HFT-style agents. Consider 300 for slower, low-frequency agents.
	WindowSeconds int `mapstructure:"window_seconds"`

	// SimilarityThreshold is the minimum Jaccard similarity (0.0 to 1.0) on bi-grams
	// required to consider two requests as part of a loop.
	// Default: 0.95
	SimilarityThreshold float64 `mapstructure:"similarity_threshold"`

	// DefeatPadding enables structural truncation of large JSON string values
	// to prevent semantic padding attacks.
	// Default: false (due to potential CPU overhead on massive payloads)
	DefeatPadding bool `mapstructure:"defeat_padding"`
}

type VelocityConfig struct {
	// MaxRPS is the maximum requests per 10-second rolling window bucket, scaled by 10.
	// NOTE: This actually enforces max requests per 10s bucket. A burst of requests at the start
	// of a bucket may briefly exceed the literal per-second rate.
	// 0 = disabled. Default: 0 (disabled)
	MaxRPS float64 `mapstructure:"max_rps"`

	// MaxEndpointRepeats is the maximum endpoint repetitions within RepeatWindowSeconds.
	// 0 = disabled. Default: 0 (disabled)
	MaxEndpointRepeats  int `mapstructure:"max_endpoint_repeats"`
	RepeatWindowSeconds int `mapstructure:"repeat_window_seconds"`
}

type StallConfig struct {
	// MinHammingDistance is the minimum Hamming distance between sequential request hashes
	// required to be considered "progressing". 0 = disabled. Default: 0 (disabled)
	// IMPORTANT: Session IDs MUST be globally unique (e.g. UUID). Reusing session IDs will
	// contaminate the stall state history across sessions.
	MinHammingDistance int `mapstructure:"min_hamming_distance"`

	// LowDiversityThreshold is the number of sequential low-diversity requests before triggering.
	// Default: 5
	LowDiversityThreshold int `mapstructure:"low_diversity_threshold"`

	// Action on stall: "warn" (log+metric only) or "block".
	// Default: "warn"
	Action string `mapstructure:"action"`
}
