package kubernetes

import "github.com/codecflow/fabric/weaver/services/provider"

const Type provider.ProviderType = "kubernetes"

// Config represents Kubernetes-specific configuration
type Config struct {
	Kubeconfig string `json:"kubeconfig,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	InCluster  bool   `json:"inCluster,omitempty"`
}
