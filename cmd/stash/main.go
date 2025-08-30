package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/codecflow/fabric/stash/config"
	"github.com/codecflow/fabric/stash/server"
)

var (
	locality = flag.String("locality", "", "Server locality/region")
)

func main() {
	flag.Parse()

	// Load configuration
	configManager, err := config.NewManager[config.BlobCacheConfig]()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	cfg := configManager.Get()

	// Initialize logger
	logLevel := slog.LevelInfo
	if cfg.Global.DebugMode {
		logLevel = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Determine locality
	loc := *locality
	if loc == "" {
		loc = cfg.Global.GetLocality()
	}

	logger.Info("starting stash server", "locality", loc, "mode", cfg.Server.Mode)

	ctx := context.Background()

	// Create and start server
	srv, err := server.NewServer(ctx, &cfg, loc, logger)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Start server (blocks until shutdown)
	if err := srv.Start(cfg.Global.ServerPort); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
