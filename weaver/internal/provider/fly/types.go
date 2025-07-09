package fly

import (
	"github.com/codecflow/fabric/weaver/internal/provider"
)

// ProviderTypeFly represents the Fly.io provider type
const ProviderTypeFly provider.ProviderType = "fly"

// App represents a Fly.io application
type App struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	Status       string       `json:"status"`
	Deployed     bool         `json:"deployed"`
	Hostname     string       `json:"hostname"`
	AppURL       string       `json:"appUrl"`
	Version      int          `json:"version"`
	Release      Release      `json:"release"`
	Organization Organization `json:"organization"`
	Secrets      []Secret     `json:"secrets"`
	IPAddresses  []IPAddress  `json:"ipAddresses"`
	Services     []Service    `json:"services"`
	Machines     []Machine    `json:"machines"`
	Volumes      []Volume     `json:"volumes"`
	Config       AppConfig    `json:"config"`
	CreatedAt    string       `json:"createdAt"`
	UpdatedAt    string       `json:"updatedAt"`
}

// Machine represents a Fly.io machine
type Machine struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	State      string        `json:"state"`
	Region     string        `json:"region"`
	Config     MachineConfig `json:"config"`
	ImageRef   ImageRef      `json:"image_ref"`
	InstanceID string        `json:"instance_id"`
	PrivateIP  string        `json:"private_ip"`
	CreatedAt  string        `json:"created_at"`
	UpdatedAt  string        `json:"updated_at"`
	Events     []Event       `json:"events"`
}

// MachineConfig represents machine configuration
type MachineConfig struct {
	Image    string            `json:"image"`
	Env      map[string]string `json:"env,omitempty"`
	Cmd      []string          `json:"cmd,omitempty"`
	Guest    Guest             `json:"guest"`
	Services []Service         `json:"services,omitempty"`
	Mounts   []Mount           `json:"mounts,omitempty"`
	Restart  RestartPolicy     `json:"restart,omitempty"`
	DNS      DNSConfig         `json:"dns,omitempty"`
}

// Guest represents machine guest configuration (CPU/memory)
type Guest struct {
	CPUs             int    `json:"cpus"`
	CPUKind          string `json:"cpu_kind,omitempty"` // "shared", "performance"
	MemoryMB         int    `json:"memory_mb"`
	GPUs             int    `json:"gpus,omitempty"`
	GPUKind          string `json:"gpu_kind,omitempty"` // "a100-pcie-40gb", "a100-sxm4-80gb"
	HostDedicatedCPU bool   `json:"host_dedicated_cpu,omitempty"`
}

// Service represents a network service
type Service struct {
	Protocol     string `json:"protocol"` // "tcp", "udp"
	InternalPort int    `json:"internal_port"`
	Ports        []Port `json:"ports,omitempty"`
}

// Port represents a service port
type Port struct {
	Port     int      `json:"port"`
	Handlers []string `json:"handlers,omitempty"` // "http", "tls"
}

// Mount represents a volume mount
type Mount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Type        string `json:"type,omitempty"` // "volume"
}

// RestartPolicy represents restart configuration
type RestartPolicy struct {
	Policy string `json:"policy"` // "no", "always", "on-failure"
}

// DNSConfig represents DNS configuration
type DNSConfig struct {
	SkipRegistration bool `json:"skip_registration,omitempty"`
}

// ImageRef represents container image reference
type ImageRef struct {
	Registry   string            `json:"registry"`
	Repository string            `json:"repository"`
	Tag        string            `json:"tag"`
	Digest     string            `json:"digest"`
	Labels     map[string]string `json:"labels"`
}

// Event represents a machine event
type Event struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Status    string `json:"status"`
	Source    string `json:"source"`
	Timestamp string `json:"timestamp"`
	Request   string `json:"request,omitempty"`
}

// Release represents an app release
type Release struct {
	ID          string `json:"id"`
	Version     int    `json:"version"`
	Stable      bool   `json:"stable"`
	Description string `json:"description"`
	User        User   `json:"user"`
	CreatedAt   string `json:"createdAt"`
}

// Organization represents a Fly.io organization
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// User represents a Fly.io user
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Secret represents an app secret
type Secret struct {
	Name      string `json:"name"`
	Digest    string `json:"digest"`
	CreatedAt string `json:"createdAt"`
}

// IPAddress represents an IP address allocation
type IPAddress struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Type      string `json:"type"` // "v4", "v6"
	Region    string `json:"region"`
	CreatedAt string `json:"createdAt"`
}

// Volume represents a persistent volume
type Volume struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	SizeGB            int    `json:"sizeGb"`
	Region            string `json:"region"`
	Encrypted         bool   `json:"encrypted"`
	AttachedMachineID string `json:"attachedMachineId,omitempty"`
	CreatedAt         string `json:"createdAt"`
}

// AppConfig represents application configuration
type AppConfig struct {
	AppName       string            `json:"app"`
	PrimaryRegion string            `json:"primary_region"`
	Env           map[string]string `json:"env,omitempty"`
	Services      []Service         `json:"services,omitempty"`
	HTTPService   HTTPService       `json:"http_service,omitempty"`
}

// HTTPService represents HTTP service configuration
type HTTPService struct {
	InternalPort int  `json:"internal_port"`
	ForceHTTPS   bool `json:"force_https,omitempty"`
}

// Region represents a Fly.io region
type Region struct {
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	GatewayAvailable bool    `json:"gatewayAvailable"`
	RequiresPaidPlan bool    `json:"requiresPaidPlan"`
}

// MachineSize represents available machine sizes
type MachineSize struct {
	Name        string  `json:"name"`
	CPUs        int     `json:"cpus"`
	MemoryMB    int     `json:"memory_mb"`
	PriceMonth  float64 `json:"price_month"`
	PriceSecond float64 `json:"price_second"`
}

// CreateAppRequest represents an app creation request
type CreateAppRequest struct {
	AppName       string `json:"app_name"`
	OrgSlug       string `json:"org_slug"`
	PrimaryRegion string `json:"primary_region,omitempty"`
}

// CreateMachineRequest represents a machine creation request
type CreateMachineRequest struct {
	Name   string        `json:"name,omitempty"`
	Config MachineConfig `json:"config"`
	Region string        `json:"region,omitempty"`
}

// UpdateMachineRequest represents a machine update request
type UpdateMachineRequest struct {
	Config MachineConfig `json:"config"`
}

// Machine states
const (
	MachineStateCreated    = "created"
	MachineStateStarting   = "starting"
	MachineStateStarted    = "started"
	MachineStateStopping   = "stopping"
	MachineStateStopped    = "stopped"
	MachineStateReplacing  = "replacing"
	MachineStateDestroying = "destroying"
	MachineStateDestroyed  = "destroyed"
)

// App states
const (
	AppStatePending   = "pending"
	AppStateRunning   = "running"
	AppStateSuspended = "suspended"
)

// CPU kinds
const (
	CPUKindShared      = "shared"
	CPUKindPerformance = "performance"
)

// GPU kinds
const (
	GPUKindA100PCIe40GB = "a100-pcie-40gb"
	GPUKindA100SXM480GB = "a100-sxm4-80gb"
)
