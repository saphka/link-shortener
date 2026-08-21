package config

import (
	"context"
	"time"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	Port           uint16        `env:"PORT, default=8080"`
	ShutdownPeriod time.Duration `env:"SHUTDOWN_PERIOD, default=5s"`
	DB             Database
}

type Database struct {
	Host     string `env:"DATABASE_HOST, default=localhost"`
	Port     uint16 `env:"DATABASE_PORT, default=5432"`
	Name     string `env:"DATABASE_NAME, default=postgres"`
	User     string `env:"DATABASE_USER, default=postgres"`
	Password string `env:"DATABASE_PASSWORD, default=postgres"`
}

func Load(ctx context.Context, prefix string) (Config, error) {
	cfg := Config{}
	if err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   &cfg,
		Lookuper: envconfig.PrefixLookuper(prefix, envconfig.OsLookuper()),
	}); err != nil {
		return cfg, err
	}
	return cfg, nil
}
