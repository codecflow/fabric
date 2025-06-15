package containerd

import (
	"context"
	"fmt"
	"log"
	"time"

	"shuttle/internal/config"
	"shuttle/internal/grpc"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/cio"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/oci"
)

// Runtime manages container lifecycle using containerd
type Runtime struct {
	config *config.RuntimeConfig
	client *containerd.Client
}

// New creates a new containerd runtime
func New(cfg *config.RuntimeConfig) (*Runtime, error) {
	return &Runtime{
		config: cfg,
	}, nil
}

// Start initializes the containerd client
func (r *Runtime) Start(ctx context.Context) error {
	log.Printf("Connecting to containerd at %s", r.config.Socket)

	client, err := containerd.New(r.config.Socket)
	if err != nil {
		return fmt.Errorf("failed to connect to containerd: %w", err)
	}

	r.client = client

	// Test connection
	version, err := client.Version(ctx)
	if err != nil {
		return fmt.Errorf("failed to get containerd version: %w", err)
	}

	log.Printf("Connected to containerd version %s", version.Version)
	return nil
}

// Stop closes the containerd client
func (r *Runtime) Stop() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// StartContainer starts a new container from a workload spec
func (r *Runtime) StartContainer(ctx context.Context, spec *grpc.WorkloadSpec) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("containerd client not initialized")
	}

	// Use the configured namespace
	ctx = namespaces.WithNamespace(ctx, r.config.Namespace)

	log.Printf("Starting container for workload %s", spec.ID)

	// Pull image if needed
	image, err := r.pullImage(ctx, spec.Image)
	if err != nil {
		return "", fmt.Errorf("failed to pull image: %w", err)
	}

	// Create container
	container, err := r.createContainer(ctx, spec, image)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	// Start container
	task, err := r.startTask(ctx, container, spec)
	if err != nil {
		return "", fmt.Errorf("failed to start task: %w", err)
	}

	log.Printf("Container %s started for workload %s", container.ID(), spec.ID)

	// Wait for task to be running
	if err := r.waitForRunning(ctx, task); err != nil {
		return "", fmt.Errorf("container failed to start: %w", err)
	}

	return container.ID(), nil
}

// StopContainer stops and removes a container
func (r *Runtime) StopContainer(ctx context.Context, containerID string) error {
	if r.client == nil {
		return fmt.Errorf("containerd client not initialized")
	}

	ctx = namespaces.WithNamespace(ctx, r.config.Namespace)

	log.Printf("Stopping container %s", containerID)

	// Get container
	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return fmt.Errorf("failed to load container: %w", err)
	}

	// Get task
	task, err := container.Task(ctx, nil)
	if err != nil {
		// Task might not exist, try to delete container anyway
		log.Printf("No task found for container %s: %v", containerID, err)
	} else {
		// Kill task with SIGTERM
		if err := task.Kill(ctx, 15); err != nil { // SIGTERM = 15
			log.Printf("Failed to kill task for container %s: %v", containerID, err)
		}

		// Wait for task to exit
		exitCh, err := task.Wait(ctx)
		if err == nil {
			select {
			case <-exitCh:
			case <-time.After(30 * time.Second):
				// Force kill if graceful shutdown takes too long
				task.Kill(ctx, 9) // SIGKILL = 9
				<-exitCh
			}
		}

		// Delete task
		if _, err := task.Delete(ctx); err != nil {
			log.Printf("Failed to delete task for container %s: %v", containerID, err)
		}
	}

	// Delete container
	if err := container.Delete(ctx, containerd.WithSnapshotCleanup); err != nil {
		return fmt.Errorf("failed to delete container: %w", err)
	}

	log.Printf("Container %s stopped and removed", containerID)
	return nil
}

// pullImage pulls the container image
func (r *Runtime) pullImage(ctx context.Context, imageRef string) (containerd.Image, error) {
	log.Printf("Pulling image %s", imageRef)

	image, err := r.client.Pull(ctx, imageRef, containerd.WithPullUnpack)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image %s: %w", imageRef, err)
	}

	log.Printf("Image %s pulled successfully", imageRef)
	return image, nil
}

// createContainer creates a new container
func (r *Runtime) createContainer(ctx context.Context, spec *grpc.WorkloadSpec, image containerd.Image) (containerd.Container, error) {
	containerID := fmt.Sprintf("%s-%s", spec.Namespace, spec.Name)

	// Build OCI spec
	opts := []oci.SpecOpts{
		oci.WithImageConfig(image),
		oci.WithProcessArgs(spec.Command...),
	}

	// Add environment variables
	if len(spec.Env) > 0 {
		opts = append(opts, oci.WithEnv(spec.Env))
	}

	// Add resource limits
	if spec.Resources != nil {
		if spec.Resources.CPULimit != "" {
			// In a real implementation, parse and set CPU limits
		}
		if spec.Resources.MemoryLimit != "" {
			// In a real implementation, parse and set memory limits
		}
	}

	container, err := r.client.NewContainer(
		ctx,
		containerID,
		containerd.WithImage(image),
		containerd.WithNewSnapshot(containerID+"-snapshot", image),
		containerd.WithNewSpec(opts...),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	return container, nil
}

// startTask starts the container task
func (r *Runtime) startTask(ctx context.Context, container containerd.Container, spec *grpc.WorkloadSpec) (containerd.Task, error) {
	// Create task with stdio
	task, err := container.NewTask(ctx, cio.NewCreator(cio.WithStdio))
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Start task
	if err := task.Start(ctx); err != nil {
		task.Delete(ctx)
		return nil, fmt.Errorf("failed to start task: %w", err)
	}

	return task, nil
}

// waitForRunning waits for the task to be in running state
func (r *Runtime) waitForRunning(ctx context.Context, task containerd.Task) error {
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for container to start")
		case <-ticker.C:
			status, err := task.Status(ctx)
			if err != nil {
				return fmt.Errorf("failed to get task status: %w", err)
			}

			switch status.Status {
			case containerd.Running:
				return nil
			case containerd.Stopped:
				return fmt.Errorf("container stopped unexpectedly")
			}
		}
	}
}

// ListContainers returns all containers in the namespace
func (r *Runtime) ListContainers(ctx context.Context) ([]containerd.Container, error) {
	if r.client == nil {
		return nil, fmt.Errorf("containerd client not initialized")
	}

	ctx = namespaces.WithNamespace(ctx, r.config.Namespace)
	return r.client.Containers(ctx)
}

// GetContainerStatus returns the status of a container
func (r *Runtime) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	if r.client == nil {
		return nil, fmt.Errorf("containerd client not initialized")
	}

	ctx = namespaces.WithNamespace(ctx, r.config.Namespace)

	container, err := r.client.LoadContainer(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("failed to load container: %w", err)
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		return &ContainerStatus{
			ID:     containerID,
			Status: "stopped",
		}, nil
	}

	status, err := task.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get task status: %w", err)
	}

	return &ContainerStatus{
		ID:     containerID,
		Status: string(status.Status),
	}, nil
}

// ContainerStatus represents container status
type ContainerStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
