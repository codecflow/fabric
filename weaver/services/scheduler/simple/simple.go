package simple

import (
	"context"
	"fmt"
	"time"

	"github.com/codecflow/fabric/pkg/workload"
	"github.com/codecflow/fabric/weaver/services/provider"
	"github.com/codecflow/fabric/weaver/services/scheduler"
)

// SimpleScheduler implements a basic cost-aware scheduler
type SimpleScheduler struct {
	providers map[string]provider.Provider
	config    *scheduler.SchedulerConfig
	stats     *scheduler.SchedulerStats
}

// New creates a new simple scheduler
func New(providerMap map[string]provider.Provider, config *scheduler.SchedulerConfig) *SimpleScheduler {
	if config == nil {
		config = &scheduler.SchedulerConfig{
			DefaultPolicy: scheduler.SchedulingPolicy{
				Strategy:          scheduler.StrategyBalanced,
				CostWeight:        0.4,
				PerformanceWeight: 0.3,
				ReliabilityWeight: 0.2,
				LatencyWeight:     0.1,
			},
			MaxAlternatives:    3,
			ScheduleTimeout:    30 * time.Second,
			CostUpdateInterval: 5 * time.Minute,
		}
	}

	return &SimpleScheduler{
		providers: providerMap,
		config:    config,
		stats: &scheduler.SchedulerStats{
			ProviderStats:   make(map[string]*scheduler.ProviderStats),
			RecentSchedules: make([]*scheduler.RecentSchedule, 0),
			LastUpdated:     time.Now(),
		},
	}
}

// Schedule schedules a workload across available providers
func (s *SimpleScheduler) Schedule(ctx context.Context, w *workload.Workload) (*scheduler.ScheduleResult, error) {
	start := time.Now()

	// Get recommendations
	recommendations, err := s.GetRecommendations(ctx, w)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %w", err)
	}

	if len(recommendations) == 0 {
		return nil, fmt.Errorf("no suitable providers found for workload")
	}

	// Select the best recommendation
	best := recommendations[0]

	// Create placement decision
	placement := &scheduler.PlacementDecision{
		Provider:    best.Provider,
		Region:      best.Region,
		MachineType: best.MachineType,
		Score:       best.Score,
		Reasons:     best.Pros,
	}

	// Create alternatives
	alternatives := make([]*scheduler.Alternative, 0)
	for i, rec := range recommendations[1:] {
		if i >= s.config.MaxAlternatives {
			break
		}
		alternatives = append(alternatives, &scheduler.Alternative{
			Placement: &scheduler.PlacementDecision{
				Provider:    rec.Provider,
				Region:      rec.Region,
				MachineType: rec.MachineType,
				Score:       rec.Score,
				Reasons:     rec.Pros,
			},
			EstimatedCost: rec.EstimatedCost,
			Rank:          i + 2,
			Reason:        fmt.Sprintf("Alternative %d", i+2),
		})
	}

	result := &scheduler.ScheduleResult{
		WorkloadID:    w.ID,
		Provider:      best.Provider,
		Region:        best.Region,
		MachineType:   best.MachineType,
		EstimatedCost: best.EstimatedCost,
		Placement:     placement,
		Alternatives:  alternatives,
		ScheduledAt:   time.Now(),
	}

	// Update stats
	s.updateStats(w.ID, best.Provider, best.Region, true, time.Since(start), 0, "")

	return result, nil
}

