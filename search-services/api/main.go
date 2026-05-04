package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"yadro.com/course/api/adapters/aaa"
	"yadro.com/course/api/adapters/rest"
	"yadro.com/course/api/adapters/rest/middleware"
	"yadro.com/course/api/adapters/search"
	"yadro.com/course/api/adapters/update"
	"yadro.com/course/api/adapters/words"
	"yadro.com/course/api/config"
	"yadro.com/course/api/core"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	cfg := config.MustLoad(configPath)

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	log := mustMakeLogger(cfg.LogLevel, cfg.LogFormat)
	log.Info("starting server", "config", cfg)

	aaaInstance, err := aaa.New(cfg.TokenTTL, log)
	if err != nil {
		return fmt.Errorf("failed to init AAA: %w", err)
	}
	loginHandler := rest.NewLoginHandler(aaaInstance, log)

	wordsClient, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("cannot init words adapter: %w", err)
	}
	defer func() {
		if err := wordsClient.Close(); err != nil {
			log.Error("failed to close words client", "error", err)
		}
	}()

	updateClient, err := update.NewClient(cfg.UpdateAddress, log)
	if err != nil {
		return fmt.Errorf("cannot init update adapter: %w", err)
	}
	defer func() {
		if err := updateClient.Close(); err != nil {
			log.Error("failed to close update client", "error", err)
		}
	}()

	searchClient, err := search.NewClient(cfg.SearchAddress, log)
	if err != nil {
		return fmt.Errorf("cannot init search adapter: %w", err)
	}
	defer func() {
		if err := searchClient.Close(); err != nil {
			log.Error("failed to close search client", "error", err)
		}
	}()

	pingers := map[string]core.Pinger{
		"words":  wordsClient,
		"update": updateClient,
		"search": searchClient,
	}

	searchHandler := rest.NewSearchHandler(log, searchClient)
	searchConcurrency := middleware.Concurrency(searchHandler.ServeHTTP, cfg.SearchConcurrency)
	isearchRate := middleware.Rate(searchHandler.ServeISearchHTTP, cfg.SearchRate)

	updateHandler := middleware.Auth(rest.NewUpdateHandler(log, updateClient), aaaInstance)
	dropHandler := middleware.Auth(rest.NewDropHandler(log, updateClient), aaaInstance)

	mux := http.NewServeMux()

	mux.Handle("GET /api/words", middleware.WithMetrics(rest.NewWordsHandler(log, wordsClient)))
	mux.Handle("GET /api/db/stats", middleware.WithMetrics(rest.NewStatsHandler(log, updateClient)))
	mux.Handle("GET /api/db/status", middleware.WithMetrics(rest.NewStatusHandler(log, updateClient)))
	mux.Handle("GET /api/ping", middleware.WithMetrics(rest.NewPingHandler(log, pingers)))
	mux.Handle("POST /api/login", middleware.WithMetrics(loginHandler))

	mux.Handle("GET /api/search", middleware.WithMetrics(searchConcurrency))
	mux.Handle("GET /api/isearch", middleware.WithMetrics(isearchRate))

	mux.Handle("POST /api/db/update", middleware.WithMetrics(updateHandler))
	mux.Handle("DELETE /api/db", middleware.WithMetrics(dropHandler))

	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, req *http.Request) {
		metrics.WritePrometheus(w, true)
	})

	handlerWithCORS := middleware.CORS(mux)

	server := &http.Server{
		Addr:        cfg.APIServer.Address,
		ReadTimeout: cfg.APIServer.Timeout,
		Handler:     handlerWithCORS,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Debug("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("erroneous shutdown", "error", err)
		}
	}()

	log.Info("server listening", "address", cfg.APIServer.Address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server closed unexpectedly: %w", err)
	}
	log.Info("server stopped gracefully")
	return nil
}

func mustMakeLogger(levelStr, format string) *slog.Logger {
	var level slog.Level
	switch levelStr {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "WARN":
		level = slog.LevelWarn
	case "ERROR":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, opts)
	default:
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	return slog.New(handler)
}
