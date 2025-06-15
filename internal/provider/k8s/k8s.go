package k8s

import (
	"fabric/internal/config"
	"fabric/internal/provider"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Provider struct {
	config config.K8sConfig
	client *kubernetes.Clientset
}

func New(cfg config.K8sConfig) (provider.Provider, error) {
	var config *rest.Config
	var err error

	if cfg.Kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", cfg.Kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get k8s config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &Provider{
		config: cfg,
		client: client,
	}, nil
}

func (k *Provider) Name() string {
	return "k8s"
}

func (k *Provider) ListRegions() ([]provider.Region, error) {
	// K8s is single region (cluster)
	return []provider.Region{
		{ID: "default", Name: "Default Cluster"},
	}, nil
}

func (k *Provider) ListMachineTypes() ([]provider.MachineType, error) {
	// Basic machine types for K8s
	return []provider.MachineType{
		{
			ID:      "small",
			Name:    "Small (2 CPU, 4GB RAM)",
			CPU:     2,
			Memory:  4,
			Storage: 20,
		},
		{
			ID:      "medium",
			Name:    "Medium (4 CPU, 8GB RAM)",
			CPU:     4,
			Memory:  8,
			Storage: 40,
		},
		{
			ID:      "large",
			Name:    "Large (8 CPU, 16GB RAM)",
			CPU:     8,
			Memory:  16,
			Storage: 80,
		},
		{
			ID:      "gpu-small",
			Name:    "GPU Small (4 CPU, 16GB RAM, 1x RTX 4090)",
			CPU:     4,
			Memory:  16,
			Storage: 80,
			GPU: &provider.GPU{
				Type:   "RTX_4090",
				Count:  1,
				Memory: 24,
			},
		},
	}, nil
}

func (k *Provider) CreateInstance(spec provider.InstanceSpec) (*provider.Instance, error) {
	// TODO: Implement K8s pod creation
	return nil, fmt.Errorf("k8s provider not fully implemented")
}

func (k *Provider) DeleteInstance(id string) error {
	// TODO: Implement K8s pod deletion
	return fmt.Errorf("k8s provider not fully implemented")
}

func (k *Provider) GetInstance(id string) (*provider.Instance, error) {
	// TODO: Implement K8s pod status
	return nil, fmt.Errorf("k8s provider not fully implemented")
}

func (k *Provider) GetCostPerHour(machineType string, region string) (float64, error) {
	// Basic cost calculation for K8s
	costs := map[string]float64{
		"small":     0.10,
		"medium":    0.20,
		"large":     0.40,
		"gpu-small": 2.00,
	}

	if cost, exists := costs[machineType]; exists {
		return cost, nil
	}

	return 0, fmt.Errorf("unknown machine type: %s", machineType)
}