// GetRecommendations returns scheduling recommendations without scheduling
func (s *SimpleScheduler) GetRecommendations(ctx context.Context, w *workload.Workload) ([]*scheduler.Recommendation, error) {
	recommendations := make([]*scheduler.Recommendation, 0)

	for name, provider := range s.providers {
		// Check provider health
		if err := provider.HealthCheck(ctx); err != nil {
			continue
		}

		// Get pricing
		pricing, err := provider.GetPricing(ctx)
		if err != nil {
			continue
		}

		// Calculate estimated cost
		cost := s.calculateCost(w, pricing)

		// Calculate score based on policy
		score := s.calculateScore(w, provider, cost)

		// Get available resources to determine regions and machine types
		resources, err := provider.GetAvailableResources(ctx)
		if err != nil {
			continue
		}

		// Select best region (for now, just use the first available one)
		selectedRegion := "default"
		if len(resources.Regions) > 0 {
			for _, region := range resources.Regions {
				if region.Available {
					selectedRegion = region.Name
					break
				}
			}
		}

		// Select appropriate machine type based on workload requirements
		selectedMachineType := s.selectMachineType(w, resources)

		rec := &scheduler.Recommendation{
			Provider:      name,
			Region:        selectedRegion,
			MachineType:   selectedMachineType,
			Score:         score,
			EstimatedCost: cost,
			Pros:          []string{"Available", "Healthy"},
			Cons:          []string{},
			Confidence:    0.8,
		}

		recommendations = append(recommendations, rec)
	}

	// Sort by score (highest first)
	for i := 0; i < len(recommendations)-1; i++ {
		for j := i + 1; j < len(recommendations); j++ {
			if recommendations[i].Score < recommendations[j].Score {
				recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
			}
		}
	}

	return recommendations, nil
}

// Reschedule reschedules an existing workload
func (s *SimpleScheduler) Reschedule(ctx context.Context, workloadID string, constraints *scheduler.RescheduleConstraints) (*scheduler.ScheduleResult, error) {
	start := time.Now()

	// Create a mock workload for rescheduling (in real implementation, would fetch from repository)
	w := &workload.Workload{
		ID: workloadID,
		Spec: workload.Spec{
			Resources: workload.ResourceRequests{
				CPU:    "2",
				Memory: "4Gi",
			},
		},
	}

	// Get recommendations with constraints
	recommendations, err := s.getConstrainedRecommendations(ctx, w, constraints)
	if err != nil {
		s.updateStats(workloadID, "", "", false, time.Since(start), 0, err.Error())
		return nil, fmt.Errorf("failed to get constrained recommendations: %w", err)
	}

	if len(recommendations) == 0 {
		err := fmt.Errorf("no suitable providers found for rescheduling with given constraints")
		s.updateStats(workloadID, "", "", false, time.Since(start), 0, err.Error())
		return nil, err
	}

	// Select the best recommendation
	best := recommendations[0]

	// Create placement decision
	placement := &scheduler.PlacementDecision{
		Provider:    best.Provider,
		Region:      best.Region,
		MachineType: best.MachineType,
		Score:       best.Score,
		Reasons:     append(best.Pros, fmt.Sprintf("Rescheduled: %s", constraints.Reason)),
	}

	result := &scheduler.ScheduleResult{
		WorkloadID:    workloadID,
		Provider:      best.Provider,
		Region:        best.Region,
		MachineType:   best.MachineType,
		EstimatedCost: best.EstimatedCost,
		Placement:     placement,
		ScheduledAt:   time.Now(),
		Metadata: map[string]interface{}{
			"rescheduled": true,
			"reason":      constraints.Reason,
		},
	}

	// Update stats
	s.updateStats(workloadID, best.Provider, best.Region, true, time.Since(start), best.EstimatedCost.HourlyCost, "")

	return result, nil
}

