package placement

import (
	"context"
	"fmt"
	"sync"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/google/uuid"
)

// DockerStrategy implements Strategy using Docker as the backend
type DockerStrategy struct {
	runner     *container.DockerRunner
	placements map[string]*Placement
	mu         sync.RWMutex
}

// NewDockerStrategy creates a new Docker placement strategy
func NewDockerStrategy() (*DockerStrategy, error) {
	runner, err := container.NewDockerRunner()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker runner: %w", err)
	}

	return &DockerStrategy{
		runner:     runner,
		placements: make(map[string]*Placement),
	}, nil
}

// Name returns the name of this strategy
func (d *DockerStrategy) Name() string {
	return "docker"
}

// Create creates a container using Docker
func (d *DockerStrategy) Create(ctx context.Context, req *Request) (*Placement, error) {
	// Generate unique placement ID
	id := uuid.New().String()

	// Create placement record
	placement := &Placement{
		ID:       id,
		Provider: "docker",
		Spec:     req.Spec,
		Status:   "created",
		Metadata: make(map[string]interface{}),
	}

	// Store placement
	d.mu.Lock()
	d.placements[id] = placement
	d.mu.Unlock()

	// Handle persistent vs ephemeral
	if req.Spec.Persistent {
		// Start persistent container
		containerID, ports, err := d.runner.Start(ctx, req.Spec)
		if err != nil {
			d.mu.Lock()
			placement.Status = "failed"
			placement.Metadata["error"] = err.Error()
			d.mu.Unlock()
			return placement, err
		}

		d.mu.Lock()
		placement.ContainerID = containerID
		placement.Ports = ports
		placement.Status = "running"
		placement.Address = fmt.Sprintf("localhost:%d", getFirstPort(ports))
		d.mu.Unlock()
	} else {
		// Run ephemeral container asynchronously
		go d.runEphemeral(ctx, placement)
		placement.Status = "running"
	}

	return placement, nil
}

// runEphemeral runs an ephemeral container in the background
func (d *DockerStrategy) runEphemeral(ctx context.Context, placement *Placement) {
	result, err := d.runner.Run(ctx, placement.Spec)

	d.mu.Lock()
	defer d.mu.Unlock()

	if err != nil {
		placement.Status = "failed"
		placement.Metadata["error"] = err.Error()
	} else {
		placement.Status = "completed"
		placement.Metadata["exit_code"] = result.ExitCode
		placement.Metadata["stdout"] = result.Stdout
		placement.Metadata["stderr"] = result.Stderr
	}
}

// Get retrieves a placement by ID
func (d *DockerStrategy) Get(ctx context.Context, id string) (*Placement, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	placement, ok := d.placements[id]
	if !ok {
		return nil, fmt.Errorf("placement not found: %s", id)
	}

	return placement, nil
}

// Stop stops a running placement
func (d *DockerStrategy) Stop(ctx context.Context, id string) error {
	d.mu.RLock()
	placement, ok := d.placements[id]
	d.mu.RUnlock()

	if !ok {
		return fmt.Errorf("placement not found: %s", id)
	}

	if placement.Status != "running" {
		return fmt.Errorf("placement is not running: %s (status: %s)", id, placement.Status)
	}

	// Stop persistent containers
	if placement.Spec.Persistent && placement.ContainerID != "" {
		if err := d.runner.Stop(ctx, placement.ContainerID); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}
	}

	d.mu.Lock()
	placement.Status = "stopped"
	d.mu.Unlock()

	return nil
}

// Delete removes a placement
func (d *DockerStrategy) Delete(ctx context.Context, id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	placement, ok := d.placements[id]
	if !ok {
		return fmt.Errorf("placement not found: %s", id)
	}

	// Remove Docker container if persistent
	if placement.Spec.Persistent && placement.ContainerID != "" {
		if err := d.runner.Remove(ctx, placement.ContainerID); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	}

	delete(d.placements, id)
	return nil
}

// List returns all placements
func (d *DockerStrategy) List(ctx context.Context) ([]*Placement, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	placements := make([]*Placement, 0, len(d.placements))
	for _, p := range d.placements {
		placements = append(placements, p)
	}

	return placements, nil
}

// Close cleans up resources
func (d *DockerStrategy) Close() error {
	if d.runner != nil {
		return d.runner.Close()
	}
	return nil
}

// Helper function to get first port from map
func getFirstPort(ports map[int]int) int {
	for _, hostPort := range ports {
		return hostPort
	}
	return 0
}
