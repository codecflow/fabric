package postgres

import (
	"context"
	"database/sql"

	"github.com/codecflow/fabric/weaver/internal/namespace"
	"github.com/codecflow/fabric/weaver/internal/repository"
)

// NamespaceRepository implements namespace.Repository
type NamespaceRepository struct {
	db *sql.DB
}

// NewNamespaceRepository creates a new namespace repository
func NewNamespaceRepository(db *sql.DB) *NamespaceRepository {
	return &NamespaceRepository{db: db}
}

// Create creates a new namespace
func (r *NamespaceRepository) Create(ctx context.Context, ns *namespace.Namespace) error {
	query := `
		INSERT INTO namespaces (id, name, labels, annotations, spec, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(ctx, query,
		ns.ID,
		ns.Name,
		toJSON(ns.Labels),
		toJSON(ns.Annotations),
		toJSON(ns.Spec),
		toJSON(ns.Status),
		ns.CreatedAt,
		ns.UpdatedAt,
	)

	return err
}

// Get retrieves a namespace by name
func (r *NamespaceRepository) Get(ctx context.Context, name string) (*namespace.Namespace, error) {
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

// Update updates an existing namespace
func (r *NamespaceRepository) Update(ctx context.Context, ns *namespace.Namespace) error {
	query := `
		UPDATE namespaces 
		SET labels = $2, annotations = $3, spec = $4, status = $5, updated_at = $6
		WHERE name = $1
	`

	result, err := r.db.ExecContext(ctx, query,
		ns.Name,
		toJSON(ns.Labels),
		toJSON(ns.Annotations),
		toJSON(ns.Spec),
		toJSON(ns.Status),
		ns.UpdatedAt,
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

// Delete deletes a namespace
func (r *NamespaceRepository) Delete(ctx context.Context, name string) error {
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

// List lists namespaces with optional filtering
func (r *NamespaceRepository) List(ctx context.Context, filters map[string]string) ([]*namespace.Namespace, error) {
	query := `
		SELECT id, name, labels, annotations, spec, status, created_at, updated_at
		FROM namespaces ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
