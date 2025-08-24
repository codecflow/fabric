package types

import "time"

// Deployment represents a deployed workload
type Deployment struct {
	ID         string
	WorkloadID string
	Provider   string
	Status     DeploymentStatus
	Endpoints  []Endpoint
	Resources  *AllocatedResources
	Metadata   map[string]string
	CreatedAt  time.Time
	StartedAt  *time.Time
	StoppedAt  *time.Time
}

// DeploymentStatus represents the current state of a deployment
type DeploymentStatus string

const (
	DeploymentStatusPending DeploymentStatus = "pending"
	DeploymentStatusRunning DeploymentStatus = "running"
	DeploymentStatusStopped DeploymentStatus = "stopped"
	DeploymentStatusFailed  DeploymentStatus = "failed"
)

// Endpoint describes how to access a deployed workload
type Endpoint struct {
	Type    string // "http", "tcp", "grpc", "websocket"
	Address string // IP address or hostname
	Port    int
	Public  bool   // Externally accessible
	URL     string // Full URL (for HTTP/WebSocket)
}

// AllocatedResources describes the actual resources allocated to a deployment
type AllocatedResources struct {
	CPU     int64  // millicores
	Memory  int64  // bytes
	GPU     int    // number of GPUs
	GPUType string // e.g., "A100", "H100"
}
