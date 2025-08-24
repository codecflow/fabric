package volume

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/codecflow/fabric/pkg/types"
)

// Manager handles volume lifecycle
type Manager struct {
	mu      sync.RWMutex
	volumes map[string]*types.Volume
	backend Backend
}

// Backend defines the storage backend interface
type Backend interface {
	// CreateVolume creates a new volume
	CreateVolume(ctx context.Context, spec *types.VolumeSpec) (*types.Volume, error)

	// DeleteVolume removes a volume
	DeleteVolume(ctx context.Context, volumeID string) error

	// GetVolume retrieves volume details
	GetVolume(ctx context.Context, volumeID string) (*types.Volume, error)

	// ListVolumes lists all volumes
	ListVolumes(ctx context.Context) ([]*types.Volume, error)
}

// NewManager creates a new volume manager
func NewManager(backend Backend) *Manager {
	return &Manager{
		volumes: make(map[string]*types.Volume),
		backend: backend,
	}
}

// Create creates a new volume
func (m *Manager) Create(ctx context.Context, spec *types.VolumeSpec) (*types.Volume, error) {
	// Validate spec
	if spec.Name == "" {
		return nil, fmt.Errorf("volume name is required")
	}
	if spec.SizeGB <= 0 {
		return nil, fmt.Errorf("volume size must be greater than 0")
	}

	// Create via backend
	volume, err := m.backend.CreateVolume(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to create volume: %w", err)
	}

	// Store in memory
	m.mu.Lock()
	m.volumes[volume.ID] = volume
	m.mu.Unlock()

	return volume, nil
}

// Get retrieves a volume by ID
func (m *Manager) Get(ctx context.Context, volumeID string) (*types.Volume, error) {
	m.mu.RLock()
	volume, ok := m.volumes[volumeID]
	m.mu.RUnlock()

	if ok {
		return volume, nil
	}

	// Try fetching from backend
	volume, err := m.backend.GetVolume(ctx, volumeID)
	if err != nil {
		return nil, err
	}

	// Cache it
	m.mu.Lock()
	m.volumes[volumeID] = volume
	m.mu.Unlock()

	return volume, nil
}

// Delete removes a volume
func (m *Manager) Delete(ctx context.Context, volumeID string) error {
	// Delete from backend
	if err := m.backend.DeleteVolume(ctx, volumeID); err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}

	// Remove from memory
	m.mu.Lock()
	delete(m.volumes, volumeID)
	m.mu.Unlock()

	return nil
}

// List returns all volumes for a workspace
func (m *Manager) List(ctx context.Context, workspaceID string) ([]*types.Volume, error) {
	volumes, err := m.backend.ListVolumes(ctx)
	if err != nil {
		return nil, err
	}

	// Filter by workspace if specified
	if workspaceID != "" {
		filtered := make([]*types.Volume, 0)
		for _, v := range volumes {
			if v.WorkspaceID == workspaceID {
				filtered = append(filtered, v)
			}
		}
		return filtered, nil
	}

	return volumes, nil
}

// LocalBackend implements Backend using local filesystem
type LocalBackend struct {
	basePath string
	mu       sync.RWMutex
	volumes  map[string]*types.Volume
}

// NewLocalBackend creates a local filesystem backend
func NewLocalBackend(basePath string) *LocalBackend {
	return &LocalBackend{
		basePath: basePath,
		volumes:  make(map[string]*types.Volume),
	}
}

// CreateVolume creates a new local volume
func (b *LocalBackend) CreateVolume(ctx context.Context, spec *types.VolumeSpec) (*types.Volume, error) {
	volumeID := fmt.Sprintf("vol-%d", time.Now().Unix())
	
	volume := &types.Volume{
		ID:          volumeID,
		Name:        spec.Name,
		Type:        spec.Type,
		SizeGB:      spec.SizeGB,
		Provider:    "local",
		ProviderRef: fmt.Sprintf("%s/%s", b.basePath, volumeID),
		CreatedAt:   time.Now(),
	}

	b.mu.Lock()
	b.volumes[volumeID] = volume
	b.mu.Unlock()

	return volume, nil
}

// DeleteVolume removes a local volume
func (b *LocalBackend) DeleteVolume(ctx context.Context, volumeID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if _, ok := b.volumes[volumeID]; !ok {
		return fmt.Errorf("volume not found: %s", volumeID)
	}

	delete(b.volumes, volumeID)
	return nil
}

// GetVolume retrieves a local volume
func (b *LocalBackend) GetVolume(ctx context.Context, volumeID string) (*types.Volume, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	volume, ok := b.volumes[volumeID]
	if !ok {
		return nil, fmt.Errorf("volume not found: %s", volumeID)
	}

	return volume, nil
}

// ListVolumes lists all local volumes
func (b *LocalBackend) ListVolumes(ctx context.Context) ([]*types.Volume, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	volumes := make([]*types.Volume, 0, len(b.volumes))
	for _, v := range b.volumes {
		volumes = append(volumes, v)
	}

	return volumes, nil
}
