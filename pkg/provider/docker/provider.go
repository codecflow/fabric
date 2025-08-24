package docker

import (
	"context"
	"fmt"
	"strings"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/codecflow/fabric/pkg/provider"
	"github.com/codecflow/fabric/pkg/types"
)

// Provider implements the Provider interface for Docker
type Provider struct {
	runner *container.DockerRunner
}

// NewProvider creates a new Docker provider
func NewProvider() (*Provider, error) {
	runner, err := container.NewDockerRunner()
	if err != nil {
		return nil, fmt.Errorf("failed to create docker runner: %w", err)
	}

	return &Provider{
		runner: runner,
	}, nil
}

// Name returns the provider name
func (p *Provider) Name() string {
	return "docker"
}

// Deploy deploys a workload to Docker
func (p *Provider) Deploy(ctx context.Context, workload *types.Workload) (*types.Deployment, error) {
	// Build container spec from workload
	spec := &container.Spec{
		Image:      p.getImageName(workload),
		Command:    workload.Command,
		Env:        workload.Env,
		WorkDir:    workload.WorkingDir,
		GPU:        p.getGPUCount(workload),
		Persistent: true,
	}

	// Add port mappings (normalize protocols for Docker)
	for _, port := range workload.Ports {
		protocol := p.normalizeProtocol(port.Protocol)
		spec.Ports = append(spec.Ports, container.PortSpec{
			Container: port.Container,
			Protocol:  protocol,
		})
	}

	// Start container (persistent)
	containerID, portMap, err := p.runner.Start(ctx, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	// Build deployment
	deployment := &types.Deployment{
		ID:         containerID,
		WorkloadID: workload.ID,
		Provider:   "docker",
		Status:     types.DeploymentStatusRunning,
		Metadata:   make(map[string]string),
	}

	// Build endpoints from port mappings
	deployment.Endpoints = p.buildEndpointsFromPortMap(portMap, workload.Ports)
	
	// Set resource allocation
	deployment.Resources = &types.AllocatedResources{
		GPU: spec.GPU,
	}

	return deployment, nil
}

// Status retrieves the status of a deployment
func (p *Provider) Status(ctx context.Context, deploymentID string) (*types.Deployment, error) {
	// For now, return a simple status
	// TODO: Implement proper Docker inspect
	deployment := &types.Deployment{
		ID:       deploymentID,
		Provider: "docker",
		Status:   types.DeploymentStatusRunning,
		Metadata: make(map[string]string),
	}

	return deployment, nil
}

// Stop stops a running deployment
func (p *Provider) Stop(ctx context.Context, deploymentID string) error {
	return p.runner.Stop(ctx, deploymentID)
}

// Delete removes a deployment
func (p *Provider) Delete(ctx context.Context, deploymentID string) error {
	return p.runner.Remove(ctx, deploymentID)
}

// List lists all deployments
func (p *Provider) List(ctx context.Context, filter *provider.DeploymentFilter) ([]*types.Deployment, error) {
	// TODO: Implement proper container listing
	// For now, return empty list
	return []*types.Deployment{}, nil
}

// Capabilities returns provider capabilities
func (p *Provider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		GPU:         true,  // Docker supports NVIDIA GPUs
		GPUTypes:    []string{"nvidia"},
		MaxCPU:      0,  // No hard limit
		MaxMemory:   0,  // No hard limit
		Regions:     []string{"local"},
		Volumes:     true,
		Networking:  true,
		Checkpoint:  false,  // Not yet implemented
		Autoscaling: false,
	}
}

// Helper methods

func (p *Provider) getImageName(workload *types.Workload) string {
	if workload.Image == nil {
		return "alpine:latest"
	}

	if workload.Image.Image != "" {
		image := workload.Image.Image
		if workload.Image.Tag != "" {
			image = fmt.Sprintf("%s:%s", image, workload.Image.Tag)
		}
		return image
	}

	// If no image specified but base image is, use that
	if workload.Image.BaseImage != "" {
		return workload.Image.BaseImage
	}

	return "alpine:latest"
}

func (p *Provider) getGPUCount(workload *types.Workload) int {
	if workload.GPU == nil {
		return 0
	}
	return workload.GPU.Count
}

func (p *Provider) normalizeProtocol(protocol string) string {
	// Docker only understands "tcp" and "udp"
	// Map application-level protocols to transport protocols
	protocol = strings.ToLower(protocol)
	switch protocol {
	case "http", "https", "grpc", "ws", "wss":
		return "tcp"
	case "udp":
		return "udp"
	case "tcp", "":
		return "tcp"
	default:
		// Default to tcp for unknown protocols
		return "tcp"
	}
}

func (p *Provider) buildEndpointsFromPortMap(portMap map[int]int, ports []types.PortSpec) []types.Endpoint {
	endpoints := make([]types.Endpoint, 0)

	for _, port := range ports {
		hostPort, ok := portMap[port.Container]
		if !ok || hostPort == 0 {
			continue
		}

		endpoint := types.Endpoint{
			Type:    "tcp",
			Address: "localhost",
			Port:    hostPort,
			Public:  port.Public,
		}

		// Try to construct HTTP URL for common HTTP ports
		if port.Container == 80 || port.Container == 8000 || port.Container == 8080 ||
			strings.ToLower(port.Protocol) == "http" {
			endpoint.Type = "http"
			endpoint.URL = fmt.Sprintf("http://localhost:%d", hostPort)
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints
}
