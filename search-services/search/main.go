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

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"google.golang.org/grpc"

	searchpb "yadro.com/course/proto/search"
	"yadro.com/course/search/adapters/broker"
	"yadro.com/course/search/adapters/db"
	searchgrpc "yadro.com/course/search/adapters/grpc"
	"yadro.com/course/search/adapters/words"
	"yadro.com/course/search/config"
	"yadro.com/course/search/core"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "server configuration file")
	flag.Parse()

	cfg := config.MustLoad(configPath)

	log := mustMakeLogger(cfg.LogLevel, cfg.LogFormat)

	if err := run(cfg, log); err != nil {
		log.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func run(cfg *config.Config, log *slog.Logger) error {
	log.Info("starting search service", "address", cfg.Address, "db", cfg.DBAddress)

	dbConn, err := sqlx.Connect("postgres", cfg.DBAddress)
	if err != nil {
		return fmt.Errorf("failed to connect to db: %w", err)
	}
	dbConn.SetMaxOpenConns(20)
	dbConn.SetMaxIdleConns(20)
	dbConn.SetConnMaxLifetime(time.Minute * 5)
	defer func() {
		if err := dbConn.Close(); err != nil {
			log.Error("failed to close db connection", "error", err)
		}
	}()
	log.Info("database connected")

	wordsClient, err := words.NewClient(cfg.WordsAddress, log)
	if err != nil {
		return fmt.Errorf("failed to create words client: %w", err)
	}
	defer func() {
		if err := wordsClient.Close(); err != nil {
			log.Error("failed to close words client", "error", err)
		}
	}()
	log.Info("words client created", "addr", cfg.WordsAddress)

	comicsRepo := db.NewRepository(dbConn, log)
	searcher := core.NewService(wordsClient, comicsRepo, log, cfg.IndexTTL)

	natsSub, err := broker.NewSubscriber(cfg.BrokerAddress, log)
	if err != nil {
		return fmt.Errorf("failed to connect to nats: %w", err)
	}
	defer func() {
		if err := natsSub.Close(); err != nil {
			log.Error("failed to close nats subscriber", "error", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	err = natsSub.Subscribe(ctx, "xkcd.db.updated", func() {
		searcher.TriggerRebuild()
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to nats: %w", err)
	}

	go searcher.RunBackgroundIndexer(ctx, cfg.IndexRebuildInterval)

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	grpcServer := grpc.NewServer()
	searchpb.RegisterSearchServer(grpcServer, searchgrpc.NewServer(searcher, log))
	go func() {
		log.Info("server started", "addr", cfg.Address)
		if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Error("failed to serve", "error", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down gracefully...")

	grpcServer.GracefulStop()
	log.Info("gRPC server stopped")

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
