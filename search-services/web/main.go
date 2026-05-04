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
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()
	cfg := config.MustLoad(*configPath)

	setupLogger(cfg.LogLevel)
	slog.Info("config loaded", "address", cfg.Address, "api_address", cfg.ApiAddress)

	apiClient := api.NewClient(cfg.ApiAddress)

	service := core.NewWebService(apiClient)

	handler := httpserver.NewHandler(service, cfg.TmplPath, cfg.StaticPath, cfg.ApiAddress)

	server := &http.Server{
		Addr:    cfg.Address,
		Handler: handler.Mux(),
	}

	slog.Info("starting web server", "addr", cfg.Address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func setupLogger(levelStr string) {
	var level slog.Level
	switch levelStr {
	case "DEBUG":
		level = slog.LevelDebug
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
}
