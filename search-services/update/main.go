package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	updatepb "yadro.com/course/proto/update"
	"yadro.com/course/update/adapters/broker"
	"yadro.com/course/update/adapters/db"
	updategrpc "yadro.com/course/update/adapters/grpc"
	"yadro.com/course/update/adapters/words"
	"yadro.com/course/update/adapters/xkcd"
	"yadro.com/course/update/config"
	"yadro.com/course/update/core"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()
	cfg := config.MustLoad(configPath)

	log := mustMakeLogger(cfg.LogLevel)

	if err := run(cfg, log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, log *slog.Logger) error {
	log.Info("starting server")
	log.Debug("debug messages are enabled")

	storage, err := db.New(log, cfg.DBAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to db: %v", err)
	}
	if err := storage.Migrate(); err != nil {
		return fmt.Errorf("failed to migrate db: %v", err)
	}

	natsPublisher, err := broker.NewNatsPublisher(cfg.BrokerAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to nats: %v", err)
	}

	xkcd, err := xkcd.NewClient(cfg.XKCD.URL, cfg.XKCD.Timeout, log)
	if err != nil {
		return fmt.Errorf("failed create XKCD client: %v", err)
	}

	words, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed create Words client: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	updater, err := core.NewService(ctx, log, storage, xkcd, words, cfg.XKCD.Concurrency, natsPublisher)
	if err != nil {
		return fmt.Errorf("failed create Update service: %v", err)
	}

	log.Info("triggering initial update on startup")
	if err := updater.Update(ctx); err != nil {
		log.Warn("initial update failed to start", "error", err)
	}

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.Info("running scheduled 24h update")
				if err := updater.Update(ctx); err != nil {
					log.Warn("scheduled update failed to start", "error", err)
				}
			case <-ctx.Done():
				log.Info("stopping scheduled update ticker")
				return
			}
		}
	}()

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	updatepb.RegisterUpdateServer(s, updategrpc.NewServer(updater))
	reflection.Register(s)

	go func() {
		<-ctx.Done()
		log.Info("shutting down gRPC server...")
		s.GracefulStop()
	}()
	log.Info("server started", "addr", cfg.Address)
	if err := s.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("failed to serve: %w", err)
	}

	log.Info("closing database and clients...")

	natsPublisher.Close()

	if err := words.Close(); err != nil {
		log.Error("failed to close words client", "error", err)
	}

	if err := storage.Close(); err != nil {
		log.Error("failed to close storage", "error", err)
	}

	log.Info("shutdown complete")
	return nil
}

func mustMakeLogger(logLevel string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "DEBUG":
		level = slog.LevelDebug
	case "INFO":
		level = slog.LevelInfo
	case "ERROR":
		level = slog.LevelError
	default:
		panic("unknown log level: " + logLevel)
	}
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
