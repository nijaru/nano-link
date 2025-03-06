package config

import (
	"fmt"
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/nijaru/nano-link/internal/errors"
)

// Config holds all application configuration loaded from environment variables
type Config struct {
	Port            string        `envconfig:"PORT" default:"3000"`
	DBPath          string        `envconfig:"DB_PATH" default:"urls.db"`
	BaseURL         string        `envconfig:"BASE_URL" default:"http://localhost:3000"`
	RateLimit       int           `envconfig:"RATE_LIMIT" default:"100"`
	RateLimitWindow time.Duration `envconfig:"RATE_LIMIT_WINDOW" default:"1m"`
	CleanupInterval time.Duration `envconfig:"CLEANUP_INTERVAL" default:"24h"`
	MaxURLAge       time.Duration `envconfig:"MAX_URL_AGE" default:"720h"` // 30 days
}

// Validate performs validation checks on the configuration
func (c *Config) Validate() error {
	if c.Port == "" {
		return errors.NewValidationError("port cannot be empty")
	}
	if c.DBPath == "" {
		return errors.NewValidationError("database path cannot be empty")
	}
	if c.RateLimit <= 0 {
		return errors.NewValidationError("rate limit must be positive")
	}
	if c.RateLimitWindow <= 0 {
		return errors.NewValidationError("rate limit window must be positive")
	}
	if c.CleanupInterval <= 0 {
		return errors.NewValidationError("cleanup interval must be positive")
	}
	if c.MaxURLAge <= 0 {
		return errors.NewValidationError("max URL age must be positive")
	}
	return nil
}

// LoadConfig loads configuration from environment variables and .env file
func LoadConfig() (*Config, error) {
	// Load .env file if it exists, ignore errors as file may not exist
	if err := godotenv.Load(); err != nil {
		// This is not a fatal error, we'll continue with environment variables
	}

	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, errors.NewInternalError(fmt.Sprintf("failed to process config: %v", err))
	}

	// Validate the loaded configuration
	if err := config.Validate(); err != nil {
		return nil, err
	}

	return &config, nil
}
