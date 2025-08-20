package placement

import (
	"context"
	"fmt"
	"sync"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesStrategy implements Strategy using Kubernetes as the backend
type KubernetesStrategy struct {
	client     *kubernetes.Clientset
	namespace  string
	placements map[string]*Placement
	mu         sync.RWMutex
}

// NewKubernetesStrategy creates a new Kubernetes placement strategy
func NewKubernetesStrategy(namespace string) (*KubernetesStrategy, error) {
	if namespace == "" {
		namespace = "default"
	}

	// Try in-cluster config first, then fall back to kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &KubernetesStrategy{
		client:     clientset,
		namespace:  namespace,
		placements: make(map[string]*Placement),
	}, nil
}

// Name returns the name of this strategy
func (k *KubernetesStrategy) Name() string {
	return "kubernetes"
}

// Create creates a Pod in Kubernetes
func (k *KubernetesStrategy) Create(ctx context.Context, req *Request) (*Placement, error) {
	// Generate unique placement ID
	id := uuid.New().String()
	podName := fmt.Sprintf("fabric-%s", id[:8])

	// Create placement record
	placement := &Placement{
		ID:       id,
		Provider: "kubernetes",
		Spec:     req.Spec,
		Status:   "created",
		Metadata: make(map[string]interface{}),
	}

	// Convert container spec to Kubernetes Pod
	pod := k.specToPod(podName, req.Spec)

	// Create the Pod
	createdPod, err := k.client.CoreV1().Pods(k.namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		placement.Status = "failed"
		placement.Metadata["error"] = err.Error()
		return placement, fmt.Errorf("failed to create pod: %w", err)
	}

	placement.ContainerID = createdPod.Name
	placement.Status = "running"
	placement.Metadata["pod_name"] = createdPod.Name
	placement.Metadata["namespace"] = k.namespace

	// Create Service for port exposure if persistent
	if req.Spec.Persistent && len(req.Spec.Ports) > 0 {
		service, err := k.createService(ctx, podName, req.Spec)
		if err != nil {
			// Don't fail - pod is already created
			placement.Metadata["service_error"] = err.Error()
		} else {
			placement.Metadata["service_name"] = service.Name
			// Get NodePort mappings
			ports := make(map[int]int)
			for _, port := range service.Spec.Ports {
				ports[int(port.TargetPort.IntVal)] = int(port.NodePort)
			}
			placement.Ports = ports
			placement.Address = fmt.Sprintf("localhost:%d", ports[req.Spec.Ports[0].Container])
		}
	}

	// Store placement
	k.mu.Lock()
	k.placements[id] = placement
	k.mu.Unlock()

	return placement, nil
}

// specToPod converts a container spec to a Kubernetes Pod
func (k *KubernetesStrategy) specToPod(name string, spec *container.Spec) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.namespace,
			Labels: map[string]string{
				"app":     "fabric",
				"managed": "fabric",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "main",
					Image:   spec.Image,
					Command: spec.Command,
				},
			},
		},
	}

	// Add environment variables
	if len(spec.Env) > 0 {
		envVars := make([]corev1.EnvVar, 0, len(spec.Env))
		for k, v := range spec.Env {
			envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
		}
		pod.Spec.Containers[0].Env = envVars
	}

	// Add working directory
	if spec.WorkDir != "" {
		pod.Spec.Containers[0].WorkingDir = spec.WorkDir
	}

	// Add GPU resources
	if spec.GPU > 0 {
		pod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				"nvidia.com/gpu": *resource.NewQuantity(int64(spec.GPU), resource.DecimalSI),
			},
		}
	}

	// Add ports
	if len(spec.Ports) > 0 {
		containerPorts := make([]corev1.ContainerPort, 0, len(spec.Ports))
		for _, p := range spec.Ports {
			protocol := corev1.ProtocolTCP
			if p.Protocol == "udp" {
				protocol = corev1.ProtocolUDP
			}
			containerPorts = append(containerPorts, corev1.ContainerPort{
				ContainerPort: int32(p.Container),
				Protocol:      protocol,
			})
		}
		pod.Spec.Containers[0].Ports = containerPorts
	}

	// For persistent containers, set restart policy to Always
	if spec.Persistent {
		pod.Spec.RestartPolicy = corev1.RestartPolicyAlways
	}

	return pod
}

// createService creates a NodePort Service for port exposure
func (k *KubernetesStrategy) createService(ctx context.Context, podName string, spec *container.Spec) (*corev1.Service, error) {
	serviceName := podName + "-svc"

	servicePorts := make([]corev1.ServicePort, 0, len(spec.Ports))
	for i, p := range spec.Ports {
		protocol := corev1.ProtocolTCP
		if p.Protocol == "udp" {
			protocol = corev1.ProtocolUDP
		}
		servicePorts = append(servicePorts, corev1.ServicePort{
			Name:       fmt.Sprintf("port-%d", i),
			Port:       int32(p.Container),
			TargetPort: intstr.FromInt(p.Container),
			Protocol:   protocol,
		})
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: k.namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "fabric",
			},
			Ports: servicePorts,
		},
	}

	return k.client.CoreV1().Services(k.namespace).Create(ctx, service, metav1.CreateOptions{})
}

// Get retrieves a placement by ID
func (k *KubernetesStrategy) Get(ctx context.Context, id string) (*Placement, error) {
	k.mu.RLock()
	placement, ok := k.placements[id]
	k.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("placement not found: %s", id)
	}

	// Update status from Kubernetes
	if podName, ok := placement.Metadata["pod_name"].(string); ok {
		pod, err := k.client.CoreV1().Pods(k.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err == nil {
			placement.Status = string(pod.Status.Phase)
		}
	}

	return placement, nil
}

// Stop stops a running placement
func (k *KubernetesStrategy) Stop(ctx context.Context, id string) error {
	k.mu.RLock()
	placement, ok := k.placements[id]
	k.mu.RUnlock()

	if !ok {
		return fmt.Errorf("placement not found: %s", id)
	}

	podName, ok := placement.Metadata["pod_name"].(string)
	if !ok {
		return fmt.Errorf("pod name not found in placement metadata")
	}

	// Delete the Pod
	err := k.client.CoreV1().Pods(k.namespace).Delete(ctx, podName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	// Delete Service if exists
	if serviceName, ok := placement.Metadata["service_name"].(string); ok {
		k.client.CoreV1().Services(k.namespace).Delete(ctx, serviceName, metav1.DeleteOptions{})
	}

	k.mu.Lock()
	placement.Status = "stopped"
	k.mu.Unlock()

	return nil
}

// Delete removes a placement
func (k *KubernetesStrategy) Delete(ctx context.Context, id string) error {
	// Stop first
	if err := k.Stop(ctx, id); err != nil {
		return err
	}

	k.mu.Lock()
	delete(k.placements, id)
	k.mu.Unlock()

	return nil
}

// List returns all placements
func (k *KubernetesStrategy) List(ctx context.Context) ([]*Placement, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	placements := make([]*Placement, 0, len(k.placements))
	for _, p := range k.placements {
		placements = append(placements, p)
	}

	return placements, nil
}
