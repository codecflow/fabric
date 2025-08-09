package container

import "context"

// Runner is the interface for executing containers
type Runner interface {
	// Run executes a container with the given specification
	// and returns the result once the container completes
	Run(ctx context.Context, spec *Spec) (*Result, error)
}
