package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"shuttle/internal/config"
	"shuttle/internal/shuttle"
)

func main() {
	var configPath = flag.String("config", "/etc/shuttle/config.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Create shuttle instance
	s, err := shuttle.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create shuttle: %v", err)
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Received shutdown signal, gracefully shutting down...")
		cancel()
	}()

	// Start shuttle
	log.Printf("Starting Shuttle node runner (version: %s)", getVersion())
	if err := s.Run(ctx); err != nil {
		log.Fatalf("Shuttle failed: %v", err)
	}

	log.Println("Shuttle shutdown complete")
}

func getVersion() string {
	// In a real implementation, this would be set during build
	return "dev"
}
