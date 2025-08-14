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

	hostConfig := &container.HostConfig{
		AutoRemove: true, // Automatically remove container when it exits
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

// Close cleans up the Docker client
func (r *DockerRunner) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}
