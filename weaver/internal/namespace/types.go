package namespace

import (
	"time"
	"weaver/internal/workload"
)

// Spec defines the desired state of a namespace
type Spec struct {
	// Resource quotas
	Quotas ResourceQuotas `json:"quotas,omitempty"`

	// Network policies
	NetworkPolicy NetworkPolicy `json:"networkPolicy,omitempty"`

	// Default placement preferences
	DefaultPlacement workload.PlacementSpec `json:"defaultPlacement,omitempty"`
}

// ResourceQuotas defines resource limits for a namespace
type ResourceQuotas struct {
	MaxWorkloads int    `json:"maxWorkloads,omitempty"`
	MaxCPU       string `json:"maxCpu,omitempty"`     // e.g. "10" or "10000m"
	MaxMemory    string `json:"maxMemory,omitempty"`  // e.g. "20Gi"
	MaxGPU       string `json:"maxGpu,omitempty"`     // e.g. "4"
	MaxStorage   string `json:"maxStorage,omitempty"` // e.g. "100Gi"
}

// NetworkPolicy defines network isolation rules
type NetworkPolicy struct {
	Isolation NetworkIsolation `json:"isolation,omitempty"`
	Ingress   []NetworkRule    `json:"ingress,omitempty"`
	Egress    []NetworkRule    `json:"egress,omitempty"`
}

// NetworkIsolation defines the level of network isolation
type NetworkIsolation string

const (
	NetworkIsolationNone   NetworkIsolation = "None"   // No isolation
	NetworkIsolationStrict NetworkIsolation = "Strict" // Full isolation
	NetworkIsolationCustom NetworkIsolation = "Custom" // Custom rules
)

// NetworkRule defines a network access rule
type NetworkRule struct {
	From      []NetworkPeer   `json:"from,omitempty"`
	To        []NetworkPeer   `json:"to,omitempty"`
	Ports     []workload.Port `json:"ports,omitempty"`
	Protocols []string        `json:"protocols,omitempty"`
}

// NetworkPeer defines a network endpoint
type NetworkPeer struct {
	NamespaceSelector map[string]string `json:"namespaceSelector,omitempty"`
	WorkloadSelector  map[string]string `json:"workloadSelector,omitempty"`
	IPBlock           *IPBlock          `json:"ipBlock,omitempty"`
}

// IPBlock defines an IP address range
type IPBlock struct {
	CIDR   string   `json:"cidr"`
	Except []string `json:"except,omitempty"`
}

// Status represents the current state of a namespace
type Status struct {
	Phase   Phase  `json:"phase"`
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`

	// Resource usage
	Usage ResourceUsage `json:"usage,omitempty"`

	// Tailscale information
	TailscaleTag string `json:"tailscaleTag,omitempty"`
}

// Phase represents the lifecycle phase
type Phase string

const (
	PhasePending     Phase = "Pending"
	PhaseActive      Phase = "Active"
	PhaseTerminating Phase = "Terminating"
	PhaseUnknown     Phase = "Unknown"
)

// ResourceUsage tracks current resource consumption
type ResourceUsage struct {
	Workloads int    `json:"workloads"`
	CPU       string `json:"cpu,omitempty"`
	Memory    string `json:"memory,omitempty"`
	GPU       string `json:"gpu,omitempty"`
	Storage   string `json:"storage,omitempty"`
}

// Namespace represents a complete namespace definition
type Namespace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	Spec   Spec   `json:"spec"`
	Status Status `json:"status"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// List represents a list of namespaces
type List struct {
	Items []Namespace `json:"items"`
	Total int         `json:"total"`
}

// Filter defines filters for namespace queries
type Filter struct {
	Labels map[string]string
	Phase  Phase
}
