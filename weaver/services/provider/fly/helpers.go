package fly

import (
	"strconv"
	"strings"

	"github.com/codecflow/fabric/pkg/workload"
)

// parseGuest converts Fabric resource specifications to Fly.io Guest format
func parseGuest(w *workload.Workload) Guest {
	return Guest{
		CPUs:     parseCPUs(w.Spec.Resources.CPU),
		CPUKind:  parseCPUKind(w.Spec.Resources.CPU),
		MemoryMB: parseMemoryMB(w.Spec.Resources.Memory),
		GPUs:     parseGPUs(w.Spec.Resources.GPU),
		GPUKind:  parseGPUKind(w.Spec.Resources.GPU),
	}
}

// parseCPUs converts CPU specification to number of CPUs
func parseCPUs(cpuSpec string) int {
	if cpuSpec == "" {
		return 1 // Default 1 CPU
	}

	// Handle Kubernetes-style CPU specifications
	if strings.HasSuffix(cpuSpec, "m") {
		// Millicores (e.g., "2000m" = 2 cores)
		milliStr := strings.TrimSuffix(cpuSpec, "m")
		if milli, err := strconv.Atoi(milliStr); err == nil {
			cores := milli / 1000
			if cores < 1 {
				cores = 1
			}
			return cores
		}
	}

	// Try to parse as direct number
	if cores, err := strconv.Atoi(cpuSpec); err == nil {
		if cores < 1 {
			cores = 1
		}
		return cores
	}

	return 1 // Default
}

// parseCPUKind determines CPU kind from specification
func parseCPUKind(cpuSpec string) string {
	// Check for performance indicators in the spec
	spec := strings.ToLower(cpuSpec)
	if strings.Contains(spec, "performance") || strings.Contains(spec, "dedicated") {
		return CPUKindPerformance
	}
	return CPUKindShared // Default to shared
}

// parseMemoryMB converts memory specification to MB
func parseMemoryMB(memorySpec string) int {
	if memorySpec == "" {
		return 1024 // Default 1GB
	}

	spec := strings.ToLower(memorySpec)

	if strings.HasSuffix(spec, "gi") {
		// Gibibytes to MB
		giStr := strings.TrimSuffix(spec, "gi")
		if gi, err := strconv.Atoi(giStr); err == nil {
			return gi * 1024 // 1 GiB = 1024 MiB
		}
	}

	if strings.HasSuffix(spec, "gb") {
		// Gigabytes to MB
		gbStr := strings.TrimSuffix(spec, "gb")
		if gb, err := strconv.Atoi(gbStr); err == nil {
			return gb * 1000 // 1 GB = 1000 MB
		}
	}

	if strings.HasSuffix(spec, "mi") {
		// Mebibytes
		miStr := strings.TrimSuffix(spec, "mi")
		if mi, err := strconv.Atoi(miStr); err == nil {
			return mi
		}
	}

	if strings.HasSuffix(spec, "mb") {
		// Megabytes
		mbStr := strings.TrimSuffix(spec, "mb")
		if mb, err := strconv.Atoi(mbStr); err == nil {
			return mb
		}
	}

	// Try to parse as direct number (assume GB)
	if gb, err := strconv.Atoi(spec); err == nil {
		return gb * 1000
	}

	return 1024 // Default 1GB
}

// parseGPUs converts GPU specification to number of GPUs
func parseGPUs(gpuSpec string) int {
	if gpuSpec == "" {
		return 0 // No GPU required
	}

	// Extract GPU count
	if strings.Contains(gpuSpec, "=") {
		parts := strings.Split(gpuSpec, "=")
		if len(parts) > 1 {
			if count, err := strconv.Atoi(parts[1]); err == nil {
				return count
			}
		}
	}

	// If it's just a number, assume that many GPUs
	if count, err := strconv.Atoi(gpuSpec); err == nil {
		return count
	}

	return 1 // Default if GPU specified but count unclear
}

// parseGPUKind determines GPU kind from specification
func parseGPUKind(gpuSpec string) string {
	if gpuSpec == "" {
		return ""
	}

	spec := strings.ToLower(gpuSpec)

	// Check for specific GPU types
	if strings.Contains(spec, "a100") {
		if strings.Contains(spec, "80gb") || strings.Contains(spec, "sxm4") {
			return GPUKindA100SXM480GB
		}
		return GPUKindA100PCIe40GB
	}

	// Default to A100 PCIe if GPU requested but type unclear
	return GPUKindA100PCIe40GB
}

