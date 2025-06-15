package kubernetes

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps the Kubernetes client with additional functionality
type Client struct {
	kubernetes.Interface
	config *rest.Config
}

// NewClient creates a new Kubernetes client
func NewClient(config Config) (*Client, error) {
	var restConfig *rest.Config
	var err error

	if config.InCluster || config.Kubeconfig == "" {
		// Use in-cluster config
		restConfig, err = rest.InClusterConfig()
	} else {
		// Use kubeconfig file
		restConfig, err = clientcmd.BuildConfigFromFlags("", config.Kubeconfig)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	client, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Client{
		Interface: client,
		config:    restConfig,
	}, nil
}

// Config returns the underlying REST config
func (c *Client) Config() *rest.Config {
	return c.config
}
