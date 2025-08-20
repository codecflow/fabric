package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/codecflow/fabric/pkg/placement"
	"github.com/codecflow/fabric/pkg/types"
	"github.com/codecflow/fabric/pkg/worker"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

var (
	port       int
	gatewayURL string
	workerID   string
	provider   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "fabric-worker",
		Short: "Fabric Worker - Container execution server",
		Long:  "Worker server that executes containers and provides HTTP API for container management",
		RunE:  runWorker,
	}

	rootCmd.Flags().IntVar(&port, "port", 8081, "Port to listen on")
	rootCmd.Flags().StringVar(&gatewayURL, "gateway", "", "Gateway URL to register with (optional)")
	rootCmd.Flags().StringVar(&workerID, "worker-id", "", "Worker ID (auto-generated if not provided)")
	rootCmd.Flags().StringVar(&provider, "provider", "docker", "Provider backend (docker or kubernetes)")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runWorker(cmd *cobra.Command, args []string) error {
	// Generate worker ID if not provided
	if workerID == "" {
		workerID = fmt.Sprintf("worker-%s", uuid.New().String()[:8])
	}

	fmt.Printf("🚀 Starting Fabric Worker\n")
	fmt.Printf("   Provider: %s\n", provider)
	fmt.Printf("   Port: %d\n", port)
	fmt.Printf("   Worker ID: %s\n", workerID)

	// Validate provider (placement strategies exist but not integrated yet)
	switch provider {
	case "docker":
		fmt.Println("✅ Using Docker provider")
		// Check if docker strategy can be created
		if _, err := placement.NewDockerStrategy(); err != nil {
			return fmt.Errorf("failed to initialize docker strategy: %w", err)
		}
	case "kubernetes", "k8s":
		fmt.Println("✅ Using Kubernetes provider")
		// Check if kubernetes strategy can be created
		if _, err := placement.NewKubernetesStrategy("default"); err != nil {
			return fmt.Errorf("failed to initialize kubernetes strategy: %w", err)
		}
	default:
		return fmt.Errorf("unknown provider: %s (supported: docker, kubernetes)", provider)
	}

	// Create DockerRunner for now (TODO: integrate placement.Strategy)
	runner, err := container.NewDockerRunner()
	if err != nil {
		return fmt.Errorf("failed to create docker runner: %w", err)
	}
	defer runner.Close()

	// Create worker server
	server := worker.NewServer(runner, port)

	// Register with gateway if provided
	if gatewayURL != "" {
		workerAddress := fmt.Sprintf("http://localhost:%d", port)
		if err := registerWithGateway(gatewayURL, workerID, workerAddress); err != nil {
			fmt.Printf("⚠️  Failed to register with gateway: %v\n", err)
			fmt.Println("⚠️  Continuing without gateway registration...")
		}
	}

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
	fmt.Println("✅ Worker server stopped")

	return nil
}

func registerWithGateway(gatewayURL, workerID, workerAddress string) error {
	// Get system capabilities
	numCPU := runtime.NumCPU()
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)

	req := types.RegisterWorkerRequest{
		ID:      workerID,
		Address: workerAddress,
		Capabilities: types.Capabilities{
			CPU:    numCPU,
			Memory: int64(memStats.Sys),
		},
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/workers/register", gatewayURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("gateway returned error status: %d", resp.StatusCode)
	}

	var response types.RegisterWorkerResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("✅ Registered with gateway: %s\n", response.Message)
	return nil
}