// parseServices converts workload ports to Fly.io services
func parseServices(w *workload.Workload) []Service {
	var services []Service

	for _, port := range w.Spec.Ports {
		service := Service{
			Protocol:     strings.ToLower(port.Protocol),
			InternalPort: int(port.ContainerPort),
		}

		// Add external port mapping
		if port.ContainerPort == 80 || port.ContainerPort == 8080 {
			service.Ports = []Port{
				{Port: 80, Handlers: []string{"http"}},
				{Port: 443, Handlers: []string{"http", "tls"}},
			}
		} else {
			service.Ports = []Port{
				{Port: int(port.ContainerPort)},
			}
		}

		services = append(services, service)
	}

	// Default HTTP service if no ports specified
	if len(services) == 0 {
		services = append(services, Service{
			Protocol:     "tcp",
			InternalPort: 8080,
			Ports: []Port{
				{Port: 80, Handlers: []string{"http"}},
				{Port: 443, Handlers: []string{"http", "tls"}},
			},
		})
	}

	return services
}

// parseMounts converts workload volumes to Fly.io mounts
func parseMounts(w *workload.Workload) []Mount {
	var mounts []Mount

	for _, volume := range w.Spec.Volumes {
		mount := Mount{
			Source:      volume.Name,
			Destination: volume.MountPath,
			Type:        "volume",
		}
		mounts = append(mounts, mount)
	}

	return mounts
}

// parseRestartPolicy converts workload restart policy to Fly.io format
func parseRestartPolicy(w *workload.Workload) RestartPolicy {
	switch w.Spec.Restart {
	case workload.RestartPolicyAlways:
		return RestartPolicy{Policy: "always"}
	case workload.RestartPolicyOnFailure:
		return RestartPolicy{Policy: "on-failure"}
	case workload.RestartPolicyNever:
		return RestartPolicy{Policy: "no"}
	default:
		return RestartPolicy{Policy: "always"}
	}
}

// selectRegion selects the best region for a workload
func selectRegion(regions []*Region, placement *workload.PlacementSpec) string {
	if placement != nil && placement.Region != "" {
		// Try to find exact match
		for _, region := range regions {
			if region.Code == placement.Region {
				return region.Code
			}
		}
	}

	// Default region selection logic
	preferredRegions := []string{"iad", "lax", "fra", "nrt", "syd"}
	for _, preferred := range preferredRegions {
		for _, region := range regions {
			if region.Code == preferred && region.GatewayAvailable {
				return region.Code
			}
		}
	}

	// Fallback to first available region
	for _, region := range regions {
		if region.GatewayAvailable {
			return region.Code
		}
	}

	return "iad" // Ultimate fallback
}

// machineToWorkload converts a Fly.io machine to a Fabric workload
func machineToWorkload(machine *Machine, appName string) *workload.Workload {
	w := &workload.Workload{
		ID:        machine.ID,
		Name:      machine.Name,
		Namespace: "default",
		Spec: workload.Spec{
			Image:   machine.Config.Image,
			Command: machine.Config.Cmd,
			Env:     machine.Config.Env,
		},
		Status: workload.Status{
			Phase:    machineStateToPhase(machine.State),
			NodeID:   machine.Region,
			Provider: "fly",
		},
	}

	// Convert guest config back to resource requests
	w.Spec.Resources = workload.ResourceRequests{
		CPU:    strconv.Itoa(machine.Config.Guest.CPUs),
		Memory: strconv.Itoa(machine.Config.Guest.MemoryMB/1024) + "Gi",
	}

	if machine.Config.Guest.GPUs > 0 {
		w.Spec.Resources.GPU = strconv.Itoa(machine.Config.Guest.GPUs)
	}

	// Convert services back to ports
	for _, service := range machine.Config.Services {
		port := workload.Port{
			ContainerPort: int32(service.InternalPort), // nolint:gosec
			Protocol:      strings.ToUpper(service.Protocol),
		}
		w.Spec.Ports = append(w.Spec.Ports, port)
	}

	// Convert mounts back to volumes
	for _, mount := range machine.Config.Mounts {
		volume := workload.VolumeMount{
			Name:      mount.Source,
			MountPath: mount.Destination,
		}
		w.Spec.Volumes = append(w.Spec.Volumes, volume)
	}

	return w
}

// machineStateToPhase converts Fly.io machine state to Fabric workload phase
func machineStateToPhase(state string) workload.Phase {
	switch state {
	case MachineStateStarted:
		return workload.PhaseRunning
	case MachineStateCreated, MachineStateStarting:
		return workload.PhasePending
	case MachineStateStopped:
		return workload.PhaseSucceeded
	case MachineStateDestroyed, MachineStateDestroying:
		return workload.PhaseFailed
	default:
		return workload.PhaseUnknown
	}
}

// generateAppName generates a unique app name for a workload
func generateAppName(workloadName, namespace string) string {
	// Fly.io app names must be lowercase and contain only letters, numbers, and hyphens
	name := strings.ToLower(workloadName)
	if namespace != "" && namespace != "default" {
		name = strings.ToLower(namespace) + "-" + name
	}

	// Replace invalid characters
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "-")

	// Ensure it starts with a letter
	if len(name) > 0 && (name[0] < 'a' || name[0] > 'z') {
		name = "app-" + name
	}

	// Truncate if too long (Fly.io has a limit)
	if len(name) > 30 {
		name = name[:30]
	}

	return name
}
