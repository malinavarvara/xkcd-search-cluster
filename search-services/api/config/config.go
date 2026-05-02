package config

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type APIServerConfig struct {
	Address string        `yaml:"address" env:"API_ADDRESS" env-default:":8080"`
	Timeout time.Duration `yaml:"timeout" env:"API_TIMEOUT" env-default:"5s"`
}

type Config struct {
	LogLevel          string          `yaml:"log_level" env:"LOG_LEVEL" env-default:"INFO"`
	LogFormat         string          `yaml:"log_format" env:"LOG_FORMAT" env-default:"text"`
	WordsAddress      string          `yaml:"words_address" env:"WORDS_ADDRESS" env-default:"words:8080"`
	UpdateAddress     string          `yaml:"update_address" env:"UPDATE_ADDRESS" env-default:"update:8080"`
	SearchAddress     string          `yaml:"search_address" env:"SEARCH_ADDRESS" env-default:"search:8080"`
	APIServer         APIServerConfig `yaml:"api_server"`
	AdminUser         string          `env:"ADMIN_USER" env-default:"admin"`
	AdminPassword     string          `env:"ADMIN_PASSWORD" env-default:"password"`
	TokenTTL          time.Duration   `env:"TOKEN_TTL" env-default:"2m"`
	SearchConcurrency int             `env:"SEARCH_CONCURRENCY" env-default:"10"`
	SearchRate        int             `env:"SEARCH_RATE" env-default:"100"`
}

func (c *Config) Validate() error {
	if c.APIServer.Timeout <= 0 {
		return errors.New("api_server.timeout must be positive")
	}
	if c.APIServer.Address == "" {
		return errors.New("api_server.address must not be empty")
	}
	if c.WordsAddress == "" {
		return errors.New("words_address must not be empty")
	}
	switch c.LogLevel {
	case "DEBUG", "INFO", "WARN", "ERROR":
		// valid log level
	default:
		return errors.New("invalid log level")
	}
	return nil
}

func MustLoad(configPath string) *Config {
	var cfg Config
	if _, err := os.Stat(configPath); err == nil {
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			slog.Error("failed to read config file", "path", configPath, "error", err)
			os.Exit(1)
		}
		slog.Info("config loaded from file", "path", configPath)
	}

	if err := cleanenv.ReadEnv(&cfg); err != nil {
		slog.Error("failed to read config from env", "error", err)
		os.Exit(1)
	}

	slog.Info("config finalized with environment variables")
	return &cfg
}
