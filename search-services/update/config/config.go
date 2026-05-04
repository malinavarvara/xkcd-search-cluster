package config

import (
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type XKCD struct {
	URL         string        `yaml:"url" env:"XKCD_URL" env-default:"https://xkcd.com"`
	Concurrency int           `yaml:"concurrency" env:"XKCD_CONCURRENCY" env-default:"1"`
	Timeout     time.Duration `yaml:"timeout" env:"XKCD_TIMEOUT" env-default:"10s"`
	CheckPeriod time.Duration `yaml:"check_period" env:"XKCD_CHECK_PERIOD" env-default:"1h"`
}

type Config struct {
	LogLevel      string `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
	Address       string `yaml:"update_address" env:"UPDATE_ADDRESS" env-default:"localhost:80"`
	XKCD          XKCD   `yaml:"xkcd"`
	DBAddress     string `yaml:"db_address" env:"DB_ADDRESS" env-default:"localhost:82"`
	WordsAddress  string `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"localhost:81"`
	BrokerAddress string `yaml:"broker_address" env:"BROKER_ADDRESS" env-default:"nats://nats:4222"`
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
