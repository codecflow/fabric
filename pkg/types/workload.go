package types

import "time"

// Workload represents a user-defined compute task
type Workload struct {
	ID          string
	WorkspaceID string
	Name        string
	Type        WorkloadType

	// Image specification
	Image *ImageSpec

	// Resources
	CPU    int64 // millicores
	Memory int64 // bytes
	GPU    *GPUSpec

	// Execution
	Command    []string
	Args       []string
	Env        map[string]string
	WorkingDir string

	// Networking
	Ports []PortSpec

	// Storage
	Volumes []VolumeMount

	// Configuration
	Secrets []string
	Timeout time.Duration

	// Metadata
	Labels      map[string]string
	Annotations map[string]string
	CreatedAt   time.Time
}

// WorkloadType defines the type of workload
type WorkloadType string

const (
	WorkloadTypeFunction WorkloadType = "function"
	WorkloadTypeEndpoint WorkloadType = "endpoint"
	WorkloadTypeTask     WorkloadType = "task"
	WorkloadTypePod      WorkloadType = "pod"
)

// ImageSpec describes how to obtain or build a container image
type ImageSpec struct {
	// From existing image
	Image  string
	Tag    string
	Digest string

	// Or build from spec
	BaseImage      string
	PythonVersion  string
	PythonPackages []string
	Commands       []string
}

// GPUSpec describes GPU requirements
type GPUSpec struct {
	Type  string // "A100", "H100", "T4", etc.
	Count int
}

// PortSpec describes a port mapping
type PortSpec struct {
	Container int
	Protocol  string // "tcp", "udp", "http"
	Public    bool   // Expose publicly
}

// VolumeMount describes a volume mount
type VolumeMount struct {
	Name      string
	MountPath string
	ReadOnly  bool
}

// VolumeSpec describes a volume to be created
type VolumeSpec struct {
	Name      string
	SizeGB    int
	Type      string // "ssd", "hdd", "ephemeral"
	MountPath string
}

// Volume represents a created volume
type Volume struct {
	ID          string
	WorkspaceID string
	Name        string
	Type        string
	SizeGB      int
	Provider    string
	ProviderRef string // Provider-specific volume ID
	CreatedAt   time.Time
}

// GPUType represents available GPU types
type GPUType struct {
	Name         string
	Memory       int64
	Available    bool
	PricePerHour float64
}

// Checkpoint represents a container checkpoint
type Checkpoint struct {
	ID           string
	DeploymentID string
	CreatedAt    time.Time
	SizeBytes    int64
	Provider     string
	RemoteKey    string
}
