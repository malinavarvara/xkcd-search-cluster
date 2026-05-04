package config

import (
	"log/slog"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Address string `yaml:"words_address" env:"WORDS_ADDRESS" env-default:":8080"`
}

func MustLoad(configPath string) *Config {
	var cfg Config
	err := cleanenv.ReadConfig(configPath, &cfg)
	if configPath != "config.yaml" {
		if err != nil {
			slog.Error("failed to read config file", "path", configPath, "error", err)
			os.Exit(1)
		}
		slog.Info("config loaded from file", "path", configPath)
	} else {
		if err == nil {
			slog.Info("config loaded from file", "path", configPath)
		} else {
			slog.Info("config file not found or invalid, falling back to env", "path", configPath, "error", err)
			if err := cleanenv.ReadEnv(&cfg); err != nil {
				slog.Error("failed to read config from env", "error", err)
				os.Exit(1)
			}
			slog.Info("config loaded from env")
		}
	}
	return &cfg
}
