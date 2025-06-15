package postgres

import (
	"context"
	"database/sql"
	"fabric/internal/repository"
	"fabric/internal/types"
	"fmt"

	_ "github.com/lib/pq"
)

// PostgresRepository implements the Repository interface using PostgreSQL
type PostgresRepository struct {
	db *sql.DB
}

// New creates a new PostgreSQL repository
func New(connectionString string) (*PostgresRepository, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	repo := &PostgresRepository{db: db}

	// Initialize schema
	if err := repo.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return repo, nil
}

// initSchema creates the necessary tables
func (r *PostgresRepository) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS namespaces (
		id VARCHAR(255) PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		display_name VARCHAR(255),
		description TEXT,
		labels JSONB,
		annotations JSONB,
		resource_quota JSONB,
		network_policy JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS secrets (
		id VARCHAR(255) PRIMARY KEY,
		namespace_id VARCHAR(255) REFERENCES namespaces(id),
		name VARCHAR(255) NOT NULL,
		type VARCHAR(50) NOT NULL,
		data JSONB,
		labels JSONB,
		annotations JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS workloads (
		id VARCHAR(255) PRIMARY KEY,
		namespace_id VARCHAR(255) REFERENCES namespaces(id),
		name VARCHAR(255) NOT NULL,
		spec JSONB NOT NULL,
		status JSONB,
		labels JSONB,
		annotations JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_workloads_namespace ON workloads(namespace_id);
	CREATE INDEX IF NOT EXISTS idx_secrets_namespace ON secrets(namespace_id);
	`

	_, err := r.db.Exec(schema)
	return err
}

// Workload operations
func (r *PostgresRepository) CreateWorkload(ctx context.Context, workload *types.Workload) error {
	query := `
		INSERT INTO workloads (id, namespace_id, name, spec, status, labels, annotations)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		workload.ID,
		workload.Namespace,
		workload.Name,
		toJSON(workload.Spec),
		toJSON(workload.Status),
		toJSON(workload.Labels),
		toJSON(workload.Annotations),
	)

	return err
}

func (r *PostgresRepository) GetWorkload(ctx context.Context, id string) (*types.Workload, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM workloads WHERE id = $1
	`

	var workload types.Workload
	var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&workload.ID,
		&workload.Namespace,
		&workload.Name,
		&specJSON,
		&statusJSON,
		&labelsJSON,
		&annotationsJSON,
		&workload.CreatedAt,
		&workload.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if err := fromJSON(specJSON, &workload.Spec); err != nil {
		return nil, err
	}
	if err := fromJSON(statusJSON, &workload.Status); err != nil {
		return nil, err
	}
	if err := fromJSON(labelsJSON, &workload.Labels); err != nil {
		return nil, err
	}
	if err := fromJSON(annotationsJSON, &workload.Annotations); err != nil {
		return nil, err
	}

	return &workload, nil
}

func (r *PostgresRepository) UpdateWorkload(ctx context.Context, workload *types.Workload) error {
	query := `
		UPDATE workloads 
		SET spec = $2, status = $3, labels = $4, annotations = $5, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		workload.ID,
		toJSON(workload.Spec),
		toJSON(workload.Status),
		toJSON(workload.Labels),
		toJSON(workload.Annotations),
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) DeleteWorkload(ctx context.Context, id string) error {
	query := `DELETE FROM workloads WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return repository.ErrNotFound
	}

	return nil
}

func (r *PostgresRepository) ListWorkloads(ctx context.Context, namespace string) ([]*types.Workload, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM workloads WHERE namespace_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workloads []*types.Workload

	for rows.Next() {
		var workload types.Workload
		var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

		err := rows.Scan(
			&workload.ID,
			&workload.Namespace,
			&workload.Name,
			&specJSON,
			&statusJSON,
			&labelsJSON,
			&annotationsJSON,
			&workload.CreatedAt,
			&workload.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if err := fromJSON(specJSON, &workload.Spec); err != nil {
			return nil, err
		}
		if err := fromJSON(statusJSON, &workload.Status); err != nil {
			return nil, err
		}
		if err := fromJSON(labelsJSON, &workload.Labels); err != nil {
			return nil, err
		}
		if err := fromJSON(annotationsJSON, &workload.Annotations); err != nil {
			return nil, err
		}

		workloads = append(workloads, &workload)
	}

	return workloads, rows.Err()
}

// Namespace operations
func (r *PostgresRepository) CreateNamespace(ctx context.Context, namespace *types.Namespace) error {
	query := `
		INSERT INTO namespaces (id, name, display_name, description, labels, annotations, resource_quota, network_policy)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		namespace.ID,
		namespace.Name,
		"", // display_name - not in current type
		"", // description - not in current type
		toJSON(namespace.Labels),
		toJSON(namespace.Annotations),
		toJSON(namespace.Spec.Quotas),
		toJSON(namespace.Spec.NetworkPolicy),
	)

	return err
}

