package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codecflow/fabric/pkg/gateway"
	"github.com/spf13/cobra"
)

var (
	port int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "fabric-gateway",
		Short: "Fabric Gateway - Control plane for worker orchestration",
		Long:  "Gateway server that manages workers and schedules container execution",
		RunE:  runGateway,
	}

	rootCmd.Flags().IntVar(&port, "port", 8080, "Port to listen on")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runGateway(cmd *cobra.Command, args []string) error {
	// Create gateway server
	server := gateway.NewServer(port)

	// Channel to listen for interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			fmt.Printf("Server error: %v\n", err)
		}
	}()

	// Wait for interrupt signal
	<-sigChan
	fmt.Println("\n🛑 Received shutdown signal")

	// Graceful shutdown with 10 second timeout
	server.GracefulShutdown(10 * time.Second)
	fmt.Println("✅ Gateway server stopped")

	return nil
}
