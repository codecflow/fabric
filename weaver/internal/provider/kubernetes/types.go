package kubernetes

import (
	"weaver/internal/provider"
)

const Type provider.ProviderType = "kubernetes"

// Config represents Kubernetes-specific configuration
type Config struct {
	Kubeconfig string `json:"kubeconfig,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	InCluster  bool   `json:"inCluster,omitempty"`
}
