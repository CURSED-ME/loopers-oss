package mcp

type Config struct {
	Enabled        bool                 `mapstructure:"enabled"`
	Servers        []ServerConfig       `mapstructure:"servers"`
	CircuitBreaker CircuitBreakerConfig `mapstructure:"circuit_breaker"`
}

type ServerConfig struct {
	Name string `mapstructure:"name"`
	URL  string `mapstructure:"url"`
}

type CircuitBreakerConfig struct {
	Threshold     int `mapstructure:"threshold"`
	WindowSeconds int `mapstructure:"window_seconds"`
}
