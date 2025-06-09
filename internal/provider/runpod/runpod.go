package runpod

import (
	"fabric/internal/config"
	"fabric/internal/provider"
	"fmt"
)

type Provider struct {
	config config.RunPodConfig
}

func New(cfg config.RunPodConfig) (provider.Provider, error) {
	return &Provider{config: cfg}, nil
}

func (r *Provider) Name() string {
	return "runpod"
}

func (r *Provider) ListRegions() ([]provider.Region, error) {
	// RunPod regions
	return []provider.Region{
		{ID: "us-east-1", Name: "US East 1"},
		{ID: "us-west-1", Name: "US West 1"},
		{ID: "eu-west-1", Name: "EU West 1"},
		{ID: "asia-southeast-1", Name: "Asia Southeast 1"},
	}, nil
}

func (r *Provider) ListMachineTypes() ([]provider.MachineType, error) {
	// RunPod GPU instances based on Nebulous accelerator mapping
	return []provider.MachineType{
		{
			ID:      "rtx-4090",
			Name:    "RTX 4090",
			CPU:     8,
			Memory:  32,
			Storage: 100,
			GPU: &provider.GPU{
				Type:   "RTX_4090",
				Count:  1,
				Memory: 24,
			},
		},
		{
			ID:      "rtx-4080",
			Name:    "RTX 4080",
			CPU:     6,
			Memory:  24,
			Storage: 80,
			GPU: &provider.GPU{
				Type:   "RTX_4080",
				Count:  1,
				Memory: 16,
			},
		},
		{
			ID:      "a100-80gb",
			Name:    "A100 80GB",
			CPU:     16,
			Memory:  64,
			Storage: 200,
			GPU: &provider.GPU{
				Type:   "A100_SXM",
				Count:  1,
				Memory: 80,
			},
		},
		{
			ID:      "h100-80gb",
			Name:    "H100 80GB",
			CPU:     20,
			Memory:  80,
			Storage: 300,
			GPU: &provider.GPU{
				Type:   "H100_SXM",
				Count:  1,
				Memory: 80,
			},
		},
	}, nil
}

func (r *Provider) CreateInstance(spec provider.InstanceSpec) (*provider.Instance, error) {
	// TODO: Implement RunPod API integration
	return nil, fmt.Errorf("runpod provider not fully implemented")
}

func (r *Provider) DeleteInstance(id string) error {
	// TODO: Implement RunPod instance deletion
	return fmt.Errorf("runpod provider not fully implemented")
}

func (r *Provider) GetInstance(id string) (*provider.Instance, error) {
	// TODO: Implement RunPod instance status
	return nil, fmt.Errorf("runpod provider not fully implemented")
}

func (r *Provider) GetCostPerHour(machineType string, region string) (float64, error) {
	// RunPod pricing (approximate)
	costs := map[string]float64{
		"rtx-4090":  1.50,
		"rtx-4080":  1.20,
		"a100-80gb": 3.50,
		"h100-80gb": 8.00,
	}

	if cost, exists := costs[machineType]; exists {
		return cost, nil
	}

	return 0, fmt.Errorf("unknown machine type: %s", machineType)
}
