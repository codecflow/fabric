package workload

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/codecflow/fabric/pkg/image"
	"github.com/codecflow/fabric/pkg/network"
	"github.com/codecflow/fabric/pkg/provider"
	"github.com/codecflow/fabric/pkg/secret"
	"github.com/codecflow/fabric/pkg/types"
	"github.com/codecflow/fabric/pkg/volume"
)

// Manager orchestrates workload deployment across all services
type Manager struct {
	mu sync.RWMutex
	
	// Core services
	imageService   *image.Service
	volumeManager  *volume.Manager
	secretManager  *secret.Manager
	networkManager *network.Manager
	
	// Provider registry
	providers map[string]provider.WorkloadProvider
	scheduler *Scheduler
	
	// State
	workloads   map[string]*types.Workload
	deployments map[string]*types.Deployment
}

// NewManager creates a new workload manager
func NewManager(
	imageService *image.Service,
	volumeManager *volume.Manager,
	secretManager *secret.Manager,
	networkManager *network.Manager,
) *Manager {
	return &Manager{
		imageService:   imageService,
		volumeManager:  volumeManager,
		secretManager:  secretManager,
		networkManager: networkManager,
		providers:      make(map[string]provider.WorkloadProvider),
		scheduler:      NewScheduler(),
		workloads:      make(map[string]*types.Workload),
		deployments:    make(map[string]*types.Deployment),
	}
}

// RegisterProvider registers a provider with the manager
func (m *Manager) RegisterProvider(p provider.WorkloadProvider) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	name := p.Name()
	if _, exists := m.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}
	
	m.providers[name] = p
	m.scheduler.AddProvider(p)
	
	return nil
}

// Create creates and deploys a workload
func (m *Manager) Create(ctx context.Context, workload *types.Workload) (*types.Deployment, error) {
	// 1. Validate workload
	if err := m.validateWorkload(workload); err != nil {
		return nil, fmt.Errorf("invalid workload: %w", err)
	}
	
	// Store workload
	m.mu.Lock()
	m.workloads[workload.ID] = workload
	m.mu.Unlock()
	
	// 2. Build image if needed
	if err := m.prepareImage(ctx, workload); err != nil {
		return nil, fmt.Errorf("failed to prepare image: %w", err)
	}
	
	// 3. Prepare secrets
	if err := m.prepareSecrets(ctx, workload); err != nil {
		return nil, fmt.Errorf("failed to prepare secrets: %w", err)
	}
	
	// 4. Prepare volumes
	if err := m.prepareVolumes(ctx, workload); err != nil {
		return nil, fmt.Errorf("failed to prepare volumes: %w", err)
	}
	
	// 5. Select provider via scheduler
	selectedProvider, err := m.scheduler.SelectProvider(workload)
	if err != nil {
		return nil, fmt.Errorf("failed to select provider: %w", err)
	}
	
	// 6. Deploy to provider
	deployment, err := selectedProvider.Deploy(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("deployment failed: %w", err)
	}
	
	// 7. Register with network manager
	if len(deployment.Endpoints) > 0 {
		_, err := m.networkManager.RegisterService(ctx, deployment.ID, workload.Name, deployment.Endpoints)
		if err != nil {
			// Log but don't fail
			fmt.Printf("warning: failed to register service: %v\n", err)
		}
	}
	
	// Store deployment
	m.mu.Lock()
	m.deployments[deployment.ID] = deployment
	m.mu.Unlock()
	
	return deployment, nil
}

// Get retrieves a workload by ID
func (m *Manager) Get(ctx context.Context, workloadID string) (*types.Workload, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	workload, ok := m.workloads[workloadID]
	if !ok {
		return nil, fmt.Errorf("workload not found: %s", workloadID)
	}
	
	return workload, nil
}

