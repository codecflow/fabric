package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/codecflow/fabric/pkg/types"
	"github.com/spf13/cobra"
)

var (
	// Global flags
	gateway string

	// Flags for run command
	image   string
	command string
	env     []string
	workdir string
	gpu     int
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "fabric",
		Short: "Fabric - Simple container orchestration",
		Long:  "Fabric is a cloud GPU compute orchestration platform for AI/ML workloads",
	}

	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run a container locally",
		Long:  "Execute a container with the specified image and command",
		RunE:  runContainer,
	}

	runCmd.Flags().StringVar(&gateway, "gateway", "", "Gateway URL (if not provided, runs locally)")
	runCmd.Flags().StringVar(&image, "image", "", "Container image to run (required)")
	runCmd.Flags().StringVar(&command, "command", "", "Command to execute in the container (required)")
	runCmd.Flags().StringArrayVar(&env, "env", []string{}, "Environment variables (format: KEY=VALUE)")
	runCmd.Flags().StringVar(&workdir, "workdir", "", "Working directory inside the container")
	runCmd.Flags().IntVar(&gpu, "gpu", 0, "Number of GPUs to allocate")

	runCmd.MarkFlagRequired("image")
	runCmd.MarkFlagRequired("command")

	rootCmd.AddCommand(runCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runContainer(cmd *cobra.Command, args []string) error {
	// Parse environment variables
	envMap := make(map[string]string)
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	// If gateway is provided, use remote execution
	if gateway != "" {
		return runRemote(envMap)
	}

	// Otherwise run locally
	return runLocal(envMap)
}

func runLocal(envMap map[string]string) error {
	// Create Docker runner
	runner, err := container.NewDockerRunner()
	if err != nil {
		return fmt.Errorf("failed to create docker runner: %w", err)
	}
	defer runner.Close()

	// Parse command (handle shell commands)
	cmdParts := []string{"sh", "-c", command}

	// Create container spec
	spec := &container.Spec{
		Image:   image,
		Command: cmdParts,
		Env:     envMap,
		WorkDir: workdir,
		GPU:     gpu,
	}

	fmt.Printf("🚀 Running container locally: %s\n", image)
	fmt.Printf("📝 Command: %s\n", command)
	if len(envMap) > 0 {
		fmt.Printf("🔧 Environment: %v\n", envMap)
	}
	if gpu > 0 {
		fmt.Printf("🎮 GPU: %d\n", gpu)
	}
	fmt.Println()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run the container
	result, err := runner.Run(ctx, spec)
	if err != nil {
		return fmt.Errorf("failed to run container: %w", err)
	}

	// Display output
	if result.Stdout != "" {
		fmt.Print(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Fprint(os.Stderr, result.Stderr)
	}

	// Display exit code if non-zero
	if result.ExitCode != 0 {
		fmt.Printf("\n❌ Container exited with code: %d\n", result.ExitCode)
		os.Exit(result.ExitCode)
	} else {
		fmt.Printf("\n✅ Container completed successfully\n")
	}

	return nil
}

func runRemote(envMap map[string]string) error {
	fmt.Printf("🌐 Submitting to gateway: %s\n", gateway)
	fmt.Printf("🚀 Container: %s\n", image)
	fmt.Printf("📝 Command: %s\n", command)
	if gpu > 0 {
		fmt.Printf("🎮 GPU: %d\n", gpu)
	}
	fmt.Println()

	// Parse command - gateway expects array
	cmdParts := []string{"sh", "-c", command}

	// Create request
	req := types.CreateContainerRequest{
		Image:   image,
		Command: cmdParts,
		Env:     envMap,
		WorkDir: workdir,
		GPU:     gpu,
	}

	// Marshal request
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/containers", gateway)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request to gateway: %w", err)
	}
	defer resp.Body.Close()

	// Check status
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gateway returned error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var response types.CreateContainerResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	fmt.Printf("✅ Container created: %s\n", response.ID)
	fmt.Printf("📍 Worker: %s\n", response.WorkerID)
	fmt.Printf("📊 Status: %s\n", response.Status)
	fmt.Println()
	fmt.Printf("💡 Check status with:\n")
	fmt.Printf("   curl %s/containers/%s\n", gateway, response.ID)

	return nil
}
