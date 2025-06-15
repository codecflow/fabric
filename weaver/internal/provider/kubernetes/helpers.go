package kubernetes

import (
	"strings"
	"weaver/internal/workload"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// workloadToPod converts a Fabric workload to a Kubernetes Pod
func toPod(w *workload.Workload, namespace string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      w.Name,
			Namespace: namespace,
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

	// Set command and args
	if len(w.Spec.Command) > 0 {
		pod.Spec.Containers[0].Command = w.Spec.Command
	}
	if len(w.Spec.Args) > 0 {
		pod.Spec.Containers[0].Args = w.Spec.Args
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
	if w.Spec.Resources.CPU != "" || w.Spec.Resources.Memory != "" || w.Spec.Resources.GPU != "" {
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

	return pod
}

// podToWorkload converts a Kubernetes Pod to a Fabric Workload
func toWorkload(pod *corev1.Pod, providerName string) *workload.Workload {
	w := &workload.Workload{
		ID:        pod.Labels["fabric.workload.id"],
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Spec: workload.Spec{
			Image: pod.Spec.Containers[0].Image,
		},
		Status: workload.Status{
			Phase:    toWorkloadPhase(pod.Status.Phase),
			NodeID:   pod.Spec.NodeName,
			Provider: providerName,
		},
		CreatedAt: pod.CreationTimestamp.Time,
		UpdatedAt: pod.CreationTimestamp.Time,
	}

	// Convert command and args
	if len(pod.Spec.Containers[0].Command) > 0 {
		w.Spec.Command = pod.Spec.Containers[0].Command
	}
	if len(pod.Spec.Containers[0].Args) > 0 {
		w.Spec.Args = pod.Spec.Containers[0].Args
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

	// Convert resources
	if resources := pod.Spec.Containers[0].Resources; len(resources.Requests) > 0 || len(resources.Limits) > 0 {
		if cpu, ok := resources.Requests[corev1.ResourceCPU]; ok {
			w.Spec.Resources.CPU = cpu.String()
		}
		if memory, ok := resources.Requests[corev1.ResourceMemory]; ok {
			w.Spec.Resources.Memory = memory.String()
		}
		if gpu, ok := resources.Requests[corev1.ResourceName("nvidia.com/gpu")]; ok {
			w.Spec.Resources.GPU = gpu.String()
		}
	}

	return w
}

// podPhaseToWorkloadPhase converts Pod phase to Workload phase
func toWorkloadPhase(phase corev1.PodPhase) workload.Phase {
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
