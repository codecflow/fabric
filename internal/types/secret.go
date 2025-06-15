package types

import (
	"time"
)

// SecretSpec defines the desired state of a secret
type SecretSpec struct {
	Type SecretType        `json:"type"`
	Data map[string][]byte `json:"data,omitempty"`

	// For external secret providers
	ExternalRef *ExternalSecretRef `json:"externalRef,omitempty"`
}

// SecretType defines the type of secret
type SecretType string

const (
	SecretTypeOpaque              SecretType = "Opaque"
	SecretTypeDockerConfigJSON    SecretType = "kubernetes.io/dockerconfigjson"
	SecretTypeBasicAuth           SecretType = "kubernetes.io/basic-auth"
	SecretTypeTLS                 SecretType = "kubernetes.io/tls"
	SecretTypeSSHAuth             SecretType = "kubernetes.io/ssh-auth"
	SecretTypeServiceAccountToken SecretType = "kubernetes.io/service-account-token"
)

// ExternalSecretRef references a secret in an external system
type ExternalSecretRef struct {
	Provider string            `json:"provider"` // "vault", "aws-secrets-manager", "azure-keyvault"
	Path     string            `json:"path"`
	Version  string            `json:"version,omitempty"`
	Auth     map[string]string `json:"auth,omitempty"`
}

// SecretStatus represents the current state of a secret
type SecretStatus struct {
	Phase   SecretPhase `json:"phase"`
	Message string      `json:"message,omitempty"`
	Reason  string      `json:"reason,omitempty"`

	// External secret sync status
	LastSync    *time.Time `json:"lastSync,omitempty"`
	SyncError   string     `json:"syncError,omitempty"`
	SyncVersion string     `json:"syncVersion,omitempty"`
}

// SecretPhase represents the lifecycle phase
type SecretPhase string

const (
	SecretPhasePending SecretPhase = "Pending"
	SecretPhaseActive  SecretPhase = "Active"
	SecretPhaseFailed  SecretPhase = "Failed"
	SecretPhaseUnknown SecretPhase = "Unknown"
)

// Secret represents a complete secret definition
type Secret struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	Spec   SecretSpec   `json:"spec"`
	Status SecretStatus `json:"status"`

	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

// SecretList represents a list of secrets
type SecretList struct {
	Items []Secret `json:"items"`
	Total int      `json:"total"`
}

// SecretReference is used to reference a secret in workload specs
type SecretReference struct {
	Name string `json:"name"`
	Key  string `json:"key,omitempty"` // If empty, all keys are used
}

// EnvFromSource represents a source for environment variables
type EnvFromSource struct {
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	Prefix    string           `json:"prefix,omitempty"`
}
