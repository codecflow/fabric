package container

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
)

// DockerRunner implements Runner using the Docker API
type DockerRunner struct {
	client *client.Client
}

// NewDockerRunner creates a new Docker-based container runner
func NewDockerRunner() (*DockerRunner, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &DockerRunner{
		client: cli,
	}, nil
}

// Run executes a container using Docker
func (r *DockerRunner) Run(ctx context.Context, spec *Spec) (*Result, error) {
	// Pull the image
	reader, err := r.client.ImagePull(ctx, spec.Image, types.ImagePullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", spec.Image, err)
	}
	// Consume the pull output to ensure it completes
	io.Copy(io.Discard, reader)
	reader.Close()

	// Prepare environment variables
	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create container config
	config := &container.Config{
		Image:        spec.Image,
		Cmd:          spec.Command,
		Env:          env,
		WorkingDir:   spec.WorkDir,
		Tty:          false,
		AttachStdout: true,
		AttachStderr: true,
	}

	// Handle port exposure for persistent containers
	if spec.Persistent && len(spec.Ports) > 0 {
		exposedPorts := nat.PortSet{}
		for _, p := range spec.Ports {
			protocol := p.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			port := nat.Port(fmt.Sprintf("%d/%s", p.Container, protocol))
			exposedPorts[port] = struct{}{}
		}
		config.ExposedPorts = exposedPorts
	}

	hostConfig := &container.HostConfig{
		AutoRemove: !spec.Persistent, // Only auto-remove if not persistent
	}

	// Port bindings for persistent containers - let Docker allocate host ports
	if spec.Persistent && len(spec.Ports) > 0 {
		portBindings := nat.PortMap{}
		for _, p := range spec.Ports {
			protocol := p.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			port := nat.Port(fmt.Sprintf("%d/%s", p.Container, protocol))
			portBindings[port] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Let Docker allocate a random port
				},
			}
		}
		hostConfig.PortBindings = portBindings
	}

	// Add GPU resources if requested
	if spec.GPU > 0 {
		hostConfig.Resources = container.Resources{
			DeviceRequests: []container.DeviceRequest{
				{
					Count:        spec.GPU,
					Capabilities: [][]string{{"gpu"}},
				},
			},
		}
	}

	// Create the container
	resp, err := r.client.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID

	// Attach to container to capture output
	attachResp, err := r.client.ContainerAttach(ctx, containerID, types.ContainerAttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach to container: %w", err)
	}
	defer attachResp.Close()

	// Start the container
	if err := r.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Read output from the container
	var stdoutBuf, stderrBuf bytes.Buffer
	outputDone := make(chan error)
	go func() {
		_, err := stdcopy.StdCopy(&stdoutBuf, &stderrBuf, attachResp.Reader)
		outputDone <- err
	}()

	// Wait for the container to finish
	statusCh, errCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("error waiting for container: %w", err)
		}
	case status := <-statusCh:
		// Wait for output to be fully read
		<-outputDone

		return &Result{
			ExitCode: int(status.StatusCode),
			Stdout:   stdoutBuf.String(),
			Stderr:   stderrBuf.String(),
		}, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("context cancelled: %w", ctx.Err())
	}

	return nil, fmt.Errorf("unexpected error in container execution")
}

// Start creates and starts a persistent container without waiting for completion
// Returns the container ID and port mappings (container port -> host port)
func (r *DockerRunner) Start(ctx context.Context, spec *Spec) (string, map[int]int, error) {
	// Pull the image
	reader, err := r.client.ImagePull(ctx, spec.Image, types.ImagePullOptions{})
	if err != nil {
		return "", nil, fmt.Errorf("failed to pull image %s: %w", spec.Image, err)
	}
	io.Copy(io.Discard, reader)
	reader.Close()

	// Prepare environment variables
	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create container config
	config := &container.Config{
		Image:      spec.Image,
		Cmd:        spec.Command,
		Env:        env,
		WorkingDir: spec.WorkDir,
		Tty:        false,
	}

	// Handle port exposure
	if len(spec.Ports) > 0 {
		exposedPorts := nat.PortSet{}
		for _, p := range spec.Ports {
			protocol := p.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			port := nat.Port(fmt.Sprintf("%d/%s", p.Container, protocol))
			exposedPorts[port] = struct{}{}
		}
		config.ExposedPorts = exposedPorts
	}

	hostConfig := &container.HostConfig{
		AutoRemove: false, // Persistent containers are not auto-removed
	}

	// Port bindings - let Docker allocate host ports
	if len(spec.Ports) > 0 {
		portBindings := nat.PortMap{}
		for _, p := range spec.Ports {
			protocol := p.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			port := nat.Port(fmt.Sprintf("%d/%s", p.Container, protocol))
			portBindings[port] = []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Let Docker allocate
				},
			}
		}
		hostConfig.PortBindings = portBindings
	}

	// Add GPU resources if requested
	if spec.GPU > 0 {
		hostConfig.Resources = container.Resources{
			DeviceRequests: []container.DeviceRequest{
				{
					Count:        spec.GPU,
					Capabilities: [][]string{{"gpu"}},
				},
			},
		}
	}

	// Create the container
	resp, err := r.client.ContainerCreate(ctx, config, hostConfig, nil, nil, "")
	if err != nil {
		return "", nil, fmt.Errorf("failed to create container: %w", err)
	}
	containerID := resp.ID

	// Start the container
	if err := r.client.ContainerStart(ctx, containerID, types.ContainerStartOptions{}); err != nil {
		return "", nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Get port mappings
	portMap := make(map[int]int)
	if len(spec.Ports) > 0 {
		inspect, err := r.client.ContainerInspect(ctx, containerID)
		if err != nil {
			return "", nil, fmt.Errorf("failed to inspect container: %w", err)
		}

		for _, p := range spec.Ports {
			protocol := p.Protocol
			if protocol == "" {
				protocol = "tcp"
			}
			port := nat.Port(fmt.Sprintf("%d/%s", p.Container, protocol))
			if bindings, ok := inspect.NetworkSettings.Ports[port]; ok && len(bindings) > 0 {
				hostPort := 0
				fmt.Sscanf(bindings[0].HostPort, "%d", &hostPort)
				portMap[p.Container] = hostPort
			}
		}
	}

	return containerID, portMap, nil
}

// Stop stops a running container
func (r *DockerRunner) Stop(ctx context.Context, containerID string) error {
	timeout := 10 // seconds
	return r.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &timeout})
}

// Remove removes a container
func (r *DockerRunner) Remove(ctx context.Context, containerID string) error {
	return r.client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	})
}

// Close cleans up the Docker client
func (r *DockerRunner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
