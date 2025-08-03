package scheduler

import (
	"context"
	"time"

	"github.com/codecflow/fabric/pkg/workload"
	"github.com/codecflow/fabric/weaver/services/provider"
)

// Scheduler defines the interface for workload scheduling
type Scheduler interface {
	// Schedule a workload across available providers
	Schedule(ctx context.Context, workload *workload.Workload) (*ScheduleResult, error)

	// Get scheduling recommendations without actually scheduling
	GetRecommendations(ctx context.Context, workload *workload.Workload) ([]*Recommendation, error)

	// Reschedule an existing workload (for migration/optimization)
	Reschedule(ctx context.Context, workloadID string, constraints *RescheduleConstraints) (*ScheduleResult, error)

	// Get current scheduling statistics
	GetStats(ctx context.Context) (*SchedulerStats, error)

	// Health check
	HealthCheck(ctx context.Context) error
}

// ScheduleResult represents the result of a scheduling operation
type ScheduleResult struct {
	WorkloadID    string                 `json:"workloadId"`
	Provider      string                 `json:"provider"`
	Region        string                 `json:"region"`
	MachineType   string                 `json:"machineType"`
	EstimatedCost *provider.CostEstimate `json:"estimatedCost,omitempty"`
	Placement     *PlacementDecision     `json:"placement"`
	Alternatives  []*Alternative         `json:"alternatives,omitempty"`
	ScheduledAt   time.Time              `json:"scheduledAt"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// PlacementDecision contains detailed placement information
type PlacementDecision struct {
	Provider    string            `json:"provider"`
	Region      string            `json:"region"`
	Zone        string            `json:"zone,omitempty"`
	MachineType string            `json:"machineType"`
	NodeID      string            `json:"nodeId,omitempty"`
	Score       float64           `json:"score"` // 0-100, higher is better
	Reasons     []string          `json:"reasons"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// Alternative represents an alternative scheduling option
type Alternative struct {
	Placement     *PlacementDecision     `json:"placement"`
	EstimatedCost *provider.CostEstimate `json:"estimatedCost,omitempty"`
	Rank          int                    `json:"rank"` // 1 = best alternative
	Reason        string                 `json:"reason"`
}

// Recommendation represents a scheduling recommendation
type Recommendation struct {
	Provider      string                 `json:"provider"`
	Region        string                 `json:"region"`
	MachineType   string                 `json:"machineType"`
	Score         float64                `json:"score"`
	EstimatedCost *provider.CostEstimate `json:"estimatedCost,omitempty"`
	Pros          []string               `json:"pros"`
	Cons          []string               `json:"cons"`
	Confidence    float64                `json:"confidence"` // 0-1
}

// RescheduleConstraints defines constraints for rescheduling
type RescheduleConstraints struct {
	MaxCostIncrease   *float64       `json:"maxCostIncrease,omitempty"` // Percentage
	RequiredProviders []string       `json:"requiredProviders,omitempty"`
	ExcludedProviders []string       `json:"excludedProviders,omitempty"`
	MaxMigrationTime  *time.Duration `json:"maxMigrationTime,omitempty"`
	PreferredRegions  []string       `json:"preferredRegions,omitempty"`
	Reason            string         `json:"reason,omitempty"`
}

// SchedulerStats represents scheduler performance statistics
type SchedulerStats struct {
	TotalScheduled      int64                     `json:"totalScheduled"`
	SuccessfulSchedules int64                     `json:"successfulSchedules"`
	FailedSchedules     int64                     `json:"failedSchedules"`
	AverageScheduleTime time.Duration             `json:"averageScheduleTime"`
	ProviderStats       map[string]*ProviderStats `json:"providerStats"`
	RecentSchedules     []*RecentSchedule         `json:"recentSchedules,omitempty"`
	LastUpdated         time.Time                 `json:"lastUpdated"`
}

// ProviderStats represents statistics for a specific provider
type ProviderStats struct {
	TotalScheduled int64         `json:"totalScheduled"`
	SuccessRate    float64       `json:"successRate"`
	AverageCost    float64       `json:"averageCost"`
	AverageLatency time.Duration `json:"averageLatency"`
	Utilization    float64       `json:"utilization"` // 0-1
	LastScheduled  time.Time     `json:"lastScheduled"`
}

// RecentSchedule represents a recent scheduling decision
type RecentSchedule struct {
	WorkloadID   string        `json:"workloadId"`
	Provider     string        `json:"provider"`
	Region       string        `json:"region"`
	Success      bool          `json:"success"`
	ScheduleTime time.Duration `json:"scheduleTime"`
	Cost         float64       `json:"cost,omitempty"`
	Timestamp    time.Time     `json:"timestamp"`
	Error        string        `json:"error,omitempty"`
}

// SchedulingPolicy defines how scheduling decisions are made
type SchedulingPolicy struct {
	Strategy           SchedulingStrategy `json:"strategy"`
	CostWeight         float64            `json:"costWeight"`        // 0-1
	PerformanceWeight  float64            `json:"performanceWeight"` // 0-1
	ReliabilityWeight  float64            `json:"reliabilityWeight"` // 0-1
	LatencyWeight      float64            `json:"latencyWeight"`     // 0-1
	MaxCostPerHour     *float64           `json:"maxCostPerHour,omitempty"`
	PreferredProviders []string           `json:"preferredProviders,omitempty"`
	ExcludedProviders  []string           `json:"excludedProviders,omitempty"`
	RequireGPU         bool               `json:"requireGpu,omitempty"`
	MinMemoryGB        *int               `json:"minMemoryGb,omitempty"`
	MinCPUCores        *int               `json:"minCpuCores,omitempty"`
}

// SchedulingStrategy defines the scheduling strategy
type SchedulingStrategy string

const (
	StrategyLowestCost       SchedulingStrategy = "lowest_cost"
	StrategyBestPerformance  SchedulingStrategy = "best_performance"
	StrategyBalanced         SchedulingStrategy = "balanced"
	StrategyHighAvailability SchedulingStrategy = "high_availability"
	StrategyCustom           SchedulingStrategy = "custom"
)

// SchedulerConfig defines configuration for the scheduler
type SchedulerConfig struct {
	DefaultPolicy      SchedulingPolicy  `json:"defaultPolicy"`
	MaxAlternatives    int               `json:"maxAlternatives"`
	ScheduleTimeout    time.Duration     `json:"scheduleTimeout"`
	CostUpdateInterval time.Duration     `json:"costUpdateInterval"`
	EnablePreemption   bool              `json:"enablePreemption"`
	PreemptionPolicy   *PreemptionPolicy `json:"preemptionPolicy,omitempty"`
	Properties         map[string]string `json:"properties,omitempty"`
}

// PreemptionPolicy defines when and how to preempt workloads
type PreemptionPolicy struct {
	Enabled            bool          `json:"enabled"`
	MaxCostSavings     float64       `json:"maxCostSavings"` // Percentage
	MinIdleTime        time.Duration `json:"minIdleTime"`    // Minimum idle time before preemption
	GracePeriod        time.Duration `json:"gracePeriod"`    // Grace period for workload shutdown
	ExcludedNamespaces []string      `json:"excludedNamespaces,omitempty"`
	ExcludedWorkloads  []string      `json:"excludedWorkloads,omitempty"`
}

// SchedulingEvent represents a scheduling event
type SchedulingEvent struct {
	Type       SchedulingEventType    `json:"type"`
	WorkloadID string                 `json:"workloadId"`
	Provider   string                 `json:"provider,omitempty"`
	Region     string                 `json:"region,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

// SchedulingEventType defines types of scheduling events
type SchedulingEventType string

const (
	EventScheduleRequested   SchedulingEventType = "schedule_requested"
	EventScheduleSucceeded   SchedulingEventType = "schedule_succeeded"
	EventScheduleFailed      SchedulingEventType = "schedule_failed"
	EventRescheduleRequested SchedulingEventType = "reschedule_requested"
	EventRescheduleSucceeded SchedulingEventType = "reschedule_succeeded"
	EventRescheduleFailed    SchedulingEventType = "reschedule_failed"
	EventPreemptionTriggered SchedulingEventType = "preemption_triggered"
	EventCostOptimization    SchedulingEventType = "cost_optimization"
)

// ResourceRequirements represents resource requirements for scheduling
type ResourceRequirements struct {
	CPU     *ResourceSpec `json:"cpu,omitempty"`
	Memory  *ResourceSpec `json:"memory,omitempty"`
	GPU     *GPUSpec      `json:"gpu,omitempty"`
	Storage *StorageSpec  `json:"storage,omitempty"`
	Network *NetworkSpec  `json:"network,omitempty"`
}

// ResourceSpec defines a resource specification
type ResourceSpec struct {
	Min       float64 `json:"min"`
	Max       float64 `json:"max,omitempty"`
	Preferred float64 `json:"preferred,omitempty"`
	Unit      string  `json:"unit"`
}

// GPUSpec defines GPU requirements
type GPUSpec struct {
	Count    int    `json:"count"`
	Type     string `json:"type,omitempty"`    // "nvidia-tesla-v100", "nvidia-a100"
	Memory   int    `json:"memory,omitempty"`  // GB
	Compute  string `json:"compute,omitempty"` // "fp16", "fp32", "int8"
	Required bool   `json:"required"`
}

// StorageSpec defines storage requirements
type StorageSpec struct {
	Size       int64  `json:"size"` // Bytes
	Type       string `json:"type"` // "ssd", "hdd", "nvme"
	IOPS       int    `json:"iops,omitempty"`
	Throughput int64  `json:"throughput,omitempty"` // MB/s
}

// NetworkSpec defines network requirements
type NetworkSpec struct {
	Bandwidth int64         `json:"bandwidth,omitempty"` // Mbps
	Latency   time.Duration `json:"latency,omitempty"`
	PublicIP  bool          `json:"publicIp,omitempty"`
}
