package k8s

import (
	"context"
	"fmt"

	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.client
}

func (c *Client) GetConfig() *rest.Config {
	return c.config
}

func (c *Client) GetNamespace() string {
	return c.namespace
}

func (c *Client) ListAllPods(ctx context.Context) (*core.PodList, error) {
	return c.client.CoreV1().Pods(c.namespace).List(ctx, meta.ListOptions{})
}

func (c *Client) DeletePod(ctx context.Context, name string) error {
	return c.client.CoreV1().Pods(c.namespace).Delete(ctx, name, meta.DeleteOptions{})
}
