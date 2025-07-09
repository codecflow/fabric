package runpod

import (
	"strconv"
	"strings"

	"github.com/codecflow/fabric/weaver/internal/workload"
)

// selectGPUType maps Fabric GPU requirements to RunPod GPU types
func (p *Provider) selectGPUType(gpuSpec string) string {
	if gpuSpec == "" {
		return "NVIDIA RTX A6000" // Default GPU type
	}

	// Parse GPU specification and map to RunPod types
	switch {
	case strings.Contains(strings.ToLower(gpuSpec), "a100"):
		return "NVIDIA A100"
	case strings.Contains(strings.ToLower(gpuSpec), "4090"):
		return "NVIDIA RTX 4090"
	case strings.Contains(strings.ToLower(gpuSpec), "a6000"):
		return "NVIDIA RTX A6000"
	default:
		return "NVIDIA RTX A6000"
	}
}

// parseGPUCount extracts GPU count from resource specification
func (p *Provider) parseGPUCount(gpuSpec string) int {
	if gpuSpec == "" {
		return 1
	}

	// Extract number from specifications like "2", "nvidia.com/gpu=2", etc.
	parts := strings.Split(gpuSpec, "=")
	if len(parts) > 1 {
		if count, err := strconv.Atoi(parts[1]); err == nil {
			return count
		}
	}

	// Try to parse the spec directly as a number
	if count, err := strconv.Atoi(gpuSpec); err == nil {
		return count
	}

	return 1 // Default to 1 GPU
}

// parseCPUCount extracts CPU count from resource specification
func (p *Provider) parseCPUCount(cpuSpec string) int {
	if cpuSpec == "" {
		return 4 // Default CPU count
	}

	// Handle Kubernetes-style CPU specifications
	if strings.HasSuffix(cpuSpec, "m") {
		// Millicores (e.g., "2000m" = 2 cores)
		milliStr := strings.TrimSuffix(cpuSpec, "m")
		if milli, err := strconv.Atoi(milliStr); err == nil {
			return milli / 1000
		}
	}

	// Try to parse as direct number
	if count, err := strconv.Atoi(cpuSpec); err == nil {
		return count
	}

	return 4 // Default
}

// parseMemoryGB extracts memory in GB from resource specification
func (p *Provider) parseMemoryGB(memorySpec string) int {
	if memorySpec == "" {
		return 16 // Default 16GB
	}

	// Remove units and convert to GB
	spec := strings.ToLower(memorySpec)

	if strings.HasSuffix(spec, "gi") {
		// Gibibytes
		giStr := strings.TrimSuffix(spec, "gi")
		if gi, err := strconv.Atoi(giStr); err == nil {
			return gi // Close enough for RunPod
		}
	}

	if strings.HasSuffix(spec, "gb") {
		// Gigabytes
		gbStr := strings.TrimSuffix(spec, "gb")
		if gb, err := strconv.Atoi(gbStr); err == nil {
			return gb
		}
	}

	if strings.HasSuffix(spec, "mi") {
		// Mebibytes
		miStr := strings.TrimSuffix(spec, "mi")
		if mi, err := strconv.Atoi(miStr); err == nil {
			return mi / 1024 // Convert to GB
		}
	}

	// Try to parse as direct number (assume GB)
	if gb, err := strconv.Atoi(spec); err == nil {
		return gb
	}

	return 16 // Default
}

// formatPorts converts Fabric port specifications to RunPod format
func (p *Provider) formatPorts(ports []workload.Port) string {
	if len(ports) == 0 {
		return ""
	}

	var portStrs []string
	for _, port := range ports {
		portStrs = append(portStrs, strconv.Itoa(int(port.ContainerPort)))
	}

	return strings.Join(portStrs, ",")
}

// runPodToWorkload converts a RunPod pod to a Fabric workload
func (p *Provider) toWorkload(pod *Pod) *workload.Workload {
	w := &workload.Workload{
		ID:        pod.ID,
		Name:      pod.Name,
		Namespace: "default",
		Spec: workload.Spec{
			Image: pod.ImageName,
			Env:   pod.Env,
		},
		Status: workload.Status{
			Phase:    p.runPodStatusToPhase(pod.Status),
			NodeID:   pod.MachineID,
			Provider: p.name,
		},
	}

	// Convert ports if available
	if pod.Runtime.Ports != nil {
		var ports []workload.Port
		for _, port := range pod.Runtime.Ports {
			ports = append(ports, workload.Port{
				ContainerPort: int32(port.PrivatePort), // nolint:gosec
				Protocol:      "TCP",
			})
		}
		w.Spec.Ports = ports
	}

	return w
}

// runPodStatusToPhase converts RunPod status to Fabric workload phase
func (p *Provider) runPodStatusToPhase(status string) workload.Phase {
	switch strings.ToLower(status) {
	case "running":
		return workload.PhaseRunning
	case "pending", "starting":
		return workload.PhasePending
	case "stopped", "terminated":
		return workload.PhaseSucceeded
	case "failed", "error":
		return workload.PhaseFailed
	default:
		return workload.PhaseUnknown
	}
}
