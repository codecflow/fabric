package postgres

import (
	"context"
	"database/sql"

	"github.com/codecflow/fabric/pkg/workload"
	"github.com/codecflow/fabric/weaver/internal/repository"
)

// WorkloadRepository implements workload.Repository
type WorkloadRepository struct {
	db *sql.DB
}

// NewWorkloadRepository creates a new workload repository
func NewWorkloadRepository(db *sql.DB) *WorkloadRepository {
	return &WorkloadRepository{db: db}
}

// Create creates a new workload
func (r *WorkloadRepository) Create(ctx context.Context, w *workload.Workload) error {
	query := `
		INSERT INTO workloads (id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.ExecContext(ctx, query,
		w.ID,
		w.Namespace,
		w.Name,
		toJSON(w.Spec),
		toJSON(w.Status),
		toJSON(w.Labels),
		toJSON(w.Annotations),
		w.CreatedAt,
		w.UpdatedAt,
	)

	return err
}

// Get retrieves a workload by ID
func (r *WorkloadRepository) Get(ctx context.Context, id string) (*workload.Workload, error) {
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

// GetByName retrieves a workload by namespace and name
func (r *WorkloadRepository) GetByName(ctx context.Context, namespace, name string) (*workload.Workload, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM workloads WHERE namespace_id = $1 AND name = $2
	`

	var w workload.Workload
	var specJSON, statusJSON, labelsJSON, annotationsJSON []byte

	err := r.db.QueryRowContext(ctx, query, namespace, name).Scan(
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

// Update updates an existing workload
func (r *WorkloadRepository) Update(ctx context.Context, w *workload.Workload) error {
	query := `
		UPDATE workloads 
		SET spec = $3, status = $4, labels = $5, annotations = $6, updated_at = $7
		WHERE namespace_id = $1 AND name = $2
	`

	result, err := r.db.ExecContext(ctx, query,
		w.Namespace,
		w.Name,
		toJSON(w.Spec),
		toJSON(w.Status),
		toJSON(w.Labels),
		toJSON(w.Annotations),
		w.UpdatedAt,
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

// Delete deletes a workload
func (r *WorkloadRepository) Delete(ctx context.Context, id string) error {
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

// List lists workloads with optional filtering
func (r *WorkloadRepository) List(ctx context.Context, namespace string, filters map[string]string) ([]*workload.Workload, error) {
	query := `
		SELECT id, namespace_id, name, spec, status, labels, annotations, created_at, updated_at
		FROM workloads WHERE namespace_id = $1 ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query, namespace)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

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
