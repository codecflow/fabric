package types

import (
	"time"
)

// NamespaceSpec defines the desired state of a namespace
type NamespaceSpec struct {
	// Resource quotas
	Quotas ResourceQuotas `json:"quotas,omitempty"`

	// Network policies
	NetworkPolicy NetworkPolicy `json:"networkPolicy,omitempty"`

	// Default placement preferences
	DefaultPlacement PlacementSpec `json:"defaultPlacement,omitempty"`
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
	From      []NetworkPeer `json:"from,omitempty"`
	To        []NetworkPeer `json:"to,omitempty"`
	Ports     []Port        `json:"ports,omitempty"`
	Protocols []string      `json:"protocols,omitempty"`
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

// NamespaceStatus represents the current state of a namespace
type NamespaceStatus struct {
	Phase   NamespacePhase `json:"phase"`
	Message string         `json:"message,omitempty"`
	Reason  string         `json:"reason,omitempty"`

	// Resource usage
	Usage ResourceUsage `json:"usage,omitempty"`

	// Tailscale information
	TailscaleTag string `json:"tailscaleTag,omitempty"`
}

// NamespacePhase represents the lifecycle phase
type NamespacePhase string

const (
	NamespacePhasePending     NamespacePhase = "Pending"
	NamespacePhaseActive      NamespacePhase = "Active"
	NamespacePhaseTerminating NamespacePhase = "Terminating"
	NamespacePhaseUnknown     NamespacePhase = "Unknown"
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

	Spec   NamespaceSpec   `json:"spec"`
	Status NamespaceStatus `json:"status"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// NamespaceList represents a list of namespaces
type NamespaceList struct {
	Items []Namespace `json:"items"`
	Total int         `json:"total"`
}