// GetDeployment retrieves a deployment by ID
func (m *Manager) GetDeployment(ctx context.Context, deploymentID string) (*types.Deployment, error) {
	m.mu.RLock()
	deployment, ok := m.deployments[deploymentID]
	m.mu.RUnlock()
	
	if ok {
		return deployment, nil
	}
	
	// Try querying providers
	for _, p := range m.providers {
		deployment, err := p.Status(ctx, deploymentID)
		if err == nil {
			return deployment, nil
		}
	}
	
	return nil, fmt.Errorf("deployment not found: %s", deploymentID)
}

// Stop stops a deployment
func (m *Manager) Stop(ctx context.Context, deploymentID string) error {
	deployment, err := m.GetDeployment(ctx, deploymentID)
	if err != nil {
		return err
	}
	
	// Get provider
	p, ok := m.providers[deployment.Provider]
	if !ok {
		return fmt.Errorf("provider not found: %s", deployment.Provider)
	}
	
	// Stop via provider
	if err := p.Stop(ctx, deploymentID); err != nil {
		return fmt.Errorf("failed to stop deployment: %w", err)
	}
	
	// Unregister from network
	m.networkManager.UnregisterService(ctx, deploymentID)
	
	// Update status
	m.mu.Lock()
	if d, ok := m.deployments[deploymentID]; ok {
		d.Status = types.DeploymentStatusStopped
		d.StoppedAt = &time.Time{}
		*d.StoppedAt = time.Now()
	}
	m.mu.Unlock()
	
	return nil
}

// Delete deletes a deployment
func (m *Manager) Delete(ctx context.Context, deploymentID string) error {
	deployment, err := m.GetDeployment(ctx, deploymentID)
	if err != nil {
		return err
	}
	
	// Get provider
	p, ok := m.providers[deployment.Provider]
	if !ok {
		return fmt.Errorf("provider not found: %s", deployment.Provider)
	}
	
	// Delete via provider
	if err := p.Delete(ctx, deploymentID); err != nil {
		return fmt.Errorf("failed to delete deployment: %w", err)
	}
	
	// Unregister from network
	m.networkManager.UnregisterService(ctx, deploymentID)
	
	// Remove from state
	m.mu.Lock()
	delete(m.deployments, deploymentID)
	m.mu.Unlock()
	
	return nil
}

// List lists all deployments
func (m *Manager) List(ctx context.Context, workspaceID string) ([]*types.Deployment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	deployments := make([]*types.Deployment, 0)
	for _, d := range m.deployments {
		if workspaceID == "" || d.WorkloadID == workspaceID {
			deployments = append(deployments, d)
		}
	}
	
	return deployments, nil
}

// Helper methods

func (m *Manager) validateWorkload(w *types.Workload) error {
	if w.ID == "" {
		return fmt.Errorf("workload ID is required")
	}
	if w.Name == "" {
		return fmt.Errorf("workload name is required")
	}
	if w.Image == nil {
		return fmt.Errorf("image specification is required")
	}
	return nil
}

func (m *Manager) prepareImage(ctx context.Context, workload *types.Workload) error {
	// If image is already specified, nothing to do
	if workload.Image.Image != "" {
		return nil
	}
	
	// Build image from spec
	result, err := m.imageService.BuildImage(ctx, workload.Image)
	if err != nil {
		return err
	}
	
	// Update workload with built image
	workload.Image.Image = result.ImageID
	workload.Image.Tag = result.ImageTag
	
	return nil
}

func (m *Manager) prepareSecrets(ctx context.Context, workload *types.Workload) error {
	if len(workload.Secrets) == 0 {
		return nil
	}
	
	// Inject secrets into environment
	if workload.Env == nil {
		workload.Env = make(map[string]string)
	}
	
	return m.secretManager.InjectIntoEnv(ctx, workload.Env, workload.Secrets, workload.WorkspaceID)
}

func (m *Manager) prepareVolumes(ctx context.Context, workload *types.Workload) error {
	// For now, just validate that volumes exist
	// In production, this would create volumes if needed
	for _, mount := range workload.Volumes {
		_, err := m.volumeManager.Get(ctx, mount.Name)
		if err != nil {
			return fmt.Errorf("volume %s not found: %w", mount.Name, err)
		}
	}
	return nil
}
