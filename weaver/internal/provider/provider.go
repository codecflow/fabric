package provider

import (
	"context"

	"github.com/codecflow/fabric/weaver/internal/workload"
)

// Provider defines the interface for cloud providers
type Provider interface {
	// Provider metadata
	Name() string
	Type() ProviderType

	// Workload lifecycle
	CreateWorkload(ctx context.Context, workload *workload.Workload) error
	GetWorkload(ctx context.Context, id string) (*workload.Workload, error)
	UpdateWorkload(ctx context.Context, workload *workload.Workload) error
	DeleteWorkload(ctx context.Context, id string) error
	ListWorkloads(ctx context.Context, namespace string) ([]*workload.Workload, error)

	// Resource management
	GetAvailableResources(ctx context.Context) (*ResourceAvailability, error)
	GetPricing(ctx context.Context) (*PricingInfo, error)

	// Health and status
	HealthCheck(ctx context.Context) error
	GetStatus(ctx context.Context) (*ProviderStatus, error)
}

// ProviderType defines the type of provider
type ProviderType string

// ResourceAvailability represents available resources on a provider
type ResourceAvailability struct {
	CPU    ResourcePool `json:"cpu"`
	Memory ResourcePool `json:"memory"`
	GPU    GPUPool      `json:"gpu"`

	Regions []RegionInfo `json:"regions"`
}

// ResourcePool represents a pool of compute resources
type ResourcePool struct {
	Total     string `json:"total"`
	Available string `json:"available"`
	Used      string `json:"used"`
}

// GPUPool represents available GPU resources
type GPUPool struct {
	Types map[string]GPUTypeInfo `json:"types"` // e.g. "nvidia-a100": {...}
}

// GPUTypeInfo represents information about a specific GPU type
type GPUTypeInfo struct {
	Name         string  `json:"name"`
	Memory       string  `json:"memory"`
	Total        int     `json:"total"`
	Available    int     `json:"available"`
	PricePerHour float64 `json:"pricePerHour"`
}

// RegionInfo represents information about a provider region
type RegionInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Zones       []string `json:"zones"`
	Available   bool     `json:"available"`
	GPUTypes    []string `json:"gpuTypes"`
}

// PricingInfo represents pricing information for a provider
type PricingInfo struct {
	Currency string                  `json:"currency"`
	CPU      PricePerUnit            `json:"cpu"`     // per vCPU hour
	Memory   PricePerUnit            `json:"memory"`  // per GB hour
	GPU      map[string]PricePerUnit `json:"gpu"`     // per GPU hour by type
	Storage  PricePerUnit            `json:"storage"` // per GB month
	Network  NetworkPricing          `json:"network"`
}

// PricePerUnit represents pricing for a resource unit
type PricePerUnit struct {
	Amount float64 `json:"amount"`
	Unit   string  `json:"unit"` // "hour", "month", "gb"
}

// NetworkPricing represents network pricing
type NetworkPricing struct {
	Ingress  PricePerUnit `json:"ingress"`  // per GB
	Egress   PricePerUnit `json:"egress"`   // per GB
	Internal PricePerUnit `json:"internal"` // per GB (usually free)
}

// ProviderStatus represents the current status of a provider
type ProviderStatus struct {
	Available bool            `json:"available"`
	Message   string          `json:"message,omitempty"`
	Regions   []RegionStatus  `json:"regions"`
	Metrics   ProviderMetrics `json:"metrics"`
}

// RegionStatus represents the status of a specific region
type RegionStatus struct {
	Name      string  `json:"name"`
	Available bool    `json:"available"`
	Load      float64 `json:"load"`    // 0.0 to 1.0
	Latency   int     `json:"latency"` // ms
}

// ProviderMetrics represents performance metrics for a provider
type ProviderMetrics struct {
	ActiveWorkloads  int     `json:"activeWorkloads"`
	TotalWorkloads   int     `json:"totalWorkloads"`
	SuccessRate      float64 `json:"successRate"`      // 0.0 to 1.0
	AverageStartTime int     `json:"averageStartTime"` // seconds
	AverageLatency   int     `json:"averageLatency"`   // ms
}

// ProviderConfig represents configuration for a provider
type ProviderConfig struct {
	Type     ProviderType      `json:"type"`
	Name     string            `json:"name"`
	Enabled  bool              `json:"enabled"`
	Priority int               `json:"priority"` // Higher = preferred
	Config   map[string]string `json:"config"`   // Provider-specific config

	// Resource limits
	MaxWorkloads int    `json:"maxWorkloads,omitempty"`
	MaxCPU       string `json:"maxCpu,omitempty"`
	MaxMemory    string `json:"maxMemory,omitempty"`
	MaxGPU       string `json:"maxGpu,omitempty"`
}

// SchedulingHint provides hints for workload placement
type SchedulingHint struct {
	PreferredProviders []string          `json:"preferredProviders,omitempty"`
	AvoidProviders     []string          `json:"avoidProviders,omitempty"`
	CostOptimized      bool              `json:"costOptimized,omitempty"`
	PerformanceFirst   bool              `json:"performanceFirst,omitempty"`
	Affinity           map[string]string `json:"affinity,omitempty"`
	AntiAffinity       map[string]string `json:"antiAffinity,omitempty"`
}

// CostEstimate represents estimated costs for a workload
type CostEstimate struct {
	Currency    string          `json:"currency"`
	HourlyCost  float64         `json:"hourlyCost"`
	DailyCost   float64         `json:"dailyCost"`
	MonthlyCost float64         `json:"monthlyCost"`
	Breakdown   []CostBreakdown `json:"breakdown"`
	Confidence  float64         `json:"confidence"` // 0-1
	ValidUntil  string          `json:"validUntil,omitempty"`
	Assumptions []string        `json:"assumptions,omitempty"`
}

// CostBreakdown represents a breakdown of costs by component
type CostBreakdown struct {
	Component   string  `json:"component"` // "cpu", "memory", "gpu", "storage", "network"
	Description string  `json:"description"`
	Amount      float64 `json:"amount"`
	Unit        string  `json:"unit"`
	Quantity    float64 `json:"quantity"`
}
