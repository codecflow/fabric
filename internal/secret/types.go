package secret

import (
	"time"
)

// Spec defines the desired state of a secret
type Spec struct {
	Type SecretType        `json:"type"`
	Data map[string][]byte `json:"data,omitempty"`

	// For external secret providers
	ExternalRef *ExternalSecretRef `json:"externalRef,omitempty"`
}

// SecretType defines the type of secret
type SecretType string

const (
	TypeOpaque              SecretType = "Opaque"
	TypeDockerConfigJSON    SecretType = "kubernetes.io/dockerconfigjson"
	TypeBasicAuth           SecretType = "kubernetes.io/basic-auth"
	TypeTLS                 SecretType = "kubernetes.io/tls"
	TypeSSHAuth             SecretType = "kubernetes.io/ssh-auth"
	TypeServiceAccountToken SecretType = "kubernetes.io/service-account-token"
)

// ExternalSecretRef references a secret in an external system
type ExternalSecretRef struct {
	Provider string            `json:"provider"` // "vault", "aws-secrets-manager", "azure-keyvault"
	Path     string            `json:"path"`
	Version  string            `json:"version,omitempty"`
	Auth     map[string]string `json:"auth,omitempty"`
}

// Status represents the current state of a secret
type Status struct {
	Phase   Phase  `json:"phase"`
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`

	// External secret sync status
	LastSync    *time.Time `json:"lastSync,omitempty"`
	SyncError   string     `json:"syncError,omitempty"`
	SyncVersion string     `json:"syncVersion,omitempty"`
}

// Phase represents the lifecycle phase
type Phase string

const (
	PhasePending Phase = "Pending"
	PhaseActive  Phase = "Active"
	PhaseFailed  Phase = "Failed"
	PhaseUnknown Phase = "Unknown"
)

// Secret represents a complete secret definition
type Secret struct {
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

// List represents a list of secrets
type List struct {
	Items []Secret `json:"items"`
	Total int      `json:"total"`
}

// Reference is used to reference a secret in workload specs
type Reference struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"` // If empty, all keys are used
}

// EnvFromSource represents a source for environment variables
type EnvFromSource struct {
	SecretRef *Reference `json:"secretRef,omitempty"`
	Prefix    string     `json:"prefix,omitempty"`
}

// Filter defines filters for secret queries
type Filter struct {
	Namespace string
	Labels    map[string]string
	Type      SecretType
}
