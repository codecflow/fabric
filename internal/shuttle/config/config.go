package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config defines the configuration for Shuttle
type Config struct {
	// Node configuration
	Node NodeConfig `yaml:"node"`

	// Weaver connection
	Weaver WeaverConfig `yaml:"weaver"`

	// Tailscale configuration
	Tailscale TailscaleConfig `yaml:"tailscale"`

	// Container runtime configuration
	Runtime RuntimeConfig `yaml:"runtime"`

	// Metrics and monitoring
	Metrics MetricsConfig `yaml:"metrics"`

	// Logging configuration
	Logging LoggingConfig `yaml:"logging"`
}

// NodeConfig defines node-specific configuration
type NodeConfig struct {
	ID       string            `yaml:"id"`
	Name     string            `yaml:"name"`
	Region   string            `yaml:"region"`
	Zone     string            `yaml:"zone"`
	Labels   map[string]string `yaml:"labels"`
	Capacity ResourceCapacity  `yaml:"capacity"`
}

// ResourceCapacity defines the node's resource capacity
type ResourceCapacity struct {
	CPU    string `yaml:"cpu"`    // e.g. "4" or "4000m"
	Memory string `yaml:"memory"` // e.g. "8Gi"
	GPU    string `yaml:"gpu"`    // e.g. "1"
	Disk   string `yaml:"disk"`   // e.g. "100Gi"
}

// WeaverConfig defines connection to Weaver control plane
type WeaverConfig struct {
	Endpoint string        `yaml:"endpoint"`
	TLS      TLSConfig     `yaml:"tls"`
	Timeout  time.Duration `yaml:"timeout"`
	Retry    RetryConfig   `yaml:"retry"`
}

// TLSConfig defines TLS configuration
type TLSConfig struct {
	Enabled    bool   `yaml:"enabled"`
	CertFile   string `yaml:"certFile"`
	KeyFile    string `yaml:"keyFile"`
	CAFile     string `yaml:"caFile"`
	ServerName string `yaml:"serverName"`
}

// RetryConfig defines retry configuration
type RetryConfig struct {
	MaxAttempts int           `yaml:"maxAttempts"`
	Backoff     time.Duration `yaml:"backoff"`
}

// TailscaleConfig defines Tailscale mesh configuration
type TailscaleConfig struct {
	Enabled   bool   `yaml:"enabled"`
	AuthKey   string `yaml:"authKey"`
	Hostname  string `yaml:"hostname"`
	Tags      string `yaml:"tags"`
	StateDir  string `yaml:"stateDir"`
	LogLevel  string `yaml:"logLevel"`
	ExitNode  bool   `yaml:"exitNode"`
	AcceptDNS bool   `yaml:"acceptDNS"`
}

// RuntimeConfig defines container runtime configuration
type RuntimeConfig struct {
	Type       string        `yaml:"type"` // "containerd", "firecracker", "kata"
	Socket     string        `yaml:"socket"`
	Namespace  string        `yaml:"namespace"`
	DataRoot   string        `yaml:"dataRoot"`
	StateRoot  string        `yaml:"stateRoot"`
	PluginDir  string        `yaml:"pluginDir"`
	MaxProcs   int           `yaml:"maxProcs"`
	GCInterval time.Duration `yaml:"gcInterval"`
}

// MetricsConfig defines metrics configuration
type MetricsConfig struct {
	Enabled   bool            `yaml:"enabled"`
	Port      int             `yaml:"port"`
	Path      string          `yaml:"path"`
	Interval  time.Duration   `yaml:"interval"`
	OpenMeter OpenMeterConfig `yaml:"openMeter"`
}

// OpenMeterConfig defines OpenMeter integration
type OpenMeterConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Endpoint string `yaml:"endpoint"`
	APIKey   string `yaml:"apiKey"`
	MeterID  string `yaml:"meterID"`
}

// LoggingConfig defines logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"` // "json" or "text"
	Output string `yaml:"output"` // "stdout", "stderr", or file path
}

// Load loads configuration from file
func Load(path string) (*Config, error) {
	// Set defaults
	cfg := &Config{
		Node: NodeConfig{
			ID:     generateNodeID(),
			Name:   getHostname(),
			Region: "unknown",
			Zone:   "unknown",
			Labels: make(map[string]string),
			Capacity: ResourceCapacity{
				CPU:    "1",
				Memory: "1Gi",
				GPU:    "0",
				Disk:   "10Gi",
			},
		},
		Weaver: WeaverConfig{
			Endpoint: "localhost:8080",
			Timeout:  30 * time.Second,
			Retry: RetryConfig{
				MaxAttempts: 3,
				Backoff:     time.Second,
			},
		},
		Tailscale: TailscaleConfig{
			Enabled:   true,
			StateDir:  "/var/lib/tailscale",
			LogLevel:  "info",
			AcceptDNS: true,
		},
		Runtime: RuntimeConfig{
			Type:       "containerd",
			Socket:     "/run/containerd/containerd.sock",
			Namespace:  "fabric",
			DataRoot:   "/var/lib/containerd",
			StateRoot:  "/run/containerd",
			MaxProcs:   0, // Use all available
			GCInterval: 5 * time.Minute,
		},
		Metrics: MetricsConfig{
			Enabled:  true,
			Port:     9090,
			Path:     "/metrics",
			Interval: 30 * time.Second,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stdout",
		},
	}

	// Load from file if it exists
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	// Override with environment variables
	if err := cfg.loadFromEnv(); err != nil {
		return nil, fmt.Errorf("failed to load environment variables: %w", err)
	}

	// Validate configuration
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadFromEnv loads configuration from environment variables
func (c *Config) loadFromEnv() error {
	if nodeID := os.Getenv("SHUTTLE_NODE_ID"); nodeID != "" {
		c.Node.ID = nodeID
	}
	if nodeName := os.Getenv("SHUTTLE_NODE_NAME"); nodeName != "" {
		c.Node.Name = nodeName
	}
	if region := os.Getenv("SHUTTLE_REGION"); region != "" {
		c.Node.Region = region
	}
	if zone := os.Getenv("SHUTTLE_ZONE"); zone != "" {
		c.Node.Zone = zone
	}
	if endpoint := os.Getenv("WEAVER_ENDPOINT"); endpoint != "" {
		c.Weaver.Endpoint = endpoint
	}
	if authKey := os.Getenv("TAILSCALE_AUTH_KEY"); authKey != "" {
		c.Tailscale.AuthKey = authKey
	}
	if socket := os.Getenv("CONTAINERD_SOCKET"); socket != "" {
		c.Runtime.Socket = socket
	}

	return nil
}

// validate validates the configuration
func (c *Config) validate() error {
	if c.Node.ID == "" {
		return fmt.Errorf("node ID is required")
	}
	if c.Node.Name == "" {
		return fmt.Errorf("node name is required")
	}
	if c.Weaver.Endpoint == "" {
		return fmt.Errorf("weaver endpoint is required")
	}
	if c.Runtime.Socket == "" {
		return fmt.Errorf("runtime socket is required")
	}

	return nil
}

// generateNodeID generates a unique node ID
func generateNodeID() string {
	hostname, _ := os.Hostname()
	return fmt.Sprintf("shuttle-%s", hostname)
}

// getHostname returns the system hostname
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}
