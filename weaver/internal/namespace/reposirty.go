package namespace

import "context"

type Repository interface {
	Create(ctx context.Context, ns *Namespace) error
	Get(ctx context.Context, name string) (*Namespace, error)
	Update(ctx context.Context, ns *Namespace) error
	Delete(ctx context.Context, name string) error
	List(ctx context.Context, filters map[string]string) ([]*Namespace, error)
}
