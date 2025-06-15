package nosana

import (
	"context"
	"fmt"
	"weaver/internal/provider"
	"weaver/internal/workload"
)

const Type provider.ProviderType = "nosana"

// Provider implements the Provider interface for Nosana
type Provider struct {
	client *Client
	name   string
}

// Config represents Nosana-specific configuration
type Config struct {
	APIKey  string `json:"apiKey"`
	Network string `json:"network,omitempty"` // "mainnet", "testnet"
}

// New creates a new Nosana provider
func New(name string, config Config) (*Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("Nosana API key is required")
	}

	client := NewClient(config.APIKey)

	return &Provider{
		client: client,
		name:   name,
	}, nil
}

// NewFromConfig creates a new Nosana provider from a config map
func NewFromConfig(name string, config map[string]string) (provider.Provider, error) {
	apiKey, exists := config["apiKey"]
	if !exists || apiKey == "" {
		return nil, fmt.Errorf("Nosana API key is required")
	}

	network := config["network"]
	if network == "" {
		network = "mainnet"
	}

	return New(name, Config{
		APIKey:  apiKey,
		Network: network,
	})
}

// Name returns the provider name
func (p *Provider) Name() string {
	return p.name
}

// Type returns the provider type
func (p *Provider) Type() provider.ProviderType {
	return Type
}

// CreateWorkload creates a new workload on Nosana
func (p *Provider) CreateWorkload(ctx context.Context, w *workload.Workload) error {
	resources := parseResources(w)

	// Get available markets to select one
	markets, err := p.client.ListMarkets(ctx)
	if err != nil {
		return fmt.Errorf("failed to get markets: %w", err)
	}

	market := p.selectMarket(markets, resources)
	if market == nil {
		return fmt.Errorf("no suitable market found")
	}

	price := p.calculatePrice(resources, market)

	req := &JobRequest{
		Name:      w.Name,
		Image:     w.Spec.Image,
		Command:   w.Spec.Command,
		Args:      w.Spec.Args,
		Env:       w.Spec.Env,
		Resources: resources,
		Price:     price,
		Market:    market.ID,
		Network:   "mainnet",
	}

	_, err = p.client.CreateJob(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create Nosana job: %w", err)
	}

	return nil
}

// GetWorkload retrieves a workload from Nosana
func (p *Provider) GetWorkload(ctx context.Context, id string) (*workload.Workload, error) {
	job, err := p.client.GetJob(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get Nosana job: %w", err)
	}

	return p.nosanaJobToWorkload(job), nil
}

// UpdateWorkload updates a workload on Nosana
func (p *Provider) UpdateWorkload(ctx context.Context, w *workload.Workload) error {
	return fmt.Errorf("Nosana does not support updating running jobs")
}

// DeleteWorkload deletes a workload from Nosana
func (p *Provider) DeleteWorkload(ctx context.Context, id string) error {
	err := p.client.CancelJob(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to cancel Nosana job: %w", err)
	}

	return nil
}

// ListWorkloads lists workloads on Nosana
func (p *Provider) ListWorkloads(ctx context.Context, namespace string) ([]*workload.Workload, error) {
	jobs, err := p.client.ListJobs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list Nosana jobs: %w", err)
	}

	var workloads []*workload.Workload
	for _, job := range jobs {
		w := p.nosanaJobToWorkload(job)
		workloads = append(workloads, w)
	}

	return workloads, nil
}

// GetAvailableResources returns available resources on Nosana
func (p *Provider) GetAvailableResources(ctx context.Context) (*provider.ResourceAvailability, error) {
	nodes, err := p.client.ListNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes: %w", err)
	}

	// Aggregate node capabilities
	totalCPU := 0
	totalMemoryGB := 0
	gpuTypes := make(map[string]provider.GPUTypeInfo)

	for _, node := range nodes {
		if node.Status == NodeStatusOnline {
			totalCPU += node.Capabilities.CPU.Cores
			// Parse memory (simplified)
			if memGB := p.parseMemoryToGB(node.Capabilities.Memory); memGB > 0 {
				totalMemoryGB += memGB
			}

			// Add GPU types
			if node.Capabilities.GPU.Count > 0 {
				gpuTypes[node.Capabilities.GPU.Model] = provider.GPUTypeInfo{
					Name:         node.Capabilities.GPU.Model,
					Memory:       node.Capabilities.GPU.Memory,
					Total:        node.Capabilities.GPU.Count,
					Available:    node.Capabilities.GPU.Count, // Simplified
					PricePerHour: node.Pricing.GPU,
				}
			}
		}
	}

	return &provider.ResourceAvailability{
		CPU: provider.ResourcePool{
			Total:     fmt.Sprintf("%d", totalCPU),
			Available: fmt.Sprintf("%d", totalCPU*80/100), // Assume 80% available
			Used:      fmt.Sprintf("%d", totalCPU*20/100),
		},
		Memory: provider.ResourcePool{
			Total:     fmt.Sprintf("%dGi", totalMemoryGB),
			Available: fmt.Sprintf("%dGi", totalMemoryGB*80/100),
			Used:      fmt.Sprintf("%dGi", totalMemoryGB*20/100),
		},
		GPU: provider.GPUPool{
			Types: gpuTypes,
		},
		Regions: []provider.RegionInfo{
			{
				Name:        "global",
				DisplayName: "Global Network",
				Available:   true,
				GPUTypes:    []string{},
			},
		},
	}, nil
}

// GetPricing returns pricing information for Nosana
func (p *Provider) GetPricing(ctx context.Context) (*provider.PricingInfo, error) {
	markets, err := p.client.ListMarkets(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get pricing: %w", err)
	}

	// Use average pricing from markets
	avgMinPrice := 0.0
	activeMarkets := 0
	for _, market := range markets {
		if market.Active {
			avgMinPrice += market.MinPrice
			activeMarkets++
		}
	}

	if activeMarkets > 0 {
		avgMinPrice /= float64(activeMarkets)
	}

	return &provider.PricingInfo{
		Currency: "USD",
		CPU: provider.PricePerUnit{
			Amount: 0.02,
			Unit:   "hour",
		},
		Memory: provider.PricePerUnit{
			Amount: 0.01,
			Unit:   "hour",
		},
		GPU: map[string]provider.PricePerUnit{
			"default": {
				Amount: avgMinPrice,
				Unit:   "hour",
			},
		},
		Storage: provider.PricePerUnit{
			Amount: 0.001,
			Unit:   "hour",
		},
		Network: provider.NetworkPricing{
			Ingress:  provider.PricePerUnit{Amount: 0.0, Unit: "gb"},
			Egress:   provider.PricePerUnit{Amount: 0.05, Unit: "gb"},
			Internal: provider.PricePerUnit{Amount: 0.0, Unit: "gb"},
		},
	}, nil
}

// HealthCheck checks if Nosana API is accessible
func (p *Provider) HealthCheck(ctx context.Context) error {
	err := p.client.GetHealth(ctx)
	if err != nil {
		return fmt.Errorf("Nosana health check failed: %w", err)
	}
	return nil
}

// GetStatus returns the current status of the Nosana provider
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
				Name:      "global",
				Available: available,
				Load:      0.4,
				Latency:   100,
			},
		},
		Metrics: provider.ProviderMetrics{
			ActiveWorkloads:  activeWorkloads,
			TotalWorkloads:   len(workloads),
			SuccessRate:      0.90,
			AverageStartTime: 60,
			AverageLatency:   100,
		},
	}, nil
}
