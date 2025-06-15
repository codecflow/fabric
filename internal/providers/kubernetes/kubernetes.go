package kubernetes

import (
	"context"
	"fabric/internal/providers"
	"fabric/internal/workload"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesProvider implements the Provider interface for Kubernetes
type KubernetesProvider struct {
	client    kubernetes.Interface
	config    *rest.Config
	namespace string
	name      string
}

// NewKubernetesProvider creates a new Kubernetes provider
func NewKubernetesProvider(name, kubeconfig, namespace string) (*KubernetesProvider, error) {
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		// Use in-cluster config
		config, err = rest.InClusterConfig()
	} else {
		// Use kubeconfig file
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	if namespace == "" {
		namespace = "default"
	}

	return &KubernetesProvider{
		client:    client,
		config:    config,
		namespace: namespace,
		name:      name,
	}, nil
}

// Name returns the provider name
func (k *KubernetesProvider) Name() string {
	return k.name
}

// Type returns the provider type
func (k *KubernetesProvider) Type() providers.ProviderType {
	return providers.ProviderTypeKubernetes
}

// CreateWorkload creates a new workload in Kubernetes
func (k *KubernetesProvider) CreateWorkload(ctx context.Context, w *workload.Workload) error {
	// Convert Fabric workload to Kubernetes Pod
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      w.Name,
			Namespace: k.namespace,
			Labels: map[string]string{
				"fabric.workload.id":   w.ID,
				"fabric.workload.name": w.Name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  w.Name,
					Image: w.Spec.Image,
				},
			},
		},
	}

	// Set environment variables
	if len(w.Spec.Env) > 0 {
		var envVars []corev1.EnvVar
		for key, value := range w.Spec.Env {
			envVars = append(envVars, corev1.EnvVar{
				Name:  key,
				Value: value,
			})
		}
		pod.Spec.Containers[0].Env = envVars
	}

	// Set resource requirements
	if w.Spec.Resources.CPU != "" || w.Spec.Resources.Memory != "" {
		resources := corev1.ResourceRequirements{
			Requests: corev1.ResourceList{},
			Limits:   corev1.ResourceList{},
		}

		if w.Spec.Resources.CPU != "" {
			cpuQuantity := resource.MustParse(w.Spec.Resources.CPU)
			resources.Requests[corev1.ResourceCPU] = cpuQuantity
			resources.Limits[corev1.ResourceCPU] = cpuQuantity
		}

		if w.Spec.Resources.Memory != "" {
			memQuantity := resource.MustParse(w.Spec.Resources.Memory)
			resources.Requests[corev1.ResourceMemory] = memQuantity
			resources.Limits[corev1.ResourceMemory] = memQuantity
		}

		if w.Spec.Resources.GPU != "" {
			gpuQuantity := resource.MustParse(w.Spec.Resources.GPU)
			resources.Requests[corev1.ResourceName("nvidia.com/gpu")] = gpuQuantity
			resources.Limits[corev1.ResourceName("nvidia.com/gpu")] = gpuQuantity
		}

		pod.Spec.Containers[0].Resources = resources
	}

	// Set ports
	if len(w.Spec.Ports) > 0 {
		var ports []corev1.ContainerPort
		for _, port := range w.Spec.Ports {
			protocol := corev1.ProtocolTCP
			if strings.ToUpper(port.Protocol) == "UDP" {
				protocol = corev1.ProtocolUDP
			}

			ports = append(ports, corev1.ContainerPort{
				ContainerPort: port.ContainerPort,
				Protocol:      protocol,
			})
		}
		pod.Spec.Containers[0].Ports = ports
	}

	// Create the pod
	_, err := k.client.CoreV1().Pods(k.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	return nil
}

// GetWorkload retrieves a workload from Kubernetes
func (k *KubernetesProvider) GetWorkload(ctx context.Context, id string) (*workload.Workload, error) {
	pods, err := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("fabric.workload.id=%s", id),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("workload not found")
	}

	pod := pods.Items[0]
	return k.podToWorkload(&pod), nil
}

// UpdateWorkload updates a workload in Kubernetes
func (k *KubernetesProvider) UpdateWorkload(ctx context.Context, w *workload.Workload) error {
	// For now, we'll just update the labels/annotations
	// In a full implementation, you might need to recreate the pod
	pods, err := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
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

	_, err = k.client.CoreV1().Pods(k.namespace).Update(ctx, pod, metav1.UpdateOptions{})
	return err
}

// DeleteWorkload deletes a workload from Kubernetes
func (k *KubernetesProvider) DeleteWorkload(ctx context.Context, id string) error {
	pods, err := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("fabric.workload.id=%s", id),
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	for _, pod := range pods.Items {
		err := k.client.CoreV1().Pods(k.namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete pod %s: %w", pod.Name, err)
		}
	}

	return nil
}

// ListWorkloads lists workloads in a namespace
func (k *KubernetesProvider) ListWorkloads(ctx context.Context, namespace string) ([]*workload.Workload, error) {
	if namespace == "" {
		namespace = k.namespace
	}

	pods, err := k.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "fabric.workload.id",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var workloads []*workload.Workload
	for _, pod := range pods.Items {
		w := k.podToWorkload(&pod)
		workloads = append(workloads, w)
	}

	return workloads, nil
}

