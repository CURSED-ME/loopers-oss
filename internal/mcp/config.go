package mcp

type Config struct {
	Enabled        bool                 `mapstructure:"enabled"`
	MaxRequestSize int64                `mapstructure:"max_request_size"`
	AllowedMethods []string             `mapstructure:"allowed_methods"`
	Servers        []ServerConfig       `mapstructure:"servers"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
	Sanitizer      SanitizerConfig      `mapstructure:"sanitizer"`
}

type SanitizerConfig struct {
	MaxDescriptionLength int                 `mapstructure:"max_description_length"`
	ToolAllowlist        map[string][]string `mapstructure:"tool_allowlist"`
}

type ServerConfig struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

type CircuitBreakerConfig struct {
	Enabled       bool `mapstructure:"enabled"`
	Threshold     int  `mapstructure:"threshold"`
	WindowSeconds int  `mapstructure:"window_seconds"`
}
