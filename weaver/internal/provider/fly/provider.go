package fly

import (
	"context"
	"fmt"
	"sync"
	"time"

	"weaver/internal/provider"
	"weaver/internal/workload"
)

const Type provider.ProviderType = "fly"

// Config represents Fly.io provider configuration
type Config struct {
	APIToken     string `json:"apiToken"`
	Organization string `json:"organization"`
	Region       string `json:"region"`
}

// Provider implements the Fabric provider interface for Fly.io
type Provider struct {
	name   string
	config Config
	client *Client

	// Cache for regions and sizes
	regions     []*Region
	sizes       []*MachineSize
	cacheExpiry time.Time
	cacheMutex  sync.RWMutex

	// App management
	apps map[string]string // workload ID -> app name
	mu   sync.RWMutex
}

// New creates a new Fly.io provider
func New(name string, config Config) (*Provider, error) {
	if config.APIToken == "" {
		return nil, fmt.Errorf("API token is required")
	}

	client := NewClient(config.APIToken)

	p := &Provider{
		name:   name,
		config: config,
		client: client,
		apps:   make(map[string]string),
	}

	return p, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return p.name
}

// Type returns the provider type
func (p *Provider) Type() provider.ProviderType {
	return Type
}

// CreateWorkload creates a new workload on Fly.io
func (p *Provider) CreateWorkload(ctx context.Context, w *workload.Workload) error {
	// Generate unique app name
	appName := generateAppName(w.Name, w.Namespace)

	// Create app first
	createAppReq := &CreateAppRequest{
		AppName: appName,
		OrgSlug: p.config.Organization,
	}

	if p.config.Region != "" {
		createAppReq.PrimaryRegion = p.config.Region
	}

	_, err := p.client.CreateApp(ctx, createAppReq)
	if err != nil {
		return fmt.Errorf("failed to create app: %w", err)
	}

	// Store app mapping
	p.mu.Lock()
	p.apps[w.ID] = appName
	p.mu.Unlock()

	// Get available regions for machine placement
	regions, err := p.getRegions(ctx)
	if err != nil {
		return fmt.Errorf("failed to get regions: %w", err)
	}

	// Select region
	region := selectRegion(regions, &w.Spec.Placement)

	// Create machine configuration
	machineConfig := MachineConfig{
		Image:    w.Spec.Image,
		Env:      w.Spec.Env,
		Cmd:      w.Spec.Command,
		Guest:    parseGuest(w),
		Services: parseServices(w),
		Mounts:   parseMounts(w),
		Restart:  parseRestartPolicy(w),
	}

	// Create machine
	createMachineReq := &CreateMachineRequest{
		Name:   w.Name,
		Config: machineConfig,
		Region: region,
	}

	machine, err := p.client.CreateMachine(ctx, appName, createMachineReq)
	if err != nil {
		// Clean up app if machine creation fails
		p.client.DeleteApp(ctx, appName)
		p.mu.Lock()
		delete(p.apps, w.ID)
		p.mu.Unlock()
		return fmt.Errorf("failed to create machine: %w", err)
	}

	// Start the machine
	if err := p.client.StartMachine(ctx, appName, machine.ID); err != nil {
		return fmt.Errorf("failed to start machine: %w", err)
	}

	// Update workload with Fly.io specific information
	w.Status.Provider = "fly"
	w.Status.NodeID = machine.Region
	w.Status.Phase = workload.PhasePending

	return nil
}

// GetWorkload retrieves a workload from Fly.io
func (p *Provider) GetWorkload(ctx context.Context, id string) (*workload.Workload, error) {
	p.mu.RLock()
	appName, exists := p.apps[id]
	p.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("workload not found")
	}

	machines, err := p.client.ListMachines(ctx, appName)
	if err != nil {
		return nil, fmt.Errorf("failed to list machines: %w", err)
	}

	if len(machines) == 0 {
		return nil, fmt.Errorf("no machines found for workload")
	}

	// Use the first machine (assuming one machine per workload)
	machine := machines[0]
	w := machineToWorkload(machine, appName)
	w.ID = id

	return w, nil
}

