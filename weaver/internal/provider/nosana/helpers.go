package nosana

import (
	"strconv"
	"strings"

	"github.com/codecflow/fabric/weaver/internal/workload"
)

// parseResources converts Fabric resource specifications to Nosana format
func parseResources(w *workload.Workload) Resources {
	return Resources{
		CPU:    parseCPU(w.Spec.Resources.CPU),
		Memory: parseMemory(w.Spec.Resources.Memory),
		GPU:    parseGPU(w.Spec.Resources.GPU),
		Disk:   "20Gi", // Default disk size since storage not in ResourceRequests
	}
}

// parseCPU converts CPU specification to Nosana format
func parseCPU(cpuSpec string) string {
	if cpuSpec == "" {
		return "2" // Default 2 cores
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
			return strconv.Itoa(cores)
		}
	}

	// Try to parse as direct number
	if cores, err := strconv.Atoi(cpuSpec); err == nil {
		return strconv.Itoa(cores)
	}

	return "2" // Default
}

// parseMemory converts memory specification to Nosana format
func parseMemory(memorySpec string) string {
	if memorySpec == "" {
		return "4Gi" // Default 4GB
	}

	// Normalize to Gi format for Nosana
	spec := strings.ToLower(memorySpec)

	if strings.HasSuffix(spec, "gi") {
		return memorySpec // Already in correct format
	}

	if strings.HasSuffix(spec, "gb") {
		// Convert GB to Gi (approximately)
		gbStr := strings.TrimSuffix(spec, "gb")
		if gb, err := strconv.Atoi(gbStr); err == nil {
			return strconv.Itoa(gb) + "Gi"
		}
	}

	if strings.HasSuffix(spec, "mi") {
		// Convert Mi to Gi
		miStr := strings.TrimSuffix(spec, "mi")
		if mi, err := strconv.Atoi(miStr); err == nil {
			gi := mi / 1024
			if gi < 1 {
				gi = 1
			}
			return strconv.Itoa(gi) + "Gi"
		}
	}

	// Try to parse as direct number (assume GB)
	if gb, err := strconv.Atoi(spec); err == nil {
		return strconv.Itoa(gb) + "Gi"
	}

	return "4Gi" // Default
}

// parseGPU converts GPU specification to Nosana format
func parseGPU(gpuSpec string) string {
	if gpuSpec == "" {
		return "" // No GPU required
	}

	// Extract GPU count and type
	if strings.Contains(gpuSpec, "=") {
		parts := strings.Split(gpuSpec, "=")
		if len(parts) > 1 {
			return parts[1] // Return the count/type part
		}
	}

	// If it's just a number, assume that many GPUs
	if count, err := strconv.Atoi(gpuSpec); err == nil {
		return strconv.Itoa(count)
	}

	return gpuSpec // Return as-is
}

// calculatePrice calculates job price based on resources and market rates
func (p *Provider) calculatePrice(resources Resources, market *Market) Price {
	// Base pricing (mock values - would be dynamic in real implementation)
	cpuPrice := 0.02    // per core hour
	memoryPrice := 0.01 // per GB hour
	gpuPrice := 0.50    // per GPU hour
	diskPrice := 0.001  // per GB hour

	// Parse resource values
	cpuCores, _ := strconv.Atoi(resources.CPU)
	memoryGB := p.parseMemoryToGB(resources.Memory)
	gpuCount := p.parseGPUCount(resources.GPU)
	diskGB := p.parseMemoryToGB(resources.Disk) // Reuse memory parser for disk

	// Calculate total hourly cost
	totalCost := float64(cpuCores)*cpuPrice +
		float64(memoryGB)*memoryPrice +
		float64(gpuCount)*gpuPrice +
		float64(diskGB)*diskPrice

	// Apply market multiplier if available
	if market != nil && market.MinPrice > 0 {
		if totalCost < market.MinPrice {
			totalCost = market.MinPrice
		}
	}

	return Price{
		Amount:   totalCost,
		Currency: "USD",
		Unit:     "hour",
	}
}

// parseMemoryToGB converts memory specification to GB
func (p *Provider) parseMemoryToGB(memorySpec string) int {
	if memorySpec == "" {
		return 4
	}

	spec := strings.ToLower(memorySpec)

	if strings.HasSuffix(spec, "gi") {
		giStr := strings.TrimSuffix(spec, "gi")
		if gi, err := strconv.Atoi(giStr); err == nil {
			return gi // Close enough for pricing
		}
	}

	if strings.HasSuffix(spec, "gb") {
		gbStr := strings.TrimSuffix(spec, "gb")
		if gb, err := strconv.Atoi(gbStr); err == nil {
			return gb
		}
	}

	return 4 // Default
}

// parseGPUCount extracts GPU count from specification
func (p *Provider) parseGPUCount(gpuSpec string) int {
	if gpuSpec == "" {
		return 0
	}

	if count, err := strconv.Atoi(gpuSpec); err == nil {
		return count
	}

	return 1 // Default if GPU specified but count unclear
}

// nosanaJobToWorkload converts a Nosana job to a Fabric workload
func (p *Provider) nosanaJobToWorkload(job *Job) *workload.Workload {
	w := &workload.Workload{
		ID:        job.ID,
		Name:      job.Name,
		Namespace: "default",
		Spec: workload.Spec{
			Image:   job.Image,
			Command: job.Command,
			Args:    job.Args,
			Env:     job.Env,
		},
		Status: workload.Status{
			Phase:    p.nosanaStatusToPhase(job.Status),
			NodeID:   job.NodeID,
			Provider: p.name,
		},
	}

	// Set resources from job
	w.Spec.Resources = workload.ResourceRequests{
		CPU:    job.Resources.CPU,
		Memory: job.Resources.Memory,
		GPU:    job.Resources.GPU,
	}

	return w
}

// nosanaStatusToPhase converts Nosana job status to Fabric workload phase
func (p *Provider) nosanaStatusToPhase(status string) workload.Phase {
	switch status {
	case JobStatusRunning:
		return workload.PhaseRunning
	case JobStatusPending:
		return workload.PhasePending
	case JobStatusCompleted:
		return workload.PhaseSucceeded
	case JobStatusFailed:
		return workload.PhaseFailed
	case JobStatusCanceled:
		return workload.PhaseFailed
	default:
		return workload.PhaseUnknown
	}
}

// selectMarket selects the best market for a workload
func (p *Provider) selectMarket(markets []*Market, _ Resources) *Market {
	if len(markets) == 0 {
		return nil
	}

	// For now, select the first active market
	// In a real implementation, this would consider pricing, availability, etc.
	for _, market := range markets {
		if market.Active {
			return market
		}
	}

	return markets[0] // Fallback to first market
}
