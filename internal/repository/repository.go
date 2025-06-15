package repository

import (
	"context"

	"fabric/internal/types"
)

// Repository defines the interface for data persistence
type Repository interface {
	// Workload operations
	CreateWorkload(ctx context.Context, workload *types.Workload) error
	GetWorkload(ctx context.Context, id string) (*types.Workload, error)
	GetWorkloadByName(ctx context.Context, namespace, name string) (*types.Workload, error)
	UpdateWorkload(ctx context.Context, workload *types.Workload) error
	DeleteWorkload(ctx context.Context, id string) error
	ListWorkloads(ctx context.Context, namespace string, filters map[string]string) ([]*types.Workload, error)

	// Namespace operations
	CreateNamespace(ctx context.Context, namespace *types.Namespace) error
	GetNamespace(ctx context.Context, name string) (*types.Namespace, error)
	UpdateNamespace(ctx context.Context, namespace *types.Namespace) error
	DeleteNamespace(ctx context.Context, name string) error
	ListNamespaces(ctx context.Context, filters map[string]string) ([]*types.Namespace, error)

	// Secret operations
	CreateSecret(ctx context.Context, secret *types.Secret) error
	GetSecret(ctx context.Context, namespace, name string) (*types.Secret, error)
	UpdateSecret(ctx context.Context, secret *types.Secret) error
	DeleteSecret(ctx context.Context, namespace, name string) error
	ListSecrets(ctx context.Context, namespace string, filters map[string]string) ([]*types.Secret, error)

	// Health and maintenance
	HealthCheck(ctx context.Context) error
	Close() error
}

// WorkloadFilter defines filters for workload queries
type WorkloadFilter struct {
	Namespace string
	Labels    map[string]string
	Phase     types.WorkloadPhase
	Provider  string
	NodeID    string
}

// NamespaceFilter defines filters for namespace queries
type NamespaceFilter struct {
	Labels map[string]string
	Phase  types.NamespacePhase
}

// SecretFilter defines filters for secret queries
type SecretFilter struct {
	Namespace string
	Labels    map[string]string
	Type      types.SecretType
}
