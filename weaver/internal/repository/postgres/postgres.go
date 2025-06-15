package postgres

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

// Repository implements all domain repository interfaces using PostgreSQL
type Repository struct {
	db        *sql.DB
	Workload  *WorkloadRepository
	Namespace *NamespaceRepository
	Secret    *SecretRepository
}

// New creates a new PostgreSQL repository
func New(connectionString string) (*Repository, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &Repository{
		db:        db,
		Workload:  NewWorkloadRepository(db),
		Namespace: NewNamespaceRepository(db),
		Secret:    NewSecretRepository(db),
	}

	// Initialize schema
	if err := repo.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return repo, nil
}

// initSchema creates the necessary tables
func (r *Repository) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS namespaces (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL UNIQUE,
		labels JSONB,
		annotations JSONB,
		spec JSONB,
		status JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS secrets (
		id VARCHAR(255) PRIMARY KEY,
		namespace_id VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		spec JSONB,
		status JSONB,
		labels JSONB,
		annotations JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		FOREIGN KEY (namespace_id) REFERENCES namespaces(name),
		UNIQUE(namespace_id, name)
	);

	CREATE TABLE IF NOT EXISTS workloads (
		id VARCHAR(255) PRIMARY KEY,
		namespace_id VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		spec JSONB NOT NULL,
		status JSONB,
		labels JSONB,
		annotations JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		FOREIGN KEY (namespace_id) REFERENCES namespaces(name),
		UNIQUE(namespace_id, name)
	);

	CREATE INDEX IF NOT EXISTS idx_workloads_namespace ON workloads(namespace_id);
	CREATE INDEX IF NOT EXISTS idx_secrets_namespace ON secrets(namespace_id);
	CREATE INDEX IF NOT EXISTS idx_namespaces_name ON namespaces(name);
	`

	_, err := r.db.Exec(schema)
	return err
}

// Health check
func (r *Repository) HealthCheck(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the database connection
func (r *Repository) Close() error {
	return r.db.Close()
}