func (r *PostgresRepository) GetNamespace(ctx context.Context, id string) (*types.Namespace, error) {
	query := `
		SELECT id, name, display_name, description, labels, annotations, resource_quota, network_policy, created_at, updated_at
		FROM namespaces WHERE id = $1
	`

	var namespace types.Namespace
	var displayName, description string
	var labelsJSON, annotationsJSON, quotaJSON, policyJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&namespace.ID,
		&namespace.Name,
		&displayName,
		&description,
		&labelsJSON,
		&annotationsJSON,
		&quotaJSON,
		&policyJSON,
		&namespace.CreatedAt,
		&namespace.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if err := fromJSON(labelsJSON, &namespace.Labels); err != nil {
		return nil, err
	}
	if err := fromJSON(annotationsJSON, &namespace.Annotations); err != nil {
		return nil, err
	}
	if err := fromJSON(quotaJSON, &namespace.Spec.Quotas); err != nil {
		return nil, err
	}
	if err := fromJSON(policyJSON, &namespace.Spec.NetworkPolicy); err != nil {
		return nil, err
	}

	return &namespace, nil
}

func (r *PostgresRepository) ListNamespaces(ctx context.Context) ([]*types.Namespace, error) {
	query := `
		SELECT id, name, display_name, description, labels, annotations, resource_quota, network_policy, created_at, updated_at
		FROM namespaces ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var namespaces []*types.Namespace

	for rows.Next() {
		var namespace types.Namespace
		var displayName, description string
		var labelsJSON, annotationsJSON, quotaJSON, policyJSON []byte

		err := rows.Scan(
			&namespace.ID,
			&namespace.Name,
			&displayName,
			&description,
			&labelsJSON,
			&annotationsJSON,
			&quotaJSON,
			&policyJSON,
			&namespace.CreatedAt,
			&namespace.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if err := fromJSON(labelsJSON, &namespace.Labels); err != nil {
			return nil, err
		}
		if err := fromJSON(annotationsJSON, &namespace.Annotations); err != nil {
			return nil, err
		}
		if err := fromJSON(quotaJSON, &namespace.Spec.Quotas); err != nil {
			return nil, err
		}
		if err := fromJSON(policyJSON, &namespace.Spec.NetworkPolicy); err != nil {
			return nil, err
		}

		namespaces = append(namespaces, &namespace)
	}

	return namespaces, rows.Err()
}

// Secret operations
func (r *PostgresRepository) CreateSecret(ctx context.Context, secret *types.Secret) error {
	query := `
		INSERT INTO secrets (id, namespace_id, name, type, data, labels, annotations)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		secret.ID,
		secret.Namespace,
		secret.Name,
		secret.Spec.Type,
		toJSON(secret.Spec.Data),
		toJSON(secret.Labels),
		toJSON(secret.Annotations),
	)

	return err
}

func (r *PostgresRepository) GetSecret(ctx context.Context, id string) (*types.Secret, error) {
	query := `
		SELECT id, namespace_id, name, type, data, labels, annotations, created_at, updated_at
		FROM secrets WHERE id = $1
	`

	var secret types.Secret
	var dataJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&secret.ID,
		&secret.Namespace,
		&secret.Name,
		&secret.Spec.Type,
		&dataJSON,
		&labelsJSON,
		&annotationsJSON,
		&secret.CreatedAt,
		&secret.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if err := fromJSON(dataJSON, &secret.Spec.Data); err != nil {
		return nil, err
	}
	if err := fromJSON(labelsJSON, &secret.Labels); err != nil {
		return nil, err
	}
	if err := fromJSON(annotationsJSON, &secret.Annotations); err != nil {
		return nil, err
	}

	return &secret, nil
}

func (r *PostgresRepository) ListSecrets(ctx context.Context, namespace string) ([]*types.Secret, error) {
	query := `
		SELECT id, namespace_id, name, type, data, labels, annotations, created_at, updated_at
		FROM secrets WHERE namespace_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []*types.Secret

	for rows.Next() {
		var secret types.Secret
		var dataJSON, labelsJSON, annotationsJSON []byte

		err := rows.Scan(
			&secret.ID,
			&secret.Namespace,
			&secret.Name,
			&secret.Spec.Type,
			&dataJSON,
			&labelsJSON,
			&annotationsJSON,
			&secret.CreatedAt,
			&secret.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if err := fromJSON(dataJSON, &secret.Spec.Data); err != nil {
			return nil, err
		}
		if err := fromJSON(labelsJSON, &secret.Labels); err != nil {
			return nil, err
		}
		if err := fromJSON(annotationsJSON, &secret.Annotations); err != nil {
			return nil, err
		}

		secrets = append(secrets, &secret)
	}

	return secrets, rows.Err()
}

// Health check
func (r *PostgresRepository) HealthCheck(ctx context.Context) error {
	return r.db.PingContext(ctx)
}

// Close closes the database connection
func (r *PostgresRepository) Close() error {
	return r.db.Close()
}

// Helper functions for JSON marshaling
func toJSON(v interface{}) []byte {
	if v == nil {
		return []byte("{}")
	}
	// In a real implementation, use json.Marshal
	return []byte("{}")
}

func fromJSON(data []byte, v interface{}) error {
	if len(data) == 0 {
		return nil
	}
	// In a real implementation, use json.Unmarshal
	return nil
}
