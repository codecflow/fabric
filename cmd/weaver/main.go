package main

import (
	"context"
	"fabric/internal/api"
	"fabric/internal/config"
	"fabric/internal/state"
	"net/http"
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

	defer appState.Close()

	router := api.SetupRoutes(appState)

	server := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Infof("Starting Weaver server on %s", cfg.Server.Address)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited")
}
