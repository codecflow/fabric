package workload

import "context"

type Repository interface {
	Create(ctx context.Context, workload *Workload) error
	Get(ctx context.Context, id string) (*Workload, error)
	GetByName(ctx context.Context, namespace, name string) (*Workload, error)
	Update(ctx context.Context, workload *Workload) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, namespace string, filters map[string]string) ([]*Workload, error)
}
