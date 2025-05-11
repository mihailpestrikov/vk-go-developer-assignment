package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"

	"vk-go-developer-assignment/internal/server"
	"vk-go-developer-assignment/pkg/config"
	"vk-go-developer-assignment/pkg/logger"
	"vk-go-developer-assignment/pkg/subpub"
)

func main() {
	configPath := flag.String("config", "config/config.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	zlog := logger.SetupLogger(cfg.Log)
	zlog.Info().Msg("Starting subscription service")

	var dropStrategy subpub.DropStrategy
	switch cfg.SubPub.DropStrategy {
	case "none":
		dropStrategy = subpub.DropNone
	case "oldest":
		dropStrategy = subpub.DropOldest
	case "newest":
		dropStrategy = subpub.DropNewest
	default:
		dropStrategy = subpub.DropNone
	}

	sp := subpub.NewSubPub(
		subpub.WithBufferSize(cfg.SubPub.BufferSize),
		subpub.WithDropStrategy(dropStrategy),
		subpub.WithWorkerPool(cfg.SubPub.WorkerPoolSize),
	)

	grpcServer, err := server.NewGRPCServer(cfg, zlog)
	if err != nil {
		zlog.Fatal().Err(err).Msg("Failed to create gRPC server")
	}

	grpcServer.RegisterPubSubService(sp)

	go func() {
		if err := grpcServer.Start(); err != nil {
			zlog.Fatal().Err(err).Msg("Failed to start gRPC server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	zlog.Info().Msg("Shutting down server...")

	grpcServer.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sp.Close(ctx); err != nil {
		zlog.Error().Err(err).Msg("Error closing SubPub")
	}

	zlog.Info().Msg("Server shutdown completed")
}
