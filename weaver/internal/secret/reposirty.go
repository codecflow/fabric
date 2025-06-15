package secret

import "context"

type Repository interface {
	Create(ctx context.Context, secret *Secret) error
	Get(ctx context.Context, namespace, name string) (*Secret, error)
	GetByID(ctx context.Context, id string) (*Secret, error)
	Update(ctx context.Context, secret *Secret) error
	Delete(ctx context.Context, namespace, name string) error
	List(ctx context.Context, filter Filter) (*List, error)
}