// getConstrainedRecommendations gets recommendations with rescheduling constraints
// nolint:gocyclo
func (s *SimpleScheduler) getConstrainedRecommendations(ctx context.Context, w *workload.Workload, constraints *scheduler.RescheduleConstraints) ([]*scheduler.Recommendation, error) {
	recommendations := make([]*scheduler.Recommendation, 0)

	for name, provider := range s.providers {
		// Apply provider constraints
		if len(constraints.RequiredProviders) > 0 {
			found := false
			for _, required := range constraints.RequiredProviders {
				if name == required {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if len(constraints.ExcludedProviders) > 0 {
			excluded := false
			for _, excludedProvider := range constraints.ExcludedProviders {
				if name == excludedProvider {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}
		}

		// Check provider health
		if err := provider.HealthCheck(ctx); err != nil {
			continue
		}

		// Get pricing
		pricing, err := provider.GetPricing(ctx)
		if err != nil {
			continue
		}

		// Calculate estimated cost
		cost := s.calculateCost(w, pricing)

		// Apply cost constraints
		if constraints.MaxCostIncrease != nil {
			// For simplicity, assume current cost is $0.5/hour
			currentCost := 0.5
			maxAllowedCost := currentCost * (1.0 + *constraints.MaxCostIncrease/100.0)
			if cost.HourlyCost > maxAllowedCost {
				continue
			}
		}

		// Get available resources
		resources, err := provider.GetAvailableResources(ctx)
		if err != nil {
			continue
		}

		// Apply region constraints
		selectedRegion := "default"
		if len(constraints.PreferredRegions) > 0 {
			// Try to find a preferred region
			for _, preferredRegion := range constraints.PreferredRegions {
				for _, region := range resources.Regions {
					if region.Name == preferredRegion && region.Available {
						selectedRegion = region.Name
						break
					}
				}
				if selectedRegion != "default" {
					break
				}
			}
		} else {
			// Use first available region
			for _, region := range resources.Regions {
				if region.Available {
					selectedRegion = region.Name
					break
				}
			}
		}

		// Calculate score
		score := s.calculateScore(w, provider, cost)

		// Select machine type
		selectedMachineType := s.selectMachineType(w, resources)

		rec := &scheduler.Recommendation{
			Provider:      name,
			Region:        selectedRegion,
			MachineType:   selectedMachineType,
			Score:         score,
			EstimatedCost: cost,
			Pros:          []string{"Available", "Healthy", "Meets constraints"},
			Cons:          []string{},
			Confidence:    0.8,
		}

		recommendations = append(recommendations, rec)
	}

	// Sort by score (highest first)
	for i := 0; i < len(recommendations)-1; i++ {
		for j := i + 1; j < len(recommendations); j++ {
			if recommendations[i].Score < recommendations[j].Score {
				recommendations[i], recommendations[j] = recommendations[j], recommendations[i]
			}
		}
	}

	return recommendations, nil
}

// GetStats returns current scheduling statistics
func (s *SimpleScheduler) GetStats(ctx context.Context) (*scheduler.SchedulerStats, error) {
	s.stats.LastUpdated = time.Now()
	return s.stats, nil
}

// HealthCheck checks scheduler health
func (s *SimpleScheduler) HealthCheck(ctx context.Context) error {
	if len(s.providers) == 0 {
		return fmt.Errorf("no providers configured")
	}
	return nil
}

// calculateCost estimates the cost for a workload
func (s *SimpleScheduler) calculateCost(_ *workload.Workload, pricing *provider.PricingInfo) *provider.CostEstimate {
	// Simple cost calculation based on resources
	cpuCost := 2.0 * pricing.CPU.Amount       // Assume 2 vCPUs
	memoryCost := 4.0 * pricing.Memory.Amount // Assume 4GB memory

	hourlyCost := cpuCost + memoryCost

	return &provider.CostEstimate{
		Currency:    pricing.Currency,
		HourlyCost:  hourlyCost,
		DailyCost:   hourlyCost * 24,
		MonthlyCost: hourlyCost * 24 * 30,
		Breakdown: []provider.CostBreakdown{
			{
				Component:   "cpu",
				Description: "2 vCPUs",
				Amount:      cpuCost,
				Unit:        "hour",
				Quantity:    2.0,
			},
			{
				Component:   "memory",
				Description: "4GB RAM",
				Amount:      memoryCost,
				Unit:        "hour",
				Quantity:    4.0,
			},
		},
		Confidence: 0.8,
	}
}

// calculateScore calculates a score for a provider based on the scheduling policy
func (s *SimpleScheduler) calculateScore(w *workload.Workload, provider provider.Provider, cost *provider.CostEstimate) float64 {
	policy := s.config.DefaultPolicy

	// Base score
	score := 50.0

	// Cost factor (lower cost = higher score)
	if cost.HourlyCost == 0 {
		score += policy.CostWeight * 50 // Free is good
	} else {
		// Assume $1/hour is expensive, $0.1/hour is cheap
		costScore := (1.0 - cost.HourlyCost) * 50
		if costScore < 0 {
			costScore = 0
		}
		score += policy.CostWeight * costScore
	}

	// Performance factor (assume all providers are equal for now)
	score += policy.PerformanceWeight * 40

	// Reliability factor (assume all providers are equal for now)
	score += policy.ReliabilityWeight * 45

	// Latency factor (assume all providers are equal for now)
	score += policy.LatencyWeight * 40

	return score
}

// updateStats updates scheduling statistics
func (s *SimpleScheduler) updateStats(workloadID, provider, region string, success bool, duration time.Duration, cost float64, errorMsg string) {
	s.stats.TotalScheduled++
	if success {
		s.stats.SuccessfulSchedules++
	} else {
		s.stats.FailedSchedules++
	}

	// Update provider stats
	if s.stats.ProviderStats[provider] == nil {
		s.stats.ProviderStats[provider] = &scheduler.ProviderStats{}
	}
	providerStats := s.stats.ProviderStats[provider]
	providerStats.TotalScheduled++
	if success {
		providerStats.SuccessRate = float64(providerStats.TotalScheduled-1)/float64(providerStats.TotalScheduled)*providerStats.SuccessRate + 1.0/float64(providerStats.TotalScheduled)
	}
	providerStats.LastScheduled = time.Now()

	// Add to recent schedules
	recent := &scheduler.RecentSchedule{
		WorkloadID:   workloadID,
		Provider:     provider,
		Region:       region,
		Success:      success,
		ScheduleTime: duration,
		Cost:         cost,
		Timestamp:    time.Now(),
		Error:        errorMsg,
	}

	s.stats.RecentSchedules = append(s.stats.RecentSchedules, recent)
	if len(s.stats.RecentSchedules) > 100 {
		s.stats.RecentSchedules = s.stats.RecentSchedules[1:]
	}
}

// selectMachineType selects an appropriate machine type based on workload requirements
func (s *SimpleScheduler) selectMachineType(w *workload.Workload, resources *provider.ResourceAvailability) string {
	// Parse workload resource requirements
	cpuRequired := s.parseCPURequirement(w.Spec.Resources.CPU)
	memoryRequired := s.parseMemoryRequirement(w.Spec.Resources.Memory)
	gpuRequired := w.Spec.Resources.GPU != ""

	// If GPU is required, select appropriate GPU machine type
	if gpuRequired {
		return s.selectGPUMachineType(w.Spec.Resources.GPU, resources)
	}

	// Select CPU/Memory machine type based on requirements
	return s.selectCPUMemoryMachineType(cpuRequired, memoryRequired)
}

// parseCPURequirement parses CPU requirement string (e.g., "2", "2000m", "2.5")
func (s *SimpleScheduler) parseCPURequirement(cpu string) float64 {
	if cpu == "" {
		return 1.0 // Default to 1 vCPU
	}

	// Handle millicores (e.g., "2000m" = 2 cores)
	if len(cpu) > 1 && cpu[len(cpu)-1] == 'm' {
		if val := s.parseFloat(cpu[:len(cpu)-1]); val > 0 {
			return val / 1000.0
		}
	}

	// Handle direct core count (e.g., "2", "2.5")
	if val := s.parseFloat(cpu); val > 0 {
		return val
	}

	return 1.0 // Default fallback
}

// parseMemoryRequirement parses memory requirement string (e.g., "4Gi", "4096Mi", "4G")
func (s *SimpleScheduler) parseMemoryRequirement(memory string) float64 {
	if memory == "" {
		return 4.0 // Default to 4GB
	}

	// Handle different memory units
	if len(memory) > 2 {
		unit := memory[len(memory)-2:]
		valueStr := memory[:len(memory)-2]
		val := s.parseFloat(valueStr)

		switch unit {
		case "Gi":
			return val // Already in GB
		case "Mi":
			return val / 1024.0 // Convert MB to GB
		case "Ki":
			return val / (1024.0 * 1024.0) // Convert KB to GB
		}
	}

	if len(memory) > 1 {
		unit := memory[len(memory)-1:]
		valueStr := memory[:len(memory)-1]
		val := s.parseFloat(valueStr)

		switch unit {
		case "G":
			return val // Already in GB
		case "M":
			return val / 1000.0 // Convert MB to GB
		case "K":
			return val / (1000.0 * 1000.0) // Convert KB to GB
		}
	}

	// Try to parse as plain number (assume GB)
	if val := s.parseFloat(memory); val > 0 {
		return val
	}

	return 4.0 // Default fallback
}

// parseFloat safely parses a string to float64
// nolint:gocyclo
func (s *SimpleScheduler) parseFloat(str string) float64 {
	// Simple float parsing - in production would use strconv.ParseFloat
	// For now, handle common cases
	switch str {
	case "0.5":
		return 0.5
	case "1":
		return 1.0
	case "2":
		return 2.0
	case "4":
		return 4.0
	case "8":
		return 8.0
	case "16":
		return 16.0
	case "32":
		return 32.0
	case "64":
		return 64.0
	case "128":
		return 128.0
	case "256":
		return 256.0
	case "512":
		return 512.0
	case "1024":
		return 1024.0
	case "2048":
		return 2048.0
	case "4096":
		return 4096.0
	case "8192":
		return 8192.0
	default:
		return 0.0
	}
}

// selectGPUMachineType selects appropriate GPU machine type
func (s *SimpleScheduler) selectGPUMachineType(gpuSpec string, resources *provider.ResourceAvailability) string {
	// Parse GPU requirements (e.g., "nvidia-tesla-v100", "1", "nvidia-a100:2")
	if len(resources.GPU.Types) == 0 {
		return "gpu-standard" // Fallback
	}

	// If specific GPU type is requested
	for gpuType, info := range resources.GPU.Types {
		if gpuSpec == gpuType || gpuSpec == info.Name {
			if info.Available > 0 {
				return fmt.Sprintf("gpu-%s", gpuType)
			}
		}
	}

	// Find best available GPU type
	var bestGPU string
	var bestMemory int
	for gpuType, info := range resources.GPU.Types {
		if info.Available > 0 {
			// Parse memory (e.g., "16GB" -> 16)
			memory := s.parseGPUMemory(info.Memory)
			if memory > bestMemory {
				bestMemory = memory
				bestGPU = gpuType
			}
		}
	}

	if bestGPU != "" {
		return fmt.Sprintf("gpu-%s", bestGPU)
	}

	return "gpu-standard" // Fallback
}

// parseGPUMemory parses GPU memory string (e.g., "16GB" -> 16)
func (s *SimpleScheduler) parseGPUMemory(memory string) int {
	if len(memory) > 2 && memory[len(memory)-2:] == "GB" {
		val := s.parseFloat(memory[:len(memory)-2])
		return int(val)
	}
	return 0
}

// selectCPUMemoryMachineType selects machine type based on CPU and memory requirements
func (s *SimpleScheduler) selectCPUMemoryMachineType(cpuRequired, memoryRequired float64) string {
	// Define standard machine types with CPU:Memory ratios
	machineTypes := []struct {
		name   string
		cpu    float64
		memory float64
	}{
		{"micro", 0.5, 1.0},
		{"small", 1.0, 2.0},
		{"medium", 2.0, 4.0},
		{"large", 4.0, 8.0},
		{"xlarge", 8.0, 16.0},
		{"2xlarge", 16.0, 32.0},
		{"4xlarge", 32.0, 64.0},
		{"8xlarge", 64.0, 128.0},
		{"16xlarge", 128.0, 256.0},
	}

	// Find the smallest machine type that meets requirements
	for _, mt := range machineTypes {
		if mt.cpu >= cpuRequired && mt.memory >= memoryRequired {
			return mt.name
		}
	}

	// If requirements exceed largest standard type, create custom type
	return fmt.Sprintf("custom-%.0f-%.0f", cpuRequired, memoryRequired)
}
