package simple

import (
	"context"
	"fabric/internal/scheduler"
	"fabric/internal/types"
	"fmt"
	"time"
)

// SimpleScheduler implements a basic cost-aware scheduler
type SimpleScheduler struct {
	providers map[string]types.Provider
	config    *scheduler.SchedulerConfig
	stats     *scheduler.SchedulerStats
}

// New creates a new simple scheduler
func New(providers map[string]types.Provider, config *scheduler.SchedulerConfig) *SimpleScheduler {
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
		providers: providers,
		config:    config,
		stats: &scheduler.SchedulerStats{
			ProviderStats:   make(map[string]*scheduler.ProviderStats),
			RecentSchedules: make([]*scheduler.RecentSchedule, 0),
			LastUpdated:     time.Now(),
		},
	}
}

// Schedule schedules a workload across available providers
func (s *SimpleScheduler) Schedule(ctx context.Context, workload *types.Workload) (*scheduler.ScheduleResult, error) {
	start := time.Now()

	// Get recommendations
	recommendations, err := s.GetRecommendations(ctx, workload)
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
		WorkloadID:    workload.ID,
		Provider:      best.Provider,
		Region:        best.Region,
		MachineType:   best.MachineType,
		EstimatedCost: best.EstimatedCost,
		Placement:     placement,
		Alternatives:  alternatives,
		ScheduledAt:   time.Now(),
	}

	// Update stats
	s.updateStats(workload.ID, best.Provider, best.Region, true, time.Since(start), 0, "")

	return result, nil
}

// GetRecommendations returns scheduling recommendations without scheduling
func (s *SimpleScheduler) GetRecommendations(ctx context.Context, workload *types.Workload) ([]*scheduler.Recommendation, error) {
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
		cost := s.calculateCost(workload, pricing)

		// Calculate score based on policy
		score := s.calculateScore(workload, provider, cost)

		rec := &scheduler.Recommendation{
			Provider:      name,
			Region:        "default", // TODO: Get actual regions
			MachineType:   "default", // TODO: Select appropriate machine type
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
	// TODO: Implement rescheduling logic
	return nil, fmt.Errorf("reschedule not implemented")
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
func (s *SimpleScheduler) calculateCost(workload *types.Workload, pricing *types.PricingInfo) *types.CostEstimate {
	// Simple cost calculation based on resources
	cpuCost := 2.0 * pricing.CPU.Amount       // Assume 2 vCPUs
	memoryCost := 4.0 * pricing.Memory.Amount // Assume 4GB memory

	hourlyCost := cpuCost + memoryCost

	return &types.CostEstimate{
		Currency:    pricing.Currency,
		HourlyCost:  hourlyCost,
		DailyCost:   hourlyCost * 24,
		MonthlyCost: hourlyCost * 24 * 30,
		Breakdown: []types.CostBreakdown{
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
func (s *SimpleScheduler) calculateScore(workload *types.Workload, provider types.Provider, cost *types.CostEstimate) float64 {
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
