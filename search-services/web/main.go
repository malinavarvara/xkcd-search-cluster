package main

import (
	"flag"
	"log/slog"
	"net/http"
	"os"

	"yadro.com/course/web/adapters/api"
	"yadro.com/course/web/adapters/httpserver"
	"yadro.com/course/web/config"
	"yadro.com/course/web/core"
)

func main() {
	// 1. Получаем путь к конфигу через флаг (удобно для Docker)
	configPath := flag.String("config", "/config.yaml", "path to config file")
	flag.Parse()

	// 2. Инициализируем логгер (slog) в зависимости от конфига
	setupLogger()

	// 3. Загружаем конфигурацию[cite: 6]
	cfg := config.MustLoad(*configPath)
	slog.Info("config loaded", "address", cfg.Address, "api_address", cfg.ApiAddress)

	// 4. Инициализируем адаптер внешней зависимости (API Client)[cite: 3, 6]
	// Он реализует порт ComicStore из core/ports.go
	apiClient := api.NewClient(cfg.ApiAddress)

	// 5. Инициализируем ядро (Бизнес-логика)
	// Передаем apiClient как реализацию интерфейса ComicStore
	service := core.NewWebService(apiClient)

	// 6. Инициализируем адаптер входящих запросов (HTTP Server)[cite: 3, 7]
	// Передаем сервис и пути к шаблонам/статике из конфига
	handler := httpserver.NewHandler(service, cfg.TmplPath, cfg.StaticPath)

	// 7. Запуск сервера[cite: 7]
	slog.Info("starting web server", "addr", cfg.Address)
	server := &http.Server{
		Addr:    cfg.Address,
		Handler: handler.Mux(),
	}

	if err := server.ListenAndServe(); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func setupLogger() {
	// Базовая настройка slog для вывода в текст
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
}