// GetAvailableResources returns available resources in the cluster
func (k *KubernetesProvider) GetAvailableResources(ctx context.Context) (*providers.ResourceAvailability, error) {
	nodes, err := k.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list nodes: %w", err)
	}

	var totalCPU, totalMemory resource.Quantity
	var usedCPU, usedMemory resource.Quantity
	gpuTypes := make(map[string]providers.GPUTypeInfo)

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
					gpuTypes[gpuType] = providers.GPUTypeInfo{
						Name:         gpuType,
						Memory:       "16Gi", // Default GPU memory
						Total:        int(quantity.Value()),
						Available:    int(quantity.Value()),
						PricePerHour: 0.0, // Would be set based on provider pricing
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

	return &providers.ResourceAvailability{
		CPU: providers.ResourcePool{
			Total:     totalCPU.String(),
			Available: availableCPU.String(),
			Used:      usedCPU.String(),
		},
		Memory: providers.ResourcePool{
			Total:     totalMemory.String(),
			Available: availableMemory.String(),
			Used:      usedMemory.String(),
		},
		GPU: providers.GPUPool{
			Types: gpuTypes,
		},
		Regions: []providers.RegionInfo{
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
func (k *KubernetesProvider) GetPricing(ctx context.Context) (*providers.PricingInfo, error) {
	// For Kubernetes, pricing is typically $0 as it's self-hosted
	// In a real implementation, you might calculate costs based on node pricing
	return &providers.PricingInfo{
		Currency: "USD",
		CPU: providers.PricePerUnit{
			Amount: 0.0,
			Unit:   "hour",
		},
		Memory: providers.PricePerUnit{
			Amount: 0.0,
			Unit:   "hour",
		},
		GPU: map[string]providers.PricePerUnit{
			"nvidia.com/gpu": {
				Amount: 0.0,
				Unit:   "hour",
			},
		},
		Storage: providers.PricePerUnit{
			Amount: 0.0,
			Unit:   "month",
		},
		Network: providers.NetworkPricing{
			Ingress: providers.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
			Egress: providers.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
			Internal: providers.PricePerUnit{
				Amount: 0.0,
				Unit:   "gb",
			},
		},
	}, nil
}

// HealthCheck checks if the provider is healthy
func (k *KubernetesProvider) HealthCheck(ctx context.Context) error {
	_, err := k.client.CoreV1().Namespaces().Get(ctx, k.namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("kubernetes health check failed: %w", err)
	}
	return nil
}

// GetStatus returns the current status of the provider
func (k *KubernetesProvider) GetStatus(ctx context.Context) (*providers.ProviderStatus, error) {
	// Check if cluster is available
	available := true
	if err := k.HealthCheck(ctx); err != nil {
		available = false
	}

	// Calculate actual metrics
	var load float64
	var activeWorkloads, totalWorkloads int

	if available {
		// Get cluster resources to calculate load
		resources, resourceErr := k.GetAvailableResources(ctx)
		if resourceErr == nil {
			// Calculate load based on resource utilization
			// Parse CPU usage percentage
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
		pods, podsErr := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
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

	// Measure actual latency by timing a simple API call
	var latency int = 10 // default fallback
	if available {
		start := time.Now()
		_, latencyErr := k.client.CoreV1().Namespaces().Get(ctx, k.namespace, metav1.GetOptions{})
		if latencyErr == nil {
			latency = int(time.Since(start).Milliseconds())
		}
	}

	status := &providers.ProviderStatus{
		Available: available,
		Regions: []providers.RegionStatus{
			{
				Name:      "default",
				Available: available,
				Load:      load,
				Latency:   latency,
			},
		},
		Metrics: providers.ProviderMetrics{
			ActiveWorkloads:  activeWorkloads,
			TotalWorkloads:   totalWorkloads,
			SuccessRate:      1.0,
			AverageStartTime: 30,
			AverageLatency:   latency,
		},
	}

	return status, nil
}

// Helper function to convert Kubernetes Pod to Fabric Workload
func (k *KubernetesProvider) podToWorkload(pod *corev1.Pod) *workload.Workload {
	w := &workload.Workload{
		ID:        pod.Labels["fabric.workload.id"],
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Spec: workload.Spec{
			Image: pod.Spec.Containers[0].Image,
		},
		Status: workload.Status{
			Phase:    k.podPhaseToWorkloadPhase(pod.Status.Phase),
			NodeID:   pod.Spec.NodeName,
			Provider: k.name,
		},
		CreatedAt: pod.CreationTimestamp.Time,
		UpdatedAt: pod.CreationTimestamp.Time,
	}

	// Convert environment variables
	if len(pod.Spec.Containers[0].Env) > 0 {
		env := make(map[string]string)
		for _, envVar := range pod.Spec.Containers[0].Env {
			env[envVar.Name] = envVar.Value
		}
		w.Spec.Env = env
	}

	// Convert ports
	if len(pod.Spec.Containers[0].Ports) > 0 {
		var ports []workload.Port
		for _, port := range pod.Spec.Containers[0].Ports {
			ports = append(ports, workload.Port{
				ContainerPort: port.ContainerPort,
				Protocol:      string(port.Protocol),
			})
		}
		w.Spec.Ports = ports
	}

	return w
}

// Helper function to convert Pod phase to Workload phase
func (k *KubernetesProvider) podPhaseToWorkloadPhase(phase corev1.PodPhase) workload.Phase {
	switch phase {
	case corev1.PodPending:
		return workload.PhasePending
	case corev1.PodRunning:
		return workload.PhaseRunning
	case corev1.PodSucceeded:
		return workload.PhaseSucceeded
	case corev1.PodFailed:
		return workload.PhaseFailed
	default:
		return workload.PhaseUnknown
	}
}
