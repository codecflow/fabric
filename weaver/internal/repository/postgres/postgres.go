package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"weaver/internal/namespace"
	"weaver/internal/repository"
	"weaver/internal/secret"
	"weaver/internal/workload"

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
		labels JSONB,
		annotations JSONB,
		spec JSONB,
		status JSONB,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
		updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS secrets (
		id VARCHAR(255) PRIMARY KEY,
		namespace_id VARCHAR(255) REFERENCES namespaces(id),
		name VARCHAR(255) NOT NULL,
		spec JSONB,
		status JSONB,
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
func (r *PostgresRepository) CreateWorkload(ctx context.Context, w *workload.Workload) error {
	query := `
		INSERT INTO workloads (id, namespace_id, name, spec, status, labels, annotations)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		w.ID,
		w.Namespace,
		w.Name,
		toJSON(w.Spec),
		toJSON(w.Status),
		toJSON(w.Labels),
		toJSON(w.Annotations),
	)

	return err
}

func (r *PostgresRepository) GetWorkload(ctx context.Context, id string) (*workload.Workload, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM workloads WHERE id = $1
	`

	var w workload.Workload
	var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&w.ID,
		&w.Namespace,
		&w.Name,
		&specJSON,
		&statusJSON,
		&labelsJSON,
		&annotationsJSON,
		&w.CreatedAt,
		&w.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if err := fromJSON(specJSON, &w.Spec); err != nil {
		return nil, err
	}
	if err := fromJSON(statusJSON, &w.Status); err != nil {
		return nil, err
	}
	if err := fromJSON(labelsJSON, &w.Labels); err != nil {
		return nil, err
	}
	if err := fromJSON(annotationsJSON, &w.Annotations); err != nil {
		return nil, err
	}

	return &w, nil
}

func (r *PostgresRepository) UpdateWorkload(ctx context.Context, w *workload.Workload) error {
	query := `
		UPDATE workloads 
		SET spec = $2, status = $3, labels = $4, annotations = $5, updated_at = NOW()
		WHERE id = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		w.ID,
		toJSON(w.Spec),
		toJSON(w.Status),
		toJSON(w.Labels),
		toJSON(w.Annotations),
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

func (r *PostgresRepository) GetWorkloadByName(ctx context.Context, ns, name string) (*workload.Workload, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM workloads WHERE namespace_id = $1 AND name = $2
	`

	var w workload.Workload
	var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, ns, name).Scan(
		&w.ID,
		&w.Namespace,
		&w.Name,
		&specJSON,
		&statusJSON,
		&labelsJSON,
		&annotationsJSON,
		&w.CreatedAt,
		&w.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if err := fromJSON(specJSON, &w.Spec); err != nil {
		return nil, err
	}
	if err := fromJSON(statusJSON, &w.Status); err != nil {
		return nil, err
	}
	if err := fromJSON(labelsJSON, &w.Labels); err != nil {
		return nil, err
	}
	if err := fromJSON(annotationsJSON, &w.Annotations); err != nil {
		return nil, err
	}

	return &w, nil
}

func (r *PostgresRepository) ListWorkloads(ctx context.Context, ns string, filters map[string]string) ([]*workload.Workload, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM workloads WHERE namespace_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, ns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workloads []*workload.Workload

	for rows.Next() {
		var w workload.Workload
		var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

		err := rows.Scan(
			&w.ID,
			&w.Namespace,
			&w.Name,
			&specJSON,
			&statusJSON,
			&labelsJSON,
			&annotationsJSON,
			&w.CreatedAt,
			&w.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if err := fromJSON(specJSON, &w.Spec); err != nil {
			return nil, err
		}
		if err := fromJSON(statusJSON, &w.Status); err != nil {
			return nil, err
		}
		if err := fromJSON(labelsJSON, &w.Labels); err != nil {
			return nil, err
		}
		if err := fromJSON(annotationsJSON, &w.Annotations); err != nil {
			return nil, err
		}

		workloads = append(workloads, &w)
	}

	return workloads, rows.Err()
}

// Namespace operations
func (r *PostgresRepository) CreateNamespace(ctx context.Context, ns *namespace.Namespace) error {
	query := `
		INSERT INTO namespaces (id, name, labels, annotations, spec, status)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := r.db.ExecContext(ctx, query,
		ns.ID,
		ns.Name,
		toJSON(ns.Labels),
		toJSON(ns.Annotations),
		toJSON(ns.Spec),
		toJSON(ns.Status),
	)

	return err
}

func (r *PostgresRepository) GetNamespace(ctx context.Context, name string) (*namespace.Namespace, error) {
	query := `
		SELECT id, name, labels, annotations, spec, status, created_at, updated_at
		FROM namespaces WHERE name = $1
	`

	var ns namespace.Namespace
	var labelsJSON, annotationsJSON, specJSON, statusJSON []byte

	err := r.db.QueryRowContext(ctx, query, name).Scan(
		&ns.ID,
		&ns.Name,
		&labelsJSON,
		&annotationsJSON,
		&specJSON,
		&statusJSON,
		&ns.CreatedAt,
		&ns.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if err := fromJSON(labelsJSON, &ns.Labels); err != nil {
		return nil, err
	}
	if err := fromJSON(annotationsJSON, &ns.Annotations); err != nil {
		return nil, err
	}
	if err := fromJSON(specJSON, &ns.Spec); err != nil {
		return nil, err
	}
	if err := fromJSON(statusJSON, &ns.Status); err != nil {
		return nil, err
	}

	return &ns, nil
}

func (r *PostgresRepository) UpdateNamespace(ctx context.Context, ns *namespace.Namespace) error {
	query := `
		UPDATE namespaces 
		SET labels = $2, annotations = $3, spec = $4, status = $5, updated_at = NOW()
		WHERE name = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		ns.Name,
		toJSON(ns.Labels),
		toJSON(ns.Annotations),
		toJSON(ns.Spec),
		toJSON(ns.Status),
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

func (r *PostgresRepository) ListNamespaces(ctx context.Context, filters map[string]string) ([]*namespace.Namespace, error) {
	query := `
		SELECT id, name, labels, annotations, spec, status, created_at, updated_at
		FROM namespaces ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var namespaces []*namespace.Namespace

	for rows.Next() {
		var ns namespace.Namespace
		var labelsJSON, annotationsJSON, specJSON, statusJSON []byte

		err := rows.Scan(
			&ns.ID,
			&ns.Name,
			&labelsJSON,
			&annotationsJSON,
			&specJSON,
			&statusJSON,
			&ns.CreatedAt,
			&ns.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if err := fromJSON(labelsJSON, &ns.Labels); err != nil {
			return nil, err
		}
		if err := fromJSON(annotationsJSON, &ns.Annotations); err != nil {
			return nil, err
		}
		if err := fromJSON(specJSON, &ns.Spec); err != nil {
			return nil, err
		}
		if err := fromJSON(statusJSON, &ns.Status); err != nil {
			return nil, err
		}

		namespaces = append(namespaces, &ns)
	}

	return namespaces, rows.Err()
}

func (r *PostgresRepository) DeleteNamespace(ctx context.Context, name string) error {
	query := `DELETE FROM namespaces WHERE name = $1`

	result, err := r.db.ExecContext(ctx, query, name)
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

// Secret operations
func (r *PostgresRepository) CreateSecret(ctx context.Context, s *secret.Secret) error {
	query := `
		INSERT INTO secrets (id, namespace_id, name, spec, status, labels, annotations)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.ExecContext(ctx, query,
		s.ID,
		s.Namespace,
		s.Name,
		toJSON(s.Spec),
		toJSON(s.Status),
		toJSON(s.Labels),
		toJSON(s.Annotations),
	)

	return err
}

func (r *PostgresRepository) GetSecret(ctx context.Context, ns, name string) (*secret.Secret, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM secrets WHERE namespace_id = $1 AND name = $2
	`

	var s secret.Secret
	var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, ns, name).Scan(
		&s.ID,
		&s.Namespace,
		&s.Name,
		&specJSON,
		&statusJSON,
		&labelsJSON,
		&annotationsJSON,
		&s.CreatedAt,
		&s.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}

	// Parse JSON fields
	if err := fromJSON(specJSON, &s.Spec); err != nil {
		return nil, err
	}
	if err := fromJSON(statusJSON, &s.Status); err != nil {
		return nil, err
	}
	if err := fromJSON(labelsJSON, &s.Labels); err != nil {
		return nil, err
	}
	if err := fromJSON(annotationsJSON, &s.Annotations); err != nil {
		return nil, err
	}

	return &s, nil
}

func (r *PostgresRepository) UpdateSecret(ctx context.Context, s *secret.Secret) error {
	query := `
		UPDATE secrets 
		SET spec = $3, status = $4, labels = $5, annotations = $6, updated_at = NOW()
		WHERE namespace_id = $1 AND name = $2
	`

	result, err := r.db.ExecContext(ctx, query,
		s.Namespace,
		s.Name,
		toJSON(s.Spec),
		toJSON(s.Status),
		toJSON(s.Labels),
		toJSON(s.Annotations),
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

func (r *PostgresRepository) ListSecrets(ctx context.Context, ns string, filters map[string]string) ([]*secret.Secret, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM secrets WHERE namespace_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, ns)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []*secret.Secret

	for rows.Next() {
		var s secret.Secret
		var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

		err := rows.Scan(
			&s.ID,
			&s.Namespace,
			&s.Name,
			&specJSON,
			&statusJSON,
			&labelsJSON,
			&annotationsJSON,
			&s.CreatedAt,
			&s.UpdatedAt,
		)

		if err != nil {
			return nil, err
		}

		// Parse JSON fields
		if err := fromJSON(specJSON, &s.Spec); err != nil {
			return nil, err
		}
		if err := fromJSON(statusJSON, &s.Status); err != nil {
			return nil, err
		}
		if err := fromJSON(labelsJSON, &s.Labels); err != nil {
			return nil, err
		}
		if err := fromJSON(annotationsJSON, &s.Annotations); err != nil {
			return nil, err
		}

		secrets = append(secrets, &s)
	}

	return secrets, rows.Err()
}

func (r *PostgresRepository) DeleteSecret(ctx context.Context, ns, name string) error {
	query := `DELETE FROM secrets WHERE namespace_id = $1 AND name = $2`

	result, err := r.db.ExecContext(ctx, query, ns, name)
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
