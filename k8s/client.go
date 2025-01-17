package k8s

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Client struct {
	config *rest.Config
	client *kubernetes.Clientset

	image      string
	prefix     string
	namespace  string
	entrypoint string
}

func NewClient(prefix, namespace, entrypoint, image string) (*Client, error) {

	config, err := GetConfig()

	if err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}

	client, err := kubernetes.NewForConfig(config)

	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{
		config:     config,
		namespace:  namespace,
		image:      image,
		prefix:     prefix,
		entrypoint: entrypoint,
		client:     client,
	}, nil
}
