package config

import (
	"os"
	"strconv"
)

// Config represents the application configuration
type Config struct {
	Server    ServerConfig    `json:"server"`
	Database  DatabaseConfig  `json:"database"`
	NATS      NATSConfig      `json:"nats"`
	Logging   LoggingConfig   `json:"logging"`
	Proxy     ProxyConfig     `json:"proxy"`
	Providers ProvidersConfig `json:"providers"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"sslMode"`
}

// NATSConfig represents NATS configuration
type NATSConfig struct {
	URL     string `json:"url"`
	Subject string `json:"subject"`
}

// LoggingConfig represents logging configuration
type LoggingConfig struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// ProxyConfig represents proxy configuration
type ProxyConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

// CRIUConfig represents CRIU snapshot configuration
type CRIUConfig struct {
	Enabled        bool   `json:"enabled"`
	SnapshotDir    string `json:"snapshot_dir"`
	IrohEnabled    bool   `json:"iroh_enabled"`
	TCPEstablished bool   `json:"tcp_established"`
	FileLocks      bool   `json:"file_locks"`
}

// IrohConfig represents Iroh storage configuration
type IrohConfig struct {
	Enabled bool   `json:"enabled"`
	NodeURL string `json:"node_url"`
}

// ProvidersConfig represents provider configurations
type ProvidersConfig struct {
	Kubernetes KubernetesConfig `json:"kubernetes"`
}

// KubernetesConfig represents Kubernetes provider configuration
type KubernetesConfig struct {
	Enabled    bool   `json:"enabled"`
	Kubeconfig string `json:"kubeconfig"`
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	config := &Config{
		Server: ServerConfig{
			Address: getEnv("SERVER_ADDRESS", ":8080"),
			Port:    getEnvInt("SERVER_PORT", 8080),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvInt("DB_PORT", 5432),
			Database: getEnv("DB_NAME", "fabric"),
			Username: getEnv("DB_USER", "fabric"),
			Password: getEnv("DB_PASSWORD", ""),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		NATS: NATSConfig{
			URL:     getEnv("NATS_URL", "nats://localhost:4222"),
			Subject: getEnv("NATS_SUBJECT", "fabric.events"),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		Proxy: ProxyConfig{
			Enabled: getEnv("PROXY_ENABLED", "false") == "true",
			Port:    getEnvInt("PROXY_PORT", 8081),
		},
	}

	return config, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an environment variable as int with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
