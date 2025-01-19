package k8s

import (
	"context"
	"fmt"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Machine struct {
	ID    string
	Image string
	Tools []string
	Env   map[string]string
}

func (c *Client) Create(ctx context.Context, m Machine) (*core.Pod, error) {
	name := c.prefix + m.ID

	err := c.CreatePVC(ctx, name, "4Gi")

	if err != nil {
		return nil, fmt.Errorf("failed to create PVC: %w", err)
	}

	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":        "machine",
				"machine-id": m.ID,
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
					Name:            "machine",
					Image:           m.Image,
					ImagePullPolicy: core.PullIfNotPresent,
					Resources: core.ResourceRequirements{
						Limits: core.ResourceList{
							"cpu":               resource.MustParse("3"),
							"memory":            resource.MustParse("4Gi"),
							"ephemeral-storage": resource.MustParse("3Gi"),
						},
						Requests: core.ResourceList{
							"cpu":               resource.MustParse("2"),
							"memory":            resource.MustParse("2Gi"),
							"ephemeral-storage": resource.MustParse("3Gi"),
						},
					},
					Env: createEnvVars(m.Env),
					VolumeMounts: []core.VolumeMount{
						{
							Name:      "data",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []core.Volume{
				{
					Name: "data",
					VolumeSource: core.VolumeSource{
						PersistentVolumeClaim: &core.PersistentVolumeClaimVolumeSource{
							ClaimName: name,
						},
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
		LabelSelector: "machine-id=" + id,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no machine found with id %s", id)
	}

	if len(pods.Items) > 1 {
		return &pods.Items[0], fmt.Errorf("multiple machines found with id %s", id)
	}

	return &pods.Items[0], nil
}

func createEnvVars(envMap map[string]string) []core.EnvVar {
	if envMap == nil {
		return []core.EnvVar{}
	}

	envVars := make([]core.EnvVar, 0, len(envMap))
	for key, value := range envMap {
		envVars = append(envVars, core.EnvVar{
			Name:  key,
			Value: value,
		})
	}
	return envVars
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
