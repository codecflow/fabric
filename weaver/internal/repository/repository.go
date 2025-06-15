package repository

import (
	"context"
	"errors"
	"weaver/internal/namespace"
	"weaver/internal/secret"
	"weaver/internal/workload"
)

// Common errors
var (
	ErrNotFound = errors.New("resource not found")
)

// Repository defines the interface for data persistence
type Repository interface {
	// Workload operations
	CreateWorkload(ctx context.Context, workload *workload.Workload) error
	GetWorkload(ctx context.Context, id string) (*workload.Workload, error)
	GetWorkloadByName(ctx context.Context, namespace, name string) (*workload.Workload, error)
	UpdateWorkload(ctx context.Context, workload *workload.Workload) error
	DeleteWorkload(ctx context.Context, id string) error
	ListWorkloads(ctx context.Context, namespace string, filters map[string]string) ([]*workload.Workload, error)

	// Namespace operations
	CreateNamespace(ctx context.Context, ns *namespace.Namespace) error
	GetNamespace(ctx context.Context, name string) (*namespace.Namespace, error)
	UpdateNamespace(ctx context.Context, ns *namespace.Namespace) error
	DeleteNamespace(ctx context.Context, name string) error
	ListNamespaces(ctx context.Context, filters map[string]string) ([]*namespace.Namespace, error)

	// Secret operations
	CreateSecret(ctx context.Context, secret *secret.Secret) error
	GetSecret(ctx context.Context, namespace, name string) (*secret.Secret, error)
	UpdateSecret(ctx context.Context, secret *secret.Secret) error
	DeleteSecret(ctx context.Context, namespace, name string) error
	ListSecrets(ctx context.Context, namespace string, filters map[string]string) ([]*secret.Secret, error)

	// Health and maintenance
	HealthCheck(ctx context.Context) error
	Close() error
}
