package postgres

import (
	"context"
	"database/sql"

	"github.com/codecflow/fabric/pkg/secret"
	"github.com/codecflow/fabric/weaver/internal/repository"
)

// SecretRepository implements secret.Repository
type SecretRepository struct {
	db *sql.DB
}

// NewSecretRepository creates a new secret repository
func NewSecretRepository(db *sql.DB) *SecretRepository {
	return &SecretRepository{db: db}
}

// Create creates a new secret
func (r *SecretRepository) Create(ctx context.Context, s *secret.Secret) error {
	query := `
		INSERT INTO secrets (id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		s.ID,
		s.Namespace,
		s.Name,
		toJSON(s.Spec),
		toJSON(s.Status),
		toJSON(s.Labels),
		toJSON(s.Annotations),
		s.CreatedAt,
		s.UpdatedAt,
	)

	return err
}

// Get retrieves a secret by namespace and name
func (r *SecretRepository) Get(ctx context.Context, namespace, name string) (*secret.Secret, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM secrets WHERE namespace_id = $1 AND name = $2
	`

	var s secret.Secret
	var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, namespace, name).Scan(
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

// GetByID retrieves a secret by ID
func (r *SecretRepository) GetByID(ctx context.Context, id string) (*secret.Secret, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM secrets WHERE id = $1
	`

	var s secret.Secret
	var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
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

// Update updates an existing secret
func (r *SecretRepository) Update(ctx context.Context, s *secret.Secret) error {
	query := `
		UPDATE secrets 
		SET spec = $3, status = $4, labels = $5, annotations = $6, updated_at = $7
		WHERE namespace_id = $1 AND name = $2
	`

	result, err := r.db.ExecContext(ctx, query,
		s.Namespace,
		s.Name,
		toJSON(s.Spec),
		toJSON(s.Status),
		toJSON(s.Labels),
		toJSON(s.Annotations),
		s.UpdatedAt,
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

// Delete deletes a secret
func (r *SecretRepository) Delete(ctx context.Context, namespace, name string) error {
	query := `DELETE FROM secrets WHERE namespace_id = $1 AND name = $2`

	result, err := r.db.ExecContext(ctx, query, namespace, name)
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

// List lists secrets with optional filtering
func (r *SecretRepository) List(ctx context.Context, filter secret.Filter) (*secret.List, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM secrets WHERE namespace_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, filter.Namespace)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }() // nolint:errcheck

	var secrets []secret.Secret

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

		secrets = append(secrets, s)
	}

	return &secret.List{Items: secrets}, rows.Err()
}
