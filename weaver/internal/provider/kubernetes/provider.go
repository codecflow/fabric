package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/codecflow/fabric/pkg/workload"
	"github.com/codecflow/fabric/weaver/internal/provider"
)

// Provider implements the Provider interface for Kubernetes
type Provider struct {
	client    *Client
	namespace string
	name      string
}

// New creates a new Kubernetes provider
func New(name string, config Config) (*Provider, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	namespace := config.Namespace
	if namespace == "" {
		namespace = "default"
	}

	return &Provider{
		client:    client,
		namespace: namespace,
		name:      name,
	}, nil
}

// NewFromConfig creates a new Kubernetes provider from a config map
func NewFromConfig(name string, config map[string]string) (provider.Provider, error) {
	kubeconfig := config["kubeconfig"]
	namespace := config["namespace"]
	inCluster := config["inCluster"] == "true"

	return New(name, Config{
		Kubeconfig: kubeconfig,
		Namespace:  namespace,
		InCluster:  inCluster,
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

// CreateWorkload creates a new workload in Kubernetes
func (p *Provider) CreateWorkload(ctx context.Context, w *workload.Workload) error {
	pod := toPod(w, p.namespace)

	_, err := p.client.CoreV1().Pods(p.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	return nil
}

// GetWorkload retrieves a workload from Kubernetes
func (p *Provider) GetWorkload(ctx context.Context, id string) (*workload.Workload, error) {
	pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("fabric.workload.id=%s", id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("workload not found")
	}

	pod := pods.Items[0]
	return toWorkload(&pod, p.name), nil
}

// UpdateWorkload updates a workload in Kubernetes
func (p *Provider) UpdateWorkload(ctx context.Context, w *workload.Workload) error {
	pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("fabric.workload.id=%s", w.ID),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("workload not found")
	}

	pod := &pods.Items[0]
	if pod.Labels == nil {
		pod.Labels = make(map[string]string)
	}
	pod.Labels["fabric.workload.name"] = w.Name

	_, err = p.client.CoreV1().Pods(p.namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}

// DeleteWorkload deletes a workload from Kubernetes
func (p *Provider) DeleteWorkload(ctx context.Context, id string) error {
	pods, err := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("fabric.workload.id=%s", id),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		err := p.client.CoreV1().Pods(p.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete pod %s: %w", pod.Name, err)
		}
	}

	return nil
}

// ListWorkloads lists workloads in a namespace
func (p *Provider) ListWorkloads(ctx context.Context, namespace string) ([]*workload.Workload, error) {
	if namespace == "" {
		namespace = p.namespace
	}

	pods, err := p.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "fabric.workload.id",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var workloads []*workload.Workload
	for _, pod := range pods.Items {
		w := toWorkload(&pod, p.name)
		workloads = append(workloads, w)
	}

	return workloads, nil
}

// GetAvailableResources returns available resources in the cluster
func (p *Provider) GetAvailableResources(ctx context.Context) (*provider.ResourceAvailability, error) {
	nodes, err := p.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var totalCPU, totalMemory resource.Quantity
	var usedCPU, usedMemory resource.Quantity
	gpuTypes := make(map[string]provider.GPUTypeInfo)

	for _, node := range nodes.Items {
		// Add node capacity
		if cpu, ok := node.Status.Capacity[corev1.ResourceCPU]; ok {
			totalCPU.Add(cpu)
		}
		if memory, ok := node.Status.Capacity[corev1.ResourceMemory]; ok {
			totalMemory.Add(memory)
		}

		// Check for GPUs
		for resourceName, quantity := range node.Status.Capacity {
			if strings.Contains(string(resourceName), "gpu") {
				gpuType := string(resourceName)
				if _, exists := gpuTypes[gpuType]; !exists {
					gpuTypes[gpuType] = provider.GPUTypeInfo{
						Name:         gpuType,
						Memory:       "16Gi",
						Total:        int(quantity.Value()),
						Available:    int(quantity.Value()),
						PricePerHour: 0.0,
					}
				} else {
					info := gpuTypes[gpuType]
					info.Total += int(quantity.Value())
					info.Available += int(quantity.Value())
					gpuTypes[gpuType] = info
				}
			}
		}

		// Add node allocatable (used resources)
		if cpu, ok := node.Status.Allocatable[corev1.ResourceCPU]; ok {
			usedCPU.Add(cpu)
		}
		if memory, ok := node.Status.Allocatable[corev1.ResourceMemory]; ok {
			usedMemory.Add(memory)
		}
	}

	// Calculate available resources
	availableCPU := totalCPU.DeepCopy()
	availableCPU.Sub(usedCPU)

	availableMemory := totalMemory.DeepCopy()
	availableMemory.Sub(usedMemory)

	return &provider.ResourceAvailability{
		CPU: provider.ResourcePool{
			Total:     totalCPU.String(),
			Available: availableCPU.String(),
			Used:      usedCPU.String(),
		},
		Memory: provider.ResourcePool{
			Total:     totalMemory.String(),
			Available: availableMemory.String(),
			Used:      usedMemory.String(),
		},
		GPU: provider.GPUPool{
			Types: gpuTypes,
		},
		Regions: []provider.RegionInfo{
			{
				Name:        "default",
				DisplayName: "Default Cluster",
				Available:   true,
				GPUTypes:    []string{},
			},
		},
	}, nil
}

// GetPricing returns pricing information for the provider
func (p *Provider) GetPricing(ctx context.Context) (*provider.PricingInfo, error) {
	return &provider.PricingInfo{
		Currency: "USD",
		CPU: provider.PricePerUnit{
			Amount: 0.0,
			Unit:   "hour",
		},
		Memory: provider.PricePerUnit{
			Amount: 0.0,
			Unit:   "hour",
		},
		GPU: map[string]provider.PricePerUnit{
			"nvidia.com/gpu": {
				Amount: 0.0,
				Unit:   "hour",
			},
		},
		Storage: provider.PricePerUnit{
			Amount: 0.0,
			Unit:   "month",
		},
		Network: provider.NetworkPricing{
			Ingress: provider.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
			Egress: provider.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
			Internal: provider.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
		},
	}, nil
}

// HealthCheck checks if the provider is healthy
func (p *Provider) HealthCheck(ctx context.Context) error {
	_, err := p.client.CoreV1().Namespaces().Get(ctx, p.namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("kubernetes health check failed: %w", err)
	}
	return nil
}

// GetStatus returns the current status of the provider
func (p *Provider) GetStatus(ctx context.Context) (*provider.ProviderStatus, error) {
	available := true
	if err := p.HealthCheck(ctx); err != nil {
		available = false
	}

	var load float64
	var activeWorkloads, totalWorkloads int

	if available {
		// Get cluster resources to calculate load
		resources, resourceErr := p.GetAvailableResources(ctx)
		if resourceErr == nil {
			if resources.CPU.Total != "" && resources.CPU.Used != "" {
				totalCPU := resource.MustParse(resources.CPU.Total)
				usedCPU := resource.MustParse(resources.CPU.Used)
				if !totalCPU.IsZero() {
					cpuLoad := float64(usedCPU.MilliValue()) / float64(totalCPU.MilliValue())
					load = cpuLoad
				}
			}
		}

		// Count actual workloads
		pods, podsErr := p.client.CoreV1().Pods(p.namespace).List(ctx, metav1.ListOptions{
			LabelSelector: "fabric.workload.id",
		})
		if podsErr == nil {
			totalWorkloads = len(pods.Items)
			for _, pod := range pods.Items {
				if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
					activeWorkloads++
				}
			}
		}
	}

	// Measure latency
	latency := 10
	if available {
		start := time.Now()
		_, latencyErr := p.client.CoreV1().Namespaces().Get(ctx, p.namespace, metav1.GetOptions{})
		if latencyErr == nil {
			latency = int(time.Since(start).Milliseconds())
		}
	}

	return &provider.ProviderStatus{
		Available: available,
		Regions: []provider.RegionStatus{
			{
				Name:      "default",
				Available: available,
				Load:      load,
				Latency:   latency,
			},
		},
		Metrics: provider.ProviderMetrics{
			ActiveWorkloads:  activeWorkloads,
			TotalWorkloads:   totalWorkloads,
			SuccessRate:      1.0,
			AverageStartTime: 30,
			AverageLatency:   latency,
		},
	}, nil
}
