package provider

import (
	"context"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/codecflow/fabric/pkg/types"
)

// PlacementStrategy defines how to acquire compute resources (WHERE to run)
type PlacementStrategy interface {
	// Create provisions a new compute resource
	Create(ctx context.Context, req *PlacementRequest) (*Placement, error)

	// Get retrieves placement details
	Get(ctx context.Context, id string) (*Placement, error)

	// Stop stops a placement (may keep it for later resume)
	Stop(ctx context.Context, id string) error

	// Delete terminates and removes a placement completely
	Delete(ctx context.Context, id string) error

	// List returns all placements
	List(ctx context.Context) ([]*Placement, error)

	// Name returns the placement strategy name
	Name() string

	// Capabilities returns what this placement supports
	Capabilities() PlacementCapabilities
}

// ExecutionStrategy defines how to run workloads inside a placement (HOW to run)
type ExecutionStrategy interface {
	// Name returns the execution strategy name (e.g., "docker", "k3s", "vm", "desktop")
	Name() string

	// Prepare sets up the placement for this execution type
	// For example: install Docker, k3s, or desktop environment
	Prepare(ctx context.Context, placement *Placement) error

	// Run executes a workload
	Run(ctx context.Context, placement *Placement, workload *Workload) (*WorkloadResult, error)

	// Stop stops a running workload
	Stop(ctx context.Context, workloadID string) error

	// Logs retrieves workload logs
	Logs(ctx context.Context, workloadID string) ([]byte, error)

	// Requirements returns what this execution needs from placement
	Requirements() ExecutionRequirements
}

// Catalogue provides pricing and availability information
type Catalogue interface {
	// GetPricing returns cost information for a resource type
	GetPricing(ctx context.Context, resourceType string) (*Pricing, error)

	// ListAvailable returns available resources
	ListAvailable(ctx context.Context) ([]Resource, error)

	// EstimateCost estimates cost for a placement request
	EstimateCost(ctx context.Context, req *PlacementRequest) (float64, error)
}

// WorkloadProvider is a lightweight interface for new-style providers
// that directly deploy workloads without the placement/execution abstraction
type WorkloadProvider interface {
	// Name returns the provider name (e.g., "docker", "kubernetes")
	Name() string

	// Deploy deploys a workload
	Deploy(ctx context.Context, workload *types.Workload) (*types.Deployment, error)

	// Status retrieves deployment status
	Status(ctx context.Context, deploymentID string) (*types.Deployment, error)

	// Stop stops a running deployment
	Stop(ctx context.Context, deploymentID string) error

	// Delete removes a deployment
	Delete(ctx context.Context, deploymentID string) error

	// List lists deployments
	List(ctx context.Context, filter *DeploymentFilter) ([]*types.Deployment, error)

	// Capabilities returns provider capabilities
	Capabilities() Capabilities
}

// Provider bundles all provider-specific implementations (legacy + new)
type Provider interface {
	// Name returns the provider name (e.g., "runpod", "ec2", "kubernetes")
	Name() string

	// Placement returns the placement strategy
	Placement() PlacementStrategy

	// Executions returns supported execution strategies
	// Some providers support multiple (e.g., EC2: docker, k3s, vm)
	Executions() []ExecutionStrategy

	// Catalogue returns pricing/availability info
	Catalogue() Catalogue
	
	// WorkloadProvider methods (embedded for compatibility)
	WorkloadProvider
}

// PlacementRequest describes desired compute resources
type PlacementRequest struct {
	ID       string
	Spec     *container.Spec
	Provider string
	Region   string
	Spot     bool
}

// Placement represents provisioned compute resources
type Placement struct {
	ID          string
	ContainerID string
	Provider    string
	Spec        *container.Spec
	Status      string
	Ports       map[int]int
	Address     string
	Metadata    map[string]interface{}
}

// PlacementCapabilities describes what a placement supports
type PlacementCapabilities struct {
	SupportsGPU         bool
	SupportsSpot        bool
	SupportsMultiRegion bool
	MaxGPU              int
	MaxCPU              int
	MaxMemoryGB         int
	SupportedExecutions []string // e.g., ["docker", "k3s", "vm"]
}

// ExecutionRequirements describes what an execution needs
type ExecutionRequirements struct {
	NeedsDocker    bool
	NeedsK8s       bool
	NeedsRawVM     bool
	MinMemoryMB    int
	MinCPU         int
	RequiresAgent  bool // Whether agent bootstrap is needed
}

// Workload represents user code/task to execute
type Workload struct {
	ID      string
	Image   string
	Command []string
	Env     map[string]string
	GPU     int
}

// WorkloadResult contains execution results
type WorkloadResult struct {
	ID       string
	Status   string
	ExitCode int
	Output   string
	Error    string
}

// Pricing contains cost information
type Pricing struct {
	Provider       string
	ResourceType   string
	PricePerHour   float64
	PricePerMinute float64
	Currency       string
	Region         string
	Spot           bool
}

// Resource describes an available compute resource
type Resource struct {
	Provider string
	Type     string
	GPU      string
	CPU      int
	MemoryGB int
	Available bool
	Pricing  *Pricing
}

// Capabilities describes what a provider supports (new architecture)
type Capabilities struct {
	GPU         bool
	GPUTypes    []string
	MaxCPU      int64
	MaxMemory   int64
	Regions     []string
	
	// Features
	Volumes     bool
	Networking  bool
	Checkpoint  bool
	Autoscaling bool
}

// DeploymentFilter is used to filter deployments when listing
type DeploymentFilter struct {
	WorkspaceID string
	WorkloadID  string
	Status      string
}
