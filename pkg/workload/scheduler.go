package workload

import (
	"fmt"

	"github.com/codecflow/fabric/pkg/provider"
	"github.com/codecflow/fabric/pkg/types"
)

// Scheduler selects the best provider for a workload
type Scheduler struct {
	providers []provider.WorkloadProvider
	policy    SchedulingPolicy
}

// SchedulingPolicy defines how to select a provider
type SchedulingPolicy string

const (
	PolicyFirstAvailable SchedulingPolicy = "first_available"
	PolicyLeastCost      SchedulingPolicy = "least_cost"
	PolicyBestFit        SchedulingPolicy = "best_fit"
)

// NewScheduler creates a new scheduler
func NewScheduler() *Scheduler {
	return &Scheduler{
		providers: make([]provider.WorkloadProvider, 0),
		policy:    PolicyFirstAvailable,
	}
}

// AddProvider adds a provider to the scheduler
func (s *Scheduler) AddProvider(p provider.WorkloadProvider) {
	s.providers = append(s.providers, p)
}

// SelectProvider selects the best provider for a workload
func (s *Scheduler) SelectProvider(workload *types.Workload) (provider.WorkloadProvider, error) {
	if len(s.providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	switch s.policy {
	case PolicyFirstAvailable:
		return s.selectFirstAvailable(workload)
	case PolicyBestFit:
		return s.selectBestFit(workload)
	default:
		return s.selectFirstAvailable(workload)
	}
}

// selectFirstAvailable returns the first provider that can handle the workload
func (s *Scheduler) selectFirstAvailable(workload *types.Workload) (provider.WorkloadProvider, error) {
	for _, p := range s.providers {
		if s.canHandle(p, workload) {
			return p, nil
		}
	}
	return nil, fmt.Errorf("no provider can handle this workload")
}

// selectBestFit returns the provider that best matches the workload requirements
func (s *Scheduler) selectBestFit(workload *types.Workload) (provider.WorkloadProvider, error) {
	var bestProvider provider.WorkloadProvider
	var bestScore int

	for _, p := range s.providers {
		if !s.canHandle(p, workload) {
			continue
		}

		score := s.scoreProvider(p, workload)
		if score > bestScore {
			bestScore = score
			bestProvider = p
		}
	}

	if bestProvider == nil {
		return nil, fmt.Errorf("no provider can handle this workload")
	}

	return bestProvider, nil
}

// canHandle checks if a provider can handle a workload
func (s *Scheduler) canHandle(p provider.WorkloadProvider, workload *types.Workload) bool {
	caps := p.Capabilities()

	// Check GPU requirement
	if workload.GPU != nil && workload.GPU.Count > 0 {
		if !caps.GPU {
			return false
		}

		// Check GPU type if specified
		if workload.GPU.Type != "" {
			found := false
			for _, gpuType := range caps.GPUTypes {
				if gpuType == workload.GPU.Type {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Check resource limits
	if caps.MaxCPU > 0 && workload.CPU > caps.MaxCPU {
		return false
	}

	if caps.MaxMemory > 0 && workload.Memory > caps.MaxMemory {
		return false
	}

	// Check volume support if needed
	if len(workload.Volumes) > 0 && !caps.Volumes {
		return false
	}

	return true
}

// scoreProvider scores a provider for a workload (higher is better)
func (s *Scheduler) scoreProvider(p provider.WorkloadProvider, workload *types.Workload) int {
	score := 0
	caps := p.Capabilities()

	// Prefer providers with exact GPU match
	if workload.GPU != nil && workload.GPU.Type != "" {
		for _, gpuType := range caps.GPUTypes {
			if gpuType == workload.GPU.Type {
				score += 10
				break
			}
		}
	}

	// Prefer providers with more capabilities
	if caps.Volumes {
		score += 1
	}
	if caps.Networking {
		score += 1
	}
	if caps.Checkpoint {
		score += 1
	}
	if caps.Autoscaling {
		score += 1
	}

	return score
}

// SetPolicy sets the scheduling policy
func (s *Scheduler) SetPolicy(policy SchedulingPolicy) {
	s.policy = policy
}
