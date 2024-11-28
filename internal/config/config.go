package config

import (
	"time"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port            string        `envconfig:"PORT" default:"3000"`
	DBPath          string        `envconfig:"DB_PATH" default:"urls.db"`
	BaseURL         string        `envconfig:"BASE_URL" default:"http://localhost:3000"`
	RateLimit       int           `envconfig:"RATE_LIMIT" default:"100"`
	RateLimitWindow time.Duration `envconfig:"RATE_LIMIT_WINDOW" default:"1m"`
	CleanupInterval time.Duration `envconfig:"CLEANUP_INTERVAL" default:"24h"`
	MaxURLAge       time.Duration `envconfig:"MAX_URL_AGE" default:"720h"` // 30 days
}

func LoadConfig() (*Config, error) {
	_ = godotenv.Load() // Load .env file if it exists

	var config Config
	if err := envconfig.Process("", &config); err != nil {
		return nil, err
	}
	return &config, nil
}
