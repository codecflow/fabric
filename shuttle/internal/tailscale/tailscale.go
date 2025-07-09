package tailscale

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/codecflow/fabric/shuttle/internal/config"
)

// Client manages Tailscale connectivity
type Client struct {
	config *config.TailscaleConfig
	cmd    *exec.Cmd
}

// New creates a new Tailscale client
func New(cfg *config.TailscaleConfig) (*Client, error) {
	if !cfg.Enabled {
		return nil, fmt.Errorf("Tailscale is disabled") // nolint:staticcheck
	}

	return &Client{
		config: cfg,
	}, nil
}

// Start starts the Tailscale daemon
func (c *Client) Start(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	log.Println("Starting Tailscale daemon...")

	// Check if tailscaled is already running
	if c.isRunning() {
		log.Println("Tailscale daemon already running")
		return c.authenticate(ctx)
	}

	// Start tailscaled daemon
	args := []string{
		"--state=" + c.config.StateDir,
		"--socket=/var/run/tailscale/tailscaled.sock",
	}

	if c.config.LogLevel != "" {
		args = append(args, "--verbose="+c.config.LogLevel)
	}

	c.cmd = exec.CommandContext(ctx, "tailscaled", args...) // nolint:gosec
	c.cmd.Stdout = os.Stdout
	c.cmd.Stderr = os.Stderr

	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start tailscaled: %w", err)
	}

	// Wait for daemon to be ready
	if err := c.waitForReady(ctx); err != nil {
		return fmt.Errorf("tailscaled failed to become ready: %w", err)
	}

	// Authenticate with Tailscale
	if err := c.authenticate(ctx); err != nil {
		return fmt.Errorf("failed to authenticate with Tailscale: %w", err)
	}

	log.Println("Tailscale started successfully")
	return nil
}

// Stop stops the Tailscale daemon
func (c *Client) Stop() error {
	if c.cmd != nil && c.cmd.Process != nil {
		log.Println("Stopping Tailscale daemon...")
		if err := c.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to stop tailscaled: %w", err)
		}
		if err := c.cmd.Wait(); err != nil {
			log.Printf("Error waiting for tailscaled to stop: %v", err)
		}
	}
	return nil
}

// isRunning checks if tailscaled is already running
func (c *Client) isRunning() bool {
	cmd := exec.Command("tailscale", "status")
	return cmd.Run() == nil
}

// waitForReady waits for tailscaled to be ready
func (c *Client) waitForReady(ctx context.Context) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for tailscaled to be ready")
		case <-ticker.C:
			if c.isRunning() {
				return nil
			}
		}
	}
}

// authenticate authenticates with Tailscale using auth key
func (c *Client) authenticate(ctx context.Context) error {
	if c.config.AuthKey == "" {
		log.Println("No auth key provided, assuming already authenticated")
		return nil
	}

	log.Println("Authenticating with Tailscale...")

	args := []string{"up", "--authkey=" + c.config.AuthKey}

	if c.config.Hostname != "" {
		args = append(args, "--hostname="+c.config.Hostname)
	}

	if c.config.Tags != "" {
		args = append(args, "--advertise-tags="+c.config.Tags)
	}

	if c.config.ExitNode {
		args = append(args, "--advertise-exit-node")
	}

	if c.config.AcceptDNS {
		args = append(args, "--accept-dns")
	}

	cmd := exec.CommandContext(ctx, "tailscale", args...) // nolint:gosec
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	log.Println("Tailscale authentication successful")
	return nil
}

// GetStatus returns the current Tailscale status
func (c *Client) GetStatus(ctx context.Context) (*Status, error) {
	cmd := exec.CommandContext(ctx, "tailscale", "status", "--json")
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	// In a real implementation, parse the JSON output
	return &Status{
		Connected: true,
		IP:        "100.64.0.1", // Placeholder
	}, nil
}

// Status represents Tailscale connection status
type Status struct {
	Connected bool   `json:"connected"`
	IP        string `json:"ip"`
	Hostname  string `json:"hostname"`
}
