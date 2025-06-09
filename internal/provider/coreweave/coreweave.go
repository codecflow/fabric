package coreweave

import (
	"fabric/internal/config"
	"fabric/internal/provider"
	"fmt"
)

type Provider struct {
	config config.CoreWeaveConfig
}

func New(cfg config.CoreWeaveConfig) (provider.Provider, error) {
	return &Provider{config: cfg}, nil
}

func (c *Provider) Name() string {
	return "coreweave"
}

func (c *Provider) ListRegions() ([]provider.Region, error) {
	// CoreWeave regions
	return []provider.Region{
		{ID: "ord1", Name: "Chicago"},
		{ID: "lga1", Name: "New York"},
		{ID: "las1", Name: "Las Vegas"},
	}, nil
}

func (c *Provider) ListMachineTypes() ([]provider.MachineType, error) {
	// CoreWeave GPU instances
	return []provider.MachineType{
		{
			ID:      "rtx-a6000",
			Name:    "RTX A6000",
			CPU:     8,
			Memory:  32,
			Storage: 100,
			GPU: &provider.GPU{
				Type:   "RTX_A6000",
				Count:  1,
				Memory: 48,
			},
		},
		{
			ID:      "a100-40gb",
			Name:    "A100 40GB",
			CPU:     12,
			Memory:  48,
			Storage: 150,
			GPU: &provider.GPU{
				Type:   "A100_PCIe",
				Count:  1,
				Memory: 40,
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

func (c *Provider) CreateInstance(spec provider.InstanceSpec) (*provider.Instance, error) {
	// TODO: Implement CoreWeave API integration
	return nil, fmt.Errorf("coreweave provider not fully implemented")
}

func (c *Provider) DeleteInstance(id string) error {
	// TODO: Implement CoreWeave instance deletion
	return fmt.Errorf("coreweave provider not fully implemented")
}

func (c *Provider) GetInstance(id string) (*provider.Instance, error) {
	// TODO: Implement CoreWeave instance status
	return nil, fmt.Errorf("coreweave provider not fully implemented")
}

func (c *Provider) GetCostPerHour(machineType string, region string) (float64, error) {
	// CoreWeave pricing (approximate)
	costs := map[string]float64{
		"rtx-a6000": 1.80,
		"a100-40gb": 2.50,
		"a100-80gb": 3.20,
		"h100-80gb": 7.50,
	}

	if cost, exists := costs[machineType]; exists {
		return cost, nil
	}

	return 0, fmt.Errorf("unknown machine type: %s", machineType)
}
