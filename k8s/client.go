package k8s

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	config *rest.Config
	client *kubernetes.Clientset
}

func NewClient() (*Client, error) {
	config, err := FromLocal()

	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		config: config,
		client: client,
	}, nil
}