// UpdateWorkload updates a workload on Fly.io
func (p *Provider) UpdateWorkload(ctx context.Context, w *workload.Workload) error {
	p.mu.RLock()
	appName, exists := p.apps[w.ID]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workload not found")
	}

	machines, err := p.client.ListMachines(ctx, appName)
	if err != nil {
		return fmt.Errorf("failed to list machines: %w", err)
	}

	if len(machines) == 0 {
		return fmt.Errorf("no machines found for workload")
	}

	machine := machines[0]

	// Update machine configuration
	machineConfig := MachineConfig{
		Image:    w.Spec.Image,
		Env:      w.Spec.Env,
		Cmd:      w.Spec.Command,
		Guest:    parseGuest(w),
		Services: parseServices(w),
		Mounts:   parseMounts(w),
		Restart:  parseRestartPolicy(w),
	}

	updateReq := &UpdateMachineRequest{
		Config: machineConfig,
	}

	_, err = p.client.UpdateMachine(ctx, appName, machine.ID, updateReq)
	if err != nil {
		return fmt.Errorf("failed to update machine: %w", err)
	}

	return nil
}

// DeleteWorkload deletes a workload from Fly.io
func (p *Provider) DeleteWorkload(ctx context.Context, id string) error {
	p.mu.RLock()
	appName, exists := p.apps[id]
	p.mu.RUnlock()

	if !exists {
		return fmt.Errorf("workload not found")
	}

	// Delete the entire app (which deletes all machines)
	err := p.client.DeleteApp(ctx, appName)
	if err != nil {
		return fmt.Errorf("failed to delete app: %w", err)
	}

	// Remove from mapping
	p.mu.Lock()
	delete(p.apps, id)
	p.mu.Unlock()

	return nil
}

// ListWorkloads lists all workloads in a namespace
func (p *Provider) ListWorkloads(ctx context.Context, namespace string) ([]*workload.Workload, error) {
	var workloads []*workload.Workload

	p.mu.RLock()
	appMappings := make(map[string]string)
	for workloadID, appName := range p.apps {
		appMappings[workloadID] = appName
	}
	p.mu.RUnlock()

	for workloadID, appName := range appMappings {
		machines, err := p.client.ListMachines(ctx, appName)
		if err != nil {
			continue // Skip failed apps
		}

		for _, machine := range machines {
			w := machineToWorkload(machine, appName)
			w.ID = workloadID
			if namespace == "" || w.Namespace == namespace {
				workloads = append(workloads, w)
			}
		}
	}

	return workloads, nil
}

// GetAvailableResources returns available resources on Fly.io
func (p *Provider) GetAvailableResources(ctx context.Context) (*provider.ResourceAvailability, error) {
	regions, err := p.getRegions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get regions: %w", err)
	}

	sizes, err := p.getSizes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sizes: %w", err)
	}

	// Convert regions
	var regionInfos []provider.RegionInfo
	for _, region := range regions {
		regionInfo := provider.RegionInfo{
			Name:        region.Code,
			DisplayName: region.Name,
			Available:   region.GatewayAvailable,
		}

		// Add GPU types if available
		if !region.RequiresPaidPlan {
			regionInfo.GPUTypes = []string{"a100-pcie-40gb", "a100-sxm4-80gb"}
		}

		regionInfos = append(regionInfos, regionInfo)
	}

	// Calculate total resources (approximate)
	totalCPUs := 0
	totalMemoryGB := 0
	gpuTypes := make(map[string]provider.GPUTypeInfo)

	for _, size := range sizes {
		totalCPUs += size.CPUs * 100 // Assume 100 instances per size
		totalMemoryGB += (size.MemoryMB / 1024) * 100
	}

	// Add GPU information
	gpuTypes["a100-pcie-40gb"] = provider.GPUTypeInfo{
		Name:         "NVIDIA A100 PCIe 40GB",
		Memory:       "40GB",
		Total:        50,
		Available:    25,
		PricePerHour: 2.0,
	}

	gpuTypes["a100-sxm4-80gb"] = provider.GPUTypeInfo{
		Name:         "NVIDIA A100 SXM4 80GB",
		Memory:       "80GB",
		Total:        20,
		Available:    10,
		PricePerHour: 4.0,
	}

	return &provider.ResourceAvailability{
		CPU: provider.ResourcePool{
			Total:     fmt.Sprintf("%d", totalCPUs),
			Available: fmt.Sprintf("%d", totalCPUs/2),
			Used:      fmt.Sprintf("%d", totalCPUs/2),
		},
		Memory: provider.ResourcePool{
			Total:     fmt.Sprintf("%dGi", totalMemoryGB),
			Available: fmt.Sprintf("%dGi", totalMemoryGB/2),
			Used:      fmt.Sprintf("%dGi", totalMemoryGB/2),
		},
		GPU: provider.GPUPool{
			Types: gpuTypes,
		},
		Regions: regionInfos,
	}, nil
}

