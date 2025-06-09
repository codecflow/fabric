package config

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	NATS      NATSConfig      `yaml:"nats"`
	Providers ProvidersConfig `yaml:"providers"`
}

type ServerConfig struct {
	Address string `yaml:"address" kong:"help='Server address',default=':8080'"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host" kong:"help='Database host',default='localhost'"`
	Port     int    `yaml:"port" kong:"help='Database port',default='5432'"`
	User     string `yaml:"user" kong:"help='Database user',default='fabric'"`
	Password string `yaml:"password" kong:"help='Database password',default='fabric'"`
	Database string `yaml:"database" kong:"help='Database name',default='fabric'"`
	SSLMode  string `yaml:"ssl_mode" kong:"help='Database SSL mode',default='disable'"`
}

type NATSConfig struct {
	URL string `yaml:"url" kong:"help='NATS URL',default='nats://localhost:4222'"`
}

type ProvidersConfig struct {
	K8s       K8sConfig       `yaml:"k8s"`
	RunPod    RunPodConfig    `yaml:"runpod"`
	CoreWeave CoreWeaveConfig `yaml:"coreweave"`
}

type K8sConfig struct {
	Enabled    bool   `yaml:"enabled" kong:"help='Enable K8s provider',default='true'"`
	Kubeconfig string `yaml:"kubeconfig" kong:"help='Kubeconfig path'"`
	Namespace  string `yaml:"namespace" kong:"help='K8s namespace',default='default'"`
}

type RunPodConfig struct {
	Enabled bool   `yaml:"enabled" kong:"help='Enable RunPod provider',default='false'"`
	APIKey  string `yaml:"api_key" kong:"help='RunPod API key'"`
}

type CoreWeaveConfig struct {
	Enabled bool   `yaml:"enabled" kong:"help='Enable CoreWeave provider',default='false'"`
	APIKey  string `yaml:"api_key" kong:"help='CoreWeave API key'"`
}

type CLI struct {
	Config   string `kong:"help='Config file path',default='config.yaml'"`
	LogLevel string `kong:"help='Log level',default='info',enum='debug,info,warn,error'"`

	// Server config
	Address string `kong:"help='Server address',default=':8080'"`

	// Database config
	DBHost     string `kong:"help='Database host',default='localhost'"`
	DBPort     int    `kong:"help='Database port',default='5432'"`
	DBUser     string `kong:"help='Database user',default='fabric'"`
	DBPassword string `kong:"help='Database password',default='fabric'"`
	DBName     string `kong:"help='Database name',default='fabric'"`
	DBSSLMode  string `kong:"help='Database SSL mode',default='disable'"`

	// NATS config
	NATSURL string `kong:"help='NATS URL',default='nats://localhost:4222'"`

	// Provider config
	K8sEnabled       bool   `kong:"help='Enable K8s provider',default='true'"`
	K8sKubeconfig    string `kong:"help='Kubeconfig path'"`
	K8sNamespace     string `kong:"help='K8s namespace',default='default'"`
	RunPodEnabled    bool   `kong:"help='Enable RunPod provider',default='false'"`
	RunPodAPIKey     string `kong:"help='RunPod API key'"`
	CoreWeaveEnabled bool   `kong:"help='Enable CoreWeave provider',default='false'"`
	CoreWeaveAPIKey  string `kong:"help='CoreWeave API key'"`
}

func Load() (*Config, error) {
	var cli CLI
	ctx := kong.Parse(&cli)

	config := &Config{
		Server: ServerConfig{
			Address: cli.Address,
		},
		Database: DatabaseConfig{
			Host:     cli.DBHost,
			Port:     cli.DBPort,
			User:     cli.DBUser,
			Password: cli.DBPassword,
			Database: cli.DBName,
			SSLMode:  cli.DBSSLMode,
		},
		NATS: NATSConfig{
			URL: cli.NATSURL,
		},
		Providers: ProvidersConfig{
			K8s: K8sConfig{
				Enabled:    cli.K8sEnabled,
				Kubeconfig: cli.K8sKubeconfig,
				Namespace:  cli.K8sNamespace,
			},
			RunPod: RunPodConfig{
				Enabled: cli.RunPodEnabled,
				APIKey:  cli.RunPodAPIKey,
			},
			CoreWeave: CoreWeaveConfig{
				Enabled: cli.CoreWeaveEnabled,
				APIKey:  cli.CoreWeaveAPIKey,
			},
		},
	}

	// Try to load config file if it exists
	if cli.Config != "" {
		if _, err := os.Stat(cli.Config); err == nil {
			data, err := os.ReadFile(cli.Config)
			if err != nil {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}

			var fileConfig Config
			if err := yaml.Unmarshal(data, &fileConfig); err != nil {
				return nil, fmt.Errorf("failed to parse config file: %w", err)
			}

			// Merge file config with CLI config (CLI takes precedence)
			mergeConfigs(config, &fileConfig)
		}
	}

	// Override with environment variables
	overrideWithEnv(config)

	_ = ctx // Satisfy linter

	return config, nil
}

func mergeConfigs(cli, file *Config) {
	// Only use file values if CLI values are defaults
	if cli.Server.Address == ":8080" && file.Server.Address != "" {
		cli.Server.Address = file.Server.Address
	}
	if cli.Database.Host == "localhost" && file.Database.Host != "" {
		cli.Database.Host = file.Database.Host
	}
	// Add more field merging as needed
}

func overrideWithEnv(config *Config) {
	if addr := os.Getenv("WEAVER_ADDRESS"); addr != "" {
		config.Server.Address = addr
	}
	if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
		config.Database.Host = dbHost
	}
	if dbUser := os.Getenv("DB_USER"); dbUser != "" {
		config.Database.User = dbUser
	}
	if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
		config.Database.Password = dbPass
	}
	if dbName := os.Getenv("DB_NAME"); dbName != "" {
		config.Database.Database = dbName
	}
	if natsURL := os.Getenv("NATS_URL"); natsURL != "" {
		config.NATS.URL = natsURL
	}
}
