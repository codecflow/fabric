package state

import (
	"database/sql"
	"fabric/internal/config"
	"fabric/internal/provider"
	"fabric/internal/provider/coreweave"
	"fabric/internal/provider/k8s"
	"fabric/internal/provider/runpod"
	"fabric/internal/scheduler"
	"fmt"

	_ "github.com/lib/pq"
	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

type AppState struct {
	DB        *sql.DB
	NATS      *nats.Conn
	Config    *config.Config
	Logger    *logrus.Logger
	Providers map[string]provider.Provider
	Scheduler *scheduler.Scheduler
}

func New(cfg *config.Config, logger *logrus.Logger) (*AppState, error) {
	// Initialize database connection
	db, err := initDatabase(cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize NATS connection
	nc, err := nats.Connect(cfg.NATS.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Initialize providers
	providers := make(map[string]provider.Provider)

	// Initialize scheduler
	sched := scheduler.New(providers, db, nc, logger)

	state := &AppState{
		DB:        db,
		NATS:      nc,
		Config:    cfg,
		Logger:    logger,
		Providers: providers,
		Scheduler: sched,
	}

	// Initialize providers after state is created
	if err := state.initProviders(); err != nil {
		return nil, fmt.Errorf("failed to initialize providers: %w", err)
	}

	return state, nil
}

func (s *AppState) Close() error {
	if s.NATS != nil {
		s.NATS.Close()
	}
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

func initDatabase(cfg config.DatabaseConfig) (*sql.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func (s *AppState) initProviders() error {
	// Initialize K8s provider if enabled
	if s.Config.Providers.K8s.Enabled {
		k8sProvider, err := k8s.New(s.Config.Providers.K8s)
		if err != nil {
			s.Logger.Warnf("Failed to initialize K8s provider: %v", err)
		} else {
			s.Providers["k8s"] = k8sProvider
			s.Logger.Info("K8s provider initialized")
		}
	}

	// Initialize RunPod provider if enabled
	if s.Config.Providers.RunPod.Enabled {
		runpodProvider, err := runpod.New(s.Config.Providers.RunPod)
		if err != nil {
			s.Logger.Warnf("Failed to initialize RunPod provider: %v", err)
		} else {
			s.Providers["runpod"] = runpodProvider
			s.Logger.Info("RunPod provider initialized")
		}
	}

	// Initialize CoreWeave provider if enabled
	if s.Config.Providers.CoreWeave.Enabled {
		coreweaveProvider, err := coreweave.New(s.Config.Providers.CoreWeave)
		if err != nil {
			s.Logger.Warnf("Failed to initialize CoreWeave provider: %v", err)
		} else {
			s.Providers["coreweave"] = coreweaveProvider
			s.Logger.Info("CoreWeave provider initialized")
		}
	}

	return nil
}