// GetPricing returns pricing information for Fly.io
func (p *Provider) GetPricing(ctx context.Context) (*provider.PricingInfo, error) {
	return &provider.PricingInfo{
		Currency: "USD",
		CPU: provider.PricePerUnit{
			Amount: 0.02,
			Unit:   "hour",
		},
		Memory: provider.PricePerUnit{
			Amount: 0.01,
			Unit:   "hour", // per GB
		},
		GPU: map[string]provider.PricePerUnit{
			"a100-pcie-40gb": {
				Amount: 2.0,
				Unit:   "hour",
			},
			"a100-sxm4-80gb": {
				Amount: 4.0,
				Unit:   "hour",
			},
		},
		Storage: provider.PricePerUnit{
			Amount: 0.15,
			Unit:   "month", // per GB
		},
		Network: provider.NetworkPricing{
			Ingress: provider.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
			Egress: provider.PricePerUnit{
				Amount: 0.02,
				Unit:   "gb",
			},
			Internal: provider.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
		},
	}, nil
}

// HealthCheck checks the health of the Fly.io provider
func (p *Provider) HealthCheck(ctx context.Context) error {
	return p.client.GetHealth(ctx)
}

// GetStatus returns the current status of the provider
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	// Check health
	healthy := p.client.GetHealth(ctx) == nil

	// Get regions for status
	regions, err := p.getRegions(ctx)
	if err != nil {
		return &provider.ProviderStatus{
			Available: false,
			Message:   fmt.Sprintf("Failed to get regions: %v", err),
		}, nil
	}

	var regionStatuses []provider.RegionStatus
	for _, region := range regions {
		regionStatuses = append(regionStatuses, provider.RegionStatus{
			Name:      region.Code,
			Available: region.GatewayAvailable,
			Load:      0.5, // Placeholder
			Latency:   50,  // Placeholder
		})
	}

	// Count active workloads
	p.mu.RLock()
	activeWorkloads := len(p.apps)
	p.mu.RUnlock()

	return &provider.ProviderStatus{
		Available: healthy,
		Message:   "Fly.io provider operational",
		Regions:   regionStatuses,
		Metrics: provider.ProviderMetrics{
			ActiveWorkloads:  activeWorkloads,
			TotalWorkloads:   activeWorkloads,
			SuccessRate:      0.95,
			AverageStartTime: 30,
			AverageLatency:   50,
		},
	}, nil
}

// getRegions retrieves and caches region information
func (p *Provider) getRegions(ctx context.Context) ([]*Region, error) {
	p.cacheMutex.RLock()
	if p.regions != nil && time.Now().Before(p.cacheExpiry) {
		defer p.cacheMutex.RUnlock()
		return p.regions, nil
	}
	p.cacheMutex.RUnlock()

	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if p.regions != nil && time.Now().Before(p.cacheExpiry) {
		return p.regions, nil
	}

	regions, err := p.client.ListRegions(ctx)
	if err != nil {
		return nil, err
	}

	p.regions = regions
	p.cacheExpiry = time.Now().Add(5 * time.Minute)

	return regions, nil
}

// getSizes retrieves and caches machine size information
func (p *Provider) getSizes(ctx context.Context) ([]*MachineSize, error) {
	p.cacheMutex.RLock()
	if p.sizes != nil && time.Now().Before(p.cacheExpiry) {
		defer p.cacheMutex.RUnlock()
		return p.sizes, nil
	}
	p.cacheMutex.RUnlock()

	p.cacheMutex.Lock()
	defer p.cacheMutex.Unlock()

	// Double-check after acquiring write lock
	if p.sizes != nil && time.Now().Before(p.cacheExpiry) {
		return p.sizes, nil
	}

	sizes, err := p.client.GetMachineSizes(ctx)
	if err != nil {
		return nil, err
	}

	p.sizes = sizes
	p.cacheExpiry = time.Now().Add(5 * time.Minute)

	return sizes, nil
}
