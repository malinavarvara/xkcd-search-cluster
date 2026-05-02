package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"yadro.com/course/words/config"

	wordspb "yadro.com/course/proto/words"
	wordgrpc "yadro.com/course/words/adapters/grpc"
	"yadro.com/course/words/adapters/stemming"
	"yadro.com/course/words/adapters/stopwords"
	"yadro.com/course/words/core"
)

func unaryLogInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	slog.Info("gRPC call started", "method", info.FullMethod)
	resp, err := handler(ctx, req)
	if err != nil {
		slog.Error("gRPC call failed", "method", info.FullMethod, "error", err)
	} else {
		slog.Info("gRPC call finished", "method", info.FullMethod)
	}
	return resp, err
}

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "config.yaml", "path to configuration file")
	flag.Parse()
	cfg := config.MustLoad(configPath)

	listener, err := net.Listen("tcp", cfg.Address)
	if err != nil {
		slog.Error("failed to listen", "addr", cfg.Address, "error", err)
		os.Exit(1)
	}

	stemmer := stemming.NewSnowballStemmer()
	stopWordChecker := stopwords.NewEnglishStopWordChecker()
	normalizer := core.NewNormalizer(stemmer, stopWordChecker, slog.Default())

	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 5 * time.Minute,
			MaxConnectionAge:  20 * time.Minute,
			Time:              1 * time.Minute,
			Timeout:           20 * time.Second,
		}),
		grpc.ChainUnaryInterceptor(recovery.UnaryServerInterceptor(), unaryLogInterceptor),
	)
	wordspb.RegisterWordsServer(grpcServer, wordgrpc.NewServer(normalizer))
	reflection.Register(grpcServer)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-stop
		slog.Info("shutting down server gracefully...")
		grpcServer.GracefulStop()
	}()

	slog.Info("server starting", "address", cfg.Address)
	if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		slog.Error("failed to serve", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped completely")
}
