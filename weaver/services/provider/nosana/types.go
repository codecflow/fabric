package nosana

// Job represents a Nosana job
type Job struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Image      string            `json:"image"`
	Command    []string          `json:"command,omitempty"`
	Args       []string          `json:"args,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	Resources  Resources         `json:"resources"`
	Status     string            `json:"status"`
	NodeID     string            `json:"nodeId,omitempty"`
	CreatedAt  string            `json:"createdAt"`
	StartedAt  string            `json:"startedAt,omitempty"`
	FinishedAt string            `json:"finishedAt,omitempty"`
	ExitCode   int               `json:"exitCode,omitempty"`
	Logs       string            `json:"logs,omitempty"`
	Price      Price             `json:"price"`
	Network    string            `json:"network"`
	Market     string            `json:"market"`
}

// Resources represents job resource requirements
type Resources struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	GPU    string `json:"gpu,omitempty"`
	Disk   string `json:"disk"`
}

// Price represents pricing information
type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Unit     string  `json:"unit"`
}

// Node represents a Nosana compute node
type Node struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Status       string            `json:"status"`
	Location     string            `json:"location"`
	Capabilities NodeCapabilities  `json:"capabilities"`
	Pricing      NodePricing       `json:"pricing"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// NodeCapabilities represents node capabilities
type NodeCapabilities struct {
	CPU    CPUInfo `json:"cpu"`
	Memory string  `json:"memory"`
	GPU    GPUInfo `json:"gpu,omitempty"`
	Disk   string  `json:"disk"`
}

// CPUInfo represents CPU information
type CPUInfo struct {
	Cores        int    `json:"cores"`
	Model        string `json:"model"`
	Architecture string `json:"architecture"`
}

// GPUInfo represents GPU information
type GPUInfo struct {
	Count  int    `json:"count"`
	Model  string `json:"model"`
	Memory string `json:"memory"`
}

// NodePricing represents node pricing
type NodePricing struct {
	CPU    float64 `json:"cpu"`    // per core hour
	Memory float64 `json:"memory"` // per GB hour
	GPU    float64 `json:"gpu"`    // per GPU hour
	Disk   float64 `json:"disk"`   // per GB hour
}

// Market represents a Nosana market
type Market struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Type        string  `json:"type"`
	Network     string  `json:"network"`
	Active      bool    `json:"active"`
	MinPrice    float64 `json:"minPrice"`
	MaxPrice    float64 `json:"maxPrice"`
}

// JobRequest represents a job creation request
type JobRequest struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Command   []string          `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	Resources Resources         `json:"resources"`
	Price     Price             `json:"price"`
	Market    string            `json:"market"`
	Network   string            `json:"network"`
}

// JobStatus represents possible job statuses
const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
	JobStatusCanceled  = "canceled"
)

// NodeStatus represents possible node statuses
const (
	NodeStatusOnline  = "online"
	NodeStatusOffline = "offline"
	NodeStatusBusy    = "busy"
)
