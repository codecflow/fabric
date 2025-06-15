package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gauge/internal/gauge"
)

func main() {
	var (
		host = flag.String("host", "0.0.0.0", "Host to bind to")
		port = flag.Int("port", 9090, "Port to bind to")
	)
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create gauge server
	server, err := gauge.New(*host, *port)
	if err != nil {
		log.Fatalf("Failed to create gauge server: %v", err)
	}

	// Start server
	if err := server.Start(ctx); err != nil {
		log.Fatalf("Failed to start gauge server: %v", err)
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down gauge server...")
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}
}
