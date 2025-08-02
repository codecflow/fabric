package workload

import (
	"time"
)

// Spec defines the specification for a workload
type Spec struct {
	Image     string            `json:"image"`
	Command   []string          `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Resources ResourceRequests  `json:"resources"`
	Volumes   []VolumeMount     `json:"volumes,omitempty"`
	Ports     []Port            `json:"ports,omitempty"`
	Sidecars  []SidecarSpec     `json:"sidecars,omitempty"`
	Restart   RestartPolicy     `json:"restart,omitempty"`
	Placement PlacementSpec     `json:"placement"`
}

// ResourceRequests specifies compute resource requirements
type ResourceRequests struct {
	CPU    string `json:"cpu,omitempty"`    // e.g. "2" or "2000m"
	Memory string `json:"memory,omitempty"` // e.g. "4Gi"
	GPU    string `json:"gpu,omitempty"`    // e.g. "1" or "nvidia.com/gpu=1"
}

// VolumeMount defines a volume mount point
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mountPath"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
	ContentID string `json:"contentId,omitempty"` // Iroh CID
}

// Port defines a network port
type Port struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"` // TCP, UDP
}

// SidecarSpec defines a sidecar container
type SidecarSpec struct {
	Name    string            `json:"name"`
	Image   string            `json:"image"`
	Command []string          `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// RestartPolicy defines restart behavior
type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "Always"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
	RestartPolicyNever     RestartPolicy = "Never"
)

// PlacementSpec defines placement constraints
type PlacementSpec struct {
	Provider    string            `json:"provider,omitempty"`    // "k8s", "runpod", "coreweave"
	Region      string            `json:"region,omitempty"`      // "us-east-1"
	Zone        string            `json:"zone,omitempty"`        // "us-east-1a"
	NodeLabels  map[string]string `json:"nodeLabels,omitempty"`  // Node selector
	Tolerations []Toleration      `json:"tolerations,omitempty"` // Taints to tolerate
}

// Toleration allows scheduling on nodes with matching taints
type Toleration struct {
	Key      string `json:"key"`
	Operator string `json:"operator"` // Equal, Exists
	Value    string `json:"value,omitempty"`
	Effect   string `json:"effect,omitempty"` // NoSchedule, PreferNoSchedule, NoExecute
}

// Status represents the current state of a workload
type Status struct {
	Phase        Phase      `json:"phase"`
	Message      string     `json:"message,omitempty"`
	Reason       string     `json:"reason,omitempty"`
	StartTime    *time.Time `json:"startTime,omitempty"`
	FinishTime   *time.Time `json:"finishTime,omitempty"`
	RestartCount int32      `json:"restartCount"`

	// Runtime information
	NodeID      string `json:"nodeId,omitempty"`
	Provider    string `json:"provider,omitempty"`
	TailscaleIP string `json:"tailscaleIp,omitempty"`
	ContainerID string `json:"containerId,omitempty"`

	// Snapshot information
	SnapshotID   string     `json:"snapshotId,omitempty"`
	LastSnapshot *time.Time `json:"lastSnapshot,omitempty"`
}

// Phase represents the lifecycle phase
type Phase string

const (
	PhasePending   Phase = "Pending"
	PhaseScheduled Phase = "Scheduled"
	PhaseRunning   Phase = "Running"
	PhaseSucceeded Phase = "Succeeded"
	PhaseFailed    Phase = "Failed"
	PhaseUnknown   Phase = "Unknown"
)

// Workload represents a complete workload definition
type Workload struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	Spec   Spec   `json:"spec"`
	Status Status `json:"status"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// List represents a list of workloads
type List struct {
	Items []Workload `json:"items"`
	Total int        `json:"total"`
}

// Filter defines filters for workload queries
type Filter struct {
	Namespace string
	Labels    map[string]string
	Phase     Phase
	Provider  string
	NodeID    string
}
