package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/google/uuid"
)

// ContainerState represents the state of a container
type ContainerState struct {
	ID          string                `json:"id"`
	ContainerID string                `json:"container_id,omitempty"` // Docker container ID
	Spec        *container.Spec       `json:"spec"`
	Status      string                `json:"status"` // created, running, completed, failed, stopped
	Result      *container.Result     `json:"result,omitempty"`
	Error       string                `json:"error,omitempty"`
	Ports       map[int]int           `json:"ports,omitempty"` // container port -> host port
	CreatedAt   time.Time             `json:"created_at"`
	StartedAt   *time.Time            `json:"started_at,omitempty"`
	EndedAt     *time.Time            `json:"ended_at,omitempty"`
	cancelFn    context.CancelFunc    // internal - not exported
}

// Manager manages container lifecycle
type Manager struct {
	runner     container.Runner
	containers map[string]*ContainerState
	mu         sync.RWMutex
}

// NewManager creates a new container manager
func NewManager(runner container.Runner) *Manager {
	return &Manager{
		runner:     runner,
		containers: make(map[string]*ContainerState),
	}
}

// Create creates and starts a container
func (m *Manager) Create(ctx context.Context, spec *container.Spec) (*ContainerState, error) {
	// Generate unique container ID
	id := uuid.New().String()

	// Create container state
	state := &ContainerState{
		ID:        id,
		Spec:      spec,
		Status:    "created",
		CreatedAt: time.Now(),
	}

	// Store in map
	m.mu.Lock()
	m.containers[id] = state
	m.mu.Unlock()

	// Handle persistent vs ephemeral containers
	if spec.Persistent {
		// Start persistent container immediately and get port mappings
		return m.startPersistent(ctx, state)
	}

	// Create cancellable context for ephemeral container
	containerCtx, cancel := context.WithCancel(ctx)
	state.cancelFn = cancel

	// Run ephemeral container asynchronously
	go m.runContainer(containerCtx, state)

	return state, nil
}

// startPersistent starts a persistent container and returns immediately with port info
func (m *Manager) startPersistent(ctx context.Context, state *ContainerState) (*ContainerState, error) {
	// Type assert to get access to Start method
	dockerRunner, ok := m.runner.(*container.DockerRunner)
	if !ok {
		return nil, fmt.Errorf("persistent containers require DockerRunner")
	}

	// Start the container
	containerID, ports, err := dockerRunner.Start(ctx, state.Spec)
	if err != nil {
		m.mu.Lock()
		state.Status = "failed"
		state.Error = err.Error()
		m.mu.Unlock()
		return state, err
	}

	// Update state with container ID and ports
	m.mu.Lock()
	state.ContainerID = containerID
	state.Ports = ports
	state.Status = "running"
	now := time.Now()
	state.StartedAt = &now
	m.mu.Unlock()

	return state, nil
}

// runContainer executes the container and updates state
func (m *Manager) runContainer(ctx context.Context, state *ContainerState) {
	// Update status to running
	m.mu.Lock()
	state.Status = "running"
	now := time.Now()
	state.StartedAt = &now
	m.mu.Unlock()

	// Run the container
	result, err := m.runner.Run(ctx, state.Spec)

	// Update final state
	m.mu.Lock()
	defer m.mu.Unlock()

	endTime := time.Now()
	state.EndedAt = &endTime

	if err != nil {
		state.Status = "failed"
		state.Error = err.Error()
	} else {
		state.Status = "completed"
		state.Result = result
	}
}

// Get retrieves a container state by ID
func (m *Manager) Get(id string) (*ContainerState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.containers[id]
	if !ok {
		return nil, fmt.Errorf("container not found: %s", id)
	}

	return state, nil
}

// List returns all containers
func (m *Manager) List() []*ContainerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make([]*ContainerState, 0, len(m.containers))
	for _, state := range m.containers {
		states = append(states, state)
	}

	return states
}

// Stop stops a running container
func (m *Manager) Stop(id string) error {
	m.mu.RLock()
	state, ok := m.containers[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}

	if state.Status != "running" {
		return fmt.Errorf("container is not running: %s (status: %s)", id, state.Status)
	}

	// Handle persistent containers differently
	if state.Spec.Persistent && state.ContainerID != "" {
		dockerRunner, ok := m.runner.(*container.DockerRunner)
		if !ok {
			return fmt.Errorf("cannot stop persistent container: runner is not DockerRunner")
		}

		// Stop the Docker container
		ctx := context.Background()
		if err := dockerRunner.Stop(ctx, state.ContainerID); err != nil {
			return fmt.Errorf("failed to stop container: %w", err)
		}

		// Remove the container
		if err := dockerRunner.Remove(ctx, state.ContainerID); err != nil {
			return fmt.Errorf("failed to remove container: %w", err)
		}
	} else {
		// For ephemeral containers, cancel the context
		if state.cancelFn != nil {
			state.cancelFn()
		}
	}

	// Update state
	m.mu.Lock()
	state.Status = "stopped"
	now := time.Now()
	state.EndedAt = &now
	m.mu.Unlock()

	return nil
}

// Delete removes a container from the manager
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}

	// Stop if still running
	if state.Status == "running" && state.cancelFn != nil {
		state.cancelFn()
	}

	delete(m.containers, id)
	return nil
}
