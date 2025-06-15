package main

import (
	"context"
	"fabric/internal/config"
	"fabric/internal/grpc"
	"fabric/internal/providers/kubernetes"
	"fabric/internal/proxy"
	"fabric/internal/repository/postgres"
	"fabric/internal/state"
	"fabric/internal/stream/nats"
	"fmt"
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

	// Initialize repository
	if cfg.Database.Host != "" {
		connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.Username,
			cfg.Database.Password, cfg.Database.Database, cfg.Database.SSLMode)

		repo, err := postgres.New(connStr)
		if err != nil {
			logger.Warnf("Failed to initialize PostgreSQL repository: %v", err)
		} else {
			appState.Repository = repo
			logger.Info("PostgreSQL repository initialized")
		}
	}

	// Initialize NATS stream
	if cfg.NATS.URL != "" {
		stream, err := nats.New(cfg.NATS.URL)
		if err != nil {
			logger.Warnf("Failed to initialize NATS stream: %v", err)
		} else {
			appState.Stream = stream
			logger.Info("NATS stream initialized")
		}
	}

	// Initialize providers from config
	if cfg.Providers.Kubernetes.Enabled {
		k8sConfig := &kubernetes.Config{
			Kubeconfig: cfg.Providers.Kubernetes.Kubeconfig,
			InCluster:  false,
			Namespace:  "default",
		}

		k8sProvider, err := kubernetes.New("kubernetes", k8sConfig)
		if err != nil {
			logger.Warnf("Failed to initialize Kubernetes provider: %v", err)
		} else {
			appState.Providers["kubernetes"] = k8sProvider
			logger.Info("Kubernetes provider initialized")
		}
	}

	// Initialize scheduler with providers
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

	// Gracefully stop gRPC server
	grpcServer.Stop()

	// Stop proxy server if running
	if appState.Proxy != nil {
		if err := appState.Proxy.Stop(); err != nil {
			logger.Warnf("Error stopping proxy server: %v", err)
		}
	}

	// Close application state (repositories, streams, etc.)
	appState.Close()

	logger.Info("Server exited gracefully")
}
