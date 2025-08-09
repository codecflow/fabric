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
	ID        string                `json:"id"`
	Spec      *container.Spec       `json:"spec"`
	Status    string                `json:"status"` // created, running, completed, failed
	Result    *container.Result     `json:"result,omitempty"`
	Error     string                `json:"error,omitempty"`
	CreatedAt time.Time             `json:"created_at"`
	StartedAt *time.Time            `json:"started_at,omitempty"`
	EndedAt   *time.Time            `json:"ended_at,omitempty"`
	cancelFn  context.CancelFunc    // internal - not exported
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

// Create creates and starts a container asynchronously
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

	// Create cancellable context for this container
	containerCtx, cancel := context.WithCancel(ctx)
	state.cancelFn = cancel

	// Run container asynchronously
	go m.runContainer(containerCtx, state)

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
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.containers[id]
	if !ok {
		return fmt.Errorf("container not found: %s", id)
	}

	if state.Status != "running" {
		return fmt.Errorf("container is not running: %s (status: %s)", id, state.Status)
	}

	// Cancel the context to stop the container
	if state.cancelFn != nil {
		state.cancelFn()
	}

	state.Status = "stopped"
	now := time.Now()
	state.EndedAt = &now

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
