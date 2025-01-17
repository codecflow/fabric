package k8s

import (
	"context"
	"fmt"
	"net/http"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

func (c *Client) Create(ctx context.Context, namespace, id string) (*core.Pod, error) {
	name := "streamer-" + id
	pod := &core.Pod{
		ObjectMeta: meta.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": "streamer",
			},
		},
		Spec: core.PodSpec{
			Hostname:  name,
			Subdomain: "entrypoint",
			NodeSelector: map[string]string{
				"cloud.google.com/compute-class": "Balanced",
			},
			Containers: []core.Container{
				{
					Name:            "streamer",
					Image:           "ghcr.io/codecflow/conductor:1.0.0",
					ImagePullPolicy: core.PullIfNotPresent,
					// Args:            []string{"tail", "-f", "/dev/null"},
					Resources: core.ResourceRequirements{
						Limits: core.ResourceList{
							"cpu":               resource.MustParse("4"),
							"memory":            resource.MustParse("8Gi"),
							"ephemeral-storage": resource.MustParse("2Gi"),
						},
						Requests: core.ResourceList{
							"cpu":               resource.MustParse("2"),
							"memory":            resource.MustParse("4Gi"),
							"ephemeral-storage": resource.MustParse("1Gi"),
						},
					},
					Env: []core.EnvVar{
						{
							Name:  "URL",
							Value: "https://pokemind.ai/",
						},
						{
							Name:  "RTMP",
							Value: "rtmp://ingest.global-contribute.live-video.net/app/live_1210610726_abc1ufMSJQ9F47o4ojIkg64rztYcV5",
						},
						{
							Name:  "DISPLAY",
							Value: ":99",
						},
						{
							Name:  "OVERLAY",
							Value: ":98",
						},
						{
							Name:  "WIDTH",
							Value: "1920",
						},
						{
							Name:  "HEIGHT",
							Value: "1080",
						},
					},
				},
				{
					Name:  "socat",
					Image: "alpine/socat",
					Command: []string{
						"socat",
						"TCP-LISTEN:9222,fork",
						"TCP:127.0.0.1:9222",
					},
					Ports: []core.ContainerPort{
						{
							ContainerPort: 9222,
						},
					},
					Resources: core.ResourceRequirements{
						Requests: core.ResourceList{
							"cpu":    resource.MustParse("50m"),
							"memory": resource.MustParse("64Mi"),
						},
						Limits: core.ResourceList{
							"cpu":    resource.MustParse("200m"),
							"memory": resource.MustParse("256Mi"),
						},
					},
				},
			},
			ImagePullSecrets: []core.LocalObjectReference{{Name: "ghcr"}},
		},
	}

	return c.client.CoreV1().Pods(namespace).Create(ctx, pod, meta.CreateOptions{})
}

func (c *Client) Find(ctx context.Context, streamID string) (*core.Pod, error) {
	pods, err := c.client.CoreV1().Pods("default").List(ctx, meta.ListOptions{
		LabelSelector: fmt.Sprintf("stream=%s", streamID),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pod found for stream %s", streamID)
	}

	if len(pods.Items) > 1 {
		return &pods.Items[0], fmt.Errorf("multiple pods found for stream %s", streamID)
	}

	return &pods.Items[0], nil
}

func (c *Client) Forward(ctx context.Context, pod, namespace string, port int) error {
	req := c.client.CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(pod).
		SubResource("portforward")

	fmt.Printf("Using URL: %s\n", req.URL().String())

	transport, upgrader, err := spdy.RoundTripperFor(c.config)
	if err != nil {
		return fmt.Errorf("failed to create round tripper: %v", err)
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	stopChan := make(chan struct{}, 1)
	readyChan := make(chan struct{})
	errChan := make(chan error, 1)
	ports := []string{fmt.Sprintf("%d:%d", port, port)}

	forwarder, err := portforward.New(dialer, ports, stopChan, readyChan, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to create port forwarder: %v", err)
	}

	fmt.Printf("Starting port forward for %s/%s on port %d...\n", namespace, pod, port)

	go func() {
		if err := forwarder.ForwardPorts(); err != nil {
			errChan <- err
		}
	}()

	select {
	case <-readyChan:
		fmt.Printf("Port forwarding is ready! Access your pod at: http://localhost:%d\n", port)
	case err := <-errChan:
		return fmt.Errorf("port forwarding failed: %v", err)
	case <-ctx.Done():
		return nil
	}

	select {
	case err := <-errChan:
		fmt.Printf("Port forwarding ended: %v\n", err)
	case <-ctx.Done():
		fmt.Println("Shutting down port forward...")
	}

	close(stopChan)
	return nil
}
