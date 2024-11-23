package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

func NewConfig() (*Config, error) {
	_ = godotenv.Load()
	var c Config
	if err := env.Parse(&c); err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &c, nil
}

type Config struct {
	Local        bool          `env:"LOCAL" envDefault:"false"`
	LogLevel     string        `env:"LOG_LEVEL" envDefault:"warn"`
	Port         int           `env:"PORT" envDefault:"55555"`
	RedisURL     string        `env:"REDIS_URL" envDefault:"localhost:6379"`
	MaxMessages  int           `env:"MAX_MESSAGES" envDefault:"1000"`
	MaxRetention time.Duration `env:"MAX_RETENTION" envDefault:"7d"`
}
