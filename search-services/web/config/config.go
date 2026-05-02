package config

import (
	"log/slog"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	LogLevel   string `yaml:"log_level" env:"LOG_LEVEL" env-default:"DEBUG"`
	LogFormat  string `yaml:"log_format" env:"LOG_FORMAT" env-default:"text"`
	Address    string `yaml:"address" env:"WEB_ADDRESS" env-default:":8081"`
	ApiAddress string `yaml:"api_address" env:"API_ADDRESS" env-default:"http://api:8080"`
	// Исправлены дефолты для Docker
	StaticPath string `yaml:"static_path" env:"STATIC_PATH" env-default:"/static"`
	TmplPath   string `yaml:"tmpl_path" env:"TMPL_PATH" env-default:"/templates/*.html"`
}

func MustLoad(configPath string) *Config {
	var cfg Config
	// Если путь к конфигу пустой или файла нет, cleanenv.ReadEnv заполнит дефолты
	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		slog.Warn("config file not found, using env or defaults", "path", configPath)
		_ = cleanenv.ReadEnv(&cfg)
	}
	return &cfg
}
