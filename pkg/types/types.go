package types

import "time"

// Worker represents a worker node in the system
type Worker struct {
	ID           string       `json:"id"`
	Address      string       `json:"address"`
	Capabilities Capabilities `json:"capabilities"`
	RegisteredAt time.Time    `json:"registered_at"`
	LastHeartbeat time.Time   `json:"last_heartbeat"`
}

// Capabilities represents the resource capabilities of a worker
type Capabilities struct {
	CPU      int   `json:"cpu"`       // CPU cores
	Memory   int64 `json:"memory"`    // Memory in bytes
	GPUCount int   `json:"gpu_count"` // Number of GPUs available
}

// RegisterWorkerRequest is the request body for worker registration
type RegisterWorkerRequest struct {
	ID           string       `json:"id"`
	Address      string       `json:"address"`
	Capabilities Capabilities `json:"capabilities"`
}

// RegisterWorkerResponse is the response for worker registration
type RegisterWorkerResponse struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

// CreateContainerRequest represents a request to create a container
type CreateContainerRequest struct {
	Image   string            `json:"image"`
	Command []string          `json:"command"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"workdir,omitempty"`
	GPU     int               `json:"gpu,omitempty"` // Number of GPUs required
}

// CreateContainerResponse represents the response for creating a container
type CreateContainerResponse struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	WorkerID string `json:"worker_id,omitempty"`
}
