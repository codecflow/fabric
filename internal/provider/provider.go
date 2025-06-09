package provider

import (
	"time"
)

type Provider interface {
	Name() string
	ListRegions() ([]Region, error)
	ListMachineTypes() ([]MachineType, error)
	CreateInstance(spec InstanceSpec) (*Instance, error)
	DeleteInstance(id string) error
	GetInstance(id string) (*Instance, error)
	GetCostPerHour(machineType string, region string) (float64, error)
}

type Region struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MachineType struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	CPU      int               `json:"cpu"`
	Memory   int               `json:"memory"` // GB
	GPU      *GPU              `json:"gpu,omitempty"`
	Storage  int               `json:"storage"` // GB
	Metadata map[string]string `json:"metadata,omitempty"`
}

type GPU struct {
	Type   string `json:"type"`
	Count  int    `json:"count"`
	Memory int    `json:"memory"` // GB
}

type InstanceSpec struct {
	Provider    string            `json:"provider"`
	Region      string            `json:"region"`
	MachineType string            `json:"machine_type"`
	Image       string            `json:"image"`
	Name        string            `json:"name"`
	Env         map[string]string `json:"env,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type Instance struct {
	ID          string            `json:"id"`
	Provider    string            `json:"provider"`
	ProviderID  string            `json:"provider_id"`
	Region      string            `json:"region"`
	MachineType string            `json:"machine_type"`
	Status      string            `json:"status"`
	IP          string            `json:"ip,omitempty"`
	TailnetIP   string            `json:"tailnet_ip,omitempty"`
	CostPerHour float64           `json:"cost_per_hour"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Status constants
const (
	StatusPending  = "pending"
	StatusRunning  = "running"
	StatusStopped  = "stopped"
	StatusFailed   = "failed"
	StatusDeleting = "deleting"
	StatusDeleted  = "deleted"
)
