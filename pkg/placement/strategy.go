package placement

import (
	"context"

	"github.com/codecflow/fabric/pkg/container"
)

// Strategy defines the interface for placing containers on different backends
type Strategy interface {
	// Create creates a container on the backend
	Create(ctx context.Context, req *Request) (*Placement, error)

	// Get retrieves information about a placement
	Get(ctx context.Context, id string) (*Placement, error)

	// Stop stops a running placement
	Stop(ctx context.Context, id string) error

	// Delete removes a placement
	Delete(ctx context.Context, id string) error

	// List returns all placements
	List(ctx context.Context) ([]*Placement, error)

	// Name returns the name of this strategy
	Name() string
}

// Request represents a request to place a container
type Request struct {
	Spec     *container.Spec
	Provider string // "docker", "kubernetes", etc.
	Region   string // Optional region/zone
}

// Placement represents a placed container
type Placement struct {
	ID          string                 // Unique placement ID
	ContainerID string                 // Backend-specific container ID
	Provider    string                 // Backend provider name
	Spec        *container.Spec        // Original spec
	Status      string                 // Status (created, running, completed, failed, stopped)
	Ports       map[int]int            // Port mappings (container -> host)
	Address     string                 // Access address
	Metadata    map[string]interface{} // Provider-specific metadata
}
