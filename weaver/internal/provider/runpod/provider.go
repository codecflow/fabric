package runpod

import (
	"context"
	"fmt"
	"weaver/internal/provider"
	"weaver/internal/workload"
)

const Type provider.ProviderType = "runpod"

// Provider implements the Provider interface for RunPod
type Provider struct {
	client *Client
	name   string
}

// Config represents RunPod-specific configuration
type Config struct {
	APIKey string `json:"apiKey"`
}

// New creates a new RunPod provider
func New(name string, config Config) (*Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("RunPod API key is required")
	}

	client := NewClient(config.APIKey)

	return &Provider{
		client: client,
		name:   name,
	}, nil
}

func NewFromConfig(name string, config map[string]string) (provider.Provider, error) {
	apiKey, exists := config["apiKey"]
	if !exists || apiKey == "" {
		return nil, fmt.Errorf("RunPod API key is required")
	}

	return New(name, Config{APIKey: apiKey})
}

func (p *Provider) Name() string {
	return p.name
}

func (p *Provider) Type() provider.ProviderType {
	return Type
}

// CreateWorkload creates a new workload on RunPod
func (p *Provider) CreateWorkload(ctx context.Context, w *workload.Workload) error {
	req := &CreatePodRequest{
		Name:          w.Name,
		ImageName:     w.Spec.Image,
		GPUTypeID:     p.selectGPUType(w.Spec.Resources.GPU),
		GPUCount:      p.parseGPUCount(w.Spec.Resources.GPU),
		VCPUCount:     p.parseCPUCount(w.Spec.Resources.CPU),
		MemoryInGB:    p.parseMemoryGB(w.Spec.Resources.Memory),
		ContainerDisk: 20, // Default 20GB
		Env:           w.Spec.Env,
		Ports:         p.formatPorts(w.Spec.Ports),
	}

	_, err := p.client.CreatePod(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create RunPod workload: %w", err)
	}

	return nil
}

// GetWorkload retrieves a workload from RunPod
func (p *Provider) GetWorkload(ctx context.Context, id string) (*workload.Workload, error) {
	pod, err := p.client.GetPod(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get RunPod workload: %w", err)
	}

	return p.toWorkload(pod), nil
}

// UpdateWorkload updates a workload on RunPod
func (p *Provider) UpdateWorkload(ctx context.Context, w *workload.Workload) error {
	return fmt.Errorf("RunPod does not support updating running workloads")
}

// DeleteWorkload deletes a workload from RunPod
func (p *Provider) DeleteWorkload(ctx context.Context, id string) error {
	err := p.client.TerminatePod(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to delete RunPod workload: %w", err)
	}

	return nil
}

// ListWorkloads lists workloads on RunPod
func (p *Provider) ListWorkloads(ctx context.Context, namespace string) ([]*workload.Workload, error) {
	pods, err := p.client.GetPods(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list RunPod workloads: %w", err)
	}

	var workloads []*workload.Workload
	for _, pod := range pods {
		w := p.toWorkload(pod)
		workloads = append(workloads, w)
	}

	return workloads, nil
}

// GetAvailableResources returns available resources on RunPod
func (p *Provider) GetAvailableResources(ctx context.Context) (*provider.ResourceAvailability, error) {
	gpuTypes, err := p.client.GetGPUTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get GPU types: %w", err)
	}

	gpuTypeMap := make(map[string]provider.GPUTypeInfo)
	for _, gpu := range gpuTypes {
		gpuTypeMap[gpu.ID] = provider.GPUTypeInfo{
			Name:         gpu.DisplayName,
			Memory:       fmt.Sprintf("%dGi", gpu.MemoryInGB),
			Total:        100, // Mock data
			Available:    80,  // Mock data
			PricePerHour: gpu.LowestPrice.UninterruptiblePrice,
		}
	}

	return &provider.ResourceAvailability{
		CPU: provider.ResourcePool{
			Total:     "unlimited",
			Available: "unlimited",
			Used:      "0",
		},
		Memory: provider.ResourcePool{
			Total:     "unlimited",
			Available: "unlimited",
			Used:      "0",
		},
		GPU: provider.GPUPool{
			Types: gpuTypeMap,
		},
		Regions: []provider.RegionInfo{
			{
				Name:        "us-east-1",
				DisplayName: "US East",
				Available:   true,
				GPUTypes:    []string{},
			},
		},
	}, nil
}

// GetPricing returns pricing information for RunPod
func (p *Provider) GetPricing(ctx context.Context) (*provider.PricingInfo, error) {
	gpuTypes, err := p.client.GetGPUTypes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing: %w", err)
	}

	gpuPricing := make(map[string]provider.PricePerUnit)
	for _, gpu := range gpuTypes {
		gpuPricing[gpu.ID] = provider.PricePerUnit{
			Amount: gpu.LowestPrice.UninterruptiblePrice,
			Unit:   "hour",
		}
	}

	return &provider.PricingInfo{
		Currency: "USD",
		CPU: provider.PricePerUnit{
			Amount: 0.0001,
			Unit:   "hour",
		},
		Memory: provider.PricePerUnit{
			Amount: 0.00005,
			Unit:   "hour",
		},
		GPU: gpuPricing,
		Storage: provider.PricePerUnit{
			Amount: 0.10,
			Unit:   "month",
		},
		Network: provider.NetworkPricing{
			Ingress:  provider.PricePerUnit{Amount: 0.0, Unit: "gb"},
			Egress:   provider.PricePerUnit{Amount: 0.09, Unit: "gb"},
			Internal: provider.PricePerUnit{Amount: 0.0, Unit: "gb"},
		},
	}, nil
}

// HealthCheck checks if RunPod API is accessible
func (p *Provider) HealthCheck(ctx context.Context) error {
	_, err := p.client.GetUser(ctx)
	if err != nil {
		return fmt.Errorf("RunPod health check failed: %w", err)
	}
	return nil
}

// GetStatus returns the current status of the RunPod provider
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	available := true
	message := ""

	if err := p.HealthCheck(ctx); err != nil {
		available = false
		message = err.Error()
	}

	workloads, _ := p.ListWorkloads(ctx, "")
	activeWorkloads := 0
	for _, w := range workloads {
		if w.Status.Phase == workload.PhaseRunning {
			activeWorkloads++
		}
	}

	return &provider.ProviderStatus{
		Available: available,
		Message:   message,
		Regions: []provider.RegionStatus{
			{
				Name:      "us-east-1",
				Available: available,
				Load:      0.3,
				Latency:   50,
			},
		},
		Metrics: provider.ProviderMetrics{
			ActiveWorkloads:  activeWorkloads,
			TotalWorkloads:   len(workloads),
			SuccessRate:      0.95,
			AverageStartTime: 45,
			AverageLatency:   50,
		},
	}, nil
}
