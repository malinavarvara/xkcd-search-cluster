package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LogLevel             string        `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
	LogFormat            string        `yaml:"log_format" env:"LOG_FORMAT" env-default:"text"`
	Address              string        `yaml:"address" env:"SEARCH_ADDRESS" env-default:"localhost:80"`
	DBAddress            string        `yaml:"db_address" env:"DB_ADDRESS" env-default:"localhost:82"`
	WordsAddress         string        `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"localhost:81"`
	BrokerAddress        string        `yaml:"broker_address" env:"BROKER_ADDRESS" env-default:"nats://localhost:4222"`
	IndexRebuildInterval time.Duration `yaml:"index_rebuild_interval" env:"INDEX_REBUILD_INTERVAL" env-default:"1h"`
	IndexTTL             time.Duration `yaml:"index_ttl" env:"INDEX_TTL" env-default:"24h"`
}

func MustLoad(configPath string) *Config {
	var cfg Config

	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			slog.Error("failed to read config file", "path", configPath, "error", err)
			os.Exit(1)
		}
		slog.Info("config loaded from file", "path", configPath)
	} else {
		slog.Info("loading config from environment variables")
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			slog.Error("failed to read config from env", "error", err)
			os.Exit(1)
		}
		slog.Info("config loaded from env")
	}

	return &cfg
}
