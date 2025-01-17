package k8s

import (
	"context"
	"fmt"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Streamer struct {
	ID   string
	URL  string
	RTMP string
}

func (c *Client) Create(ctx context.Context, s Streamer) (*core.Pod, error) {
	name := c.prefix + s.ID
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":    "streamer",
				"stream": s.ID,
			},
		},
		Spec: core.PodSpec{
			Hostname:  name,
			Subdomain: c.entrypoint,
			NodeSelector: map[string]string{
				"cloud.google.com/compute-class": "Balanced",
			},
			Containers: []core.Container{
				{
					Name:            "streamer",
					Image:           c.image,
					ImagePullPolicy: core.PullIfNotPresent,
					Resources: core.ResourceRequirements{
						Limits: core.ResourceList{
							"cpu":               resource.MustParse("3"),
							"memory":            resource.MustParse("4Gi"),
							"ephemeral-storage": resource.MustParse("1Gi"),
						},
						Requests: core.ResourceList{
							"cpu":               resource.MustParse("2"),
							"memory":            resource.MustParse("2Gi"),
							"ephemeral-storage": resource.MustParse("1Gi"),
						},
					},
					Env: []core.EnvVar{
						{Name: "URL", Value: s.URL},
						{Name: "RTMP", Value: s.RTMP},
					},
				},
			},
			ImagePullSecrets: []core.LocalObjectReference{{Name: "ghcr"}},
		},
	}

	return c.client.CoreV1().Pods(c.namespace).Create(ctx, pod, meta.CreateOptions{})
}

func (c *Client) Find(ctx context.Context, id string) (*core.Pod, error) {
	pods, err := c.client.CoreV1().Pods(c.namespace).List(ctx, meta.ListOptions{
		LabelSelector: "stream=" + id,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod found for stream %s", id)
	}

	if len(pods.Items) > 1 {
		return &pods.Items[0], fmt.Errorf("multiple pods found for stream %s", id)
	}

	return &pods.Items[0], nil
}

func (c *Client) Delete(ctx context.Context, id string) error {
	pods, err := c.Find(ctx, id)

	if err != nil {
		return err
	}

	err = c.client.CoreV1().Pods(c.namespace).Delete(ctx, pods.Name, meta.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	return nil
}
