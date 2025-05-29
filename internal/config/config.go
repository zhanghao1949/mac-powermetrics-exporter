package config

// Config holds the application configuration
type Config struct {
	Port string
}

// New creates a new configuration with default values
func New() *Config {
	return &Config{
		Port: ":9127",
	}
}
