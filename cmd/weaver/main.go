package main

import (
	"context"
	"fabric/internal/config"
	"fabric/internal/grpc"
	"fabric/internal/proxy"
	"fabric/internal/state"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fabric/internal/scheduler/simple"

	"github.com/sirupsen/logrus"
)

func main() {
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	logger.SetLevel(logrus.InfoLevel)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	appState := state.New()
	// TODO: Initialize appState components (Repository, Stream, Meter, etc.)

	// Initialize scheduler with providers
	// TODO: Load providers from config
	// For now, create a simple scheduler with empty providers
	scheduler := simple.New(appState.Providers, nil)
	appState.Scheduler = scheduler

	// Initialize proxy server
	if cfg.Proxy.Enabled {
		var err error
		appState.Proxy, err = proxy.New(&cfg.Proxy)
		if err != nil {
			logger.Fatalf("Failed to create proxy server: %v", err)
		}

		ctx := context.Background()
		if err := appState.Proxy.Start(ctx); err != nil {
			logger.Fatalf("Failed to start proxy server: %v", err)
		}
		logger.Infof("Proxy server started on port %d", cfg.Proxy.Port)
	}

	defer appState.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer(appState, logger)

	// Start gRPC server in a goroutine
	go func() {
		logger.Infof("Starting Weaver gRPC server on %s", cfg.Server.Address)
		if err := grpcServer.Start(cfg.Server.Address); err != nil {
			logger.Fatalf("gRPC server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// TODO: Implement graceful shutdown for gRPC server
	// For now, just exit
	logger.Info("Server exited")
}
