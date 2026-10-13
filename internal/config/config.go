// Package config loads application settings from environment variables.
package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	DatabaseURL string `env:"DATABASE_URL,required"`
	GRPCPort    string `env:"GRPC_PORT" envDefault:"50051"`

	JWT    JWT
	Broker Broker
	Outbox Outbox
}

type JWT struct {
	Secret string        `env:"JWT_SECRET,required"`
	TTL    time.Duration `env:"JWT_TTL" envDefault:"1h"`
}

type Broker struct {
	Host    string        `env:"BROKER_HOST" envDefault:"localhost"`
	Port    string        `env:"BROKER_PORT" envDefault:"9090"`
	Topic   string        `env:"BROKER_TOPIC" envDefault:"companies"`
	Timeout time.Duration `env:"BROKER_TIMEOUT" envDefault:"5s"` // per publish call
}

type Outbox struct {
	PollInterval time.Duration `env:"OUTBOX_POLL_INTERVAL" envDefault:"1s"`
	BatchSize    int           `env:"OUTBOX_BATCH_SIZE" envDefault:"100"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
