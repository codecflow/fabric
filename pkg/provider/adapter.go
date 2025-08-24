package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/codecflow/fabric/pkg/types"
)

// PlacementAdapter adapts PlacementStrategy to work with new Deployment types
// This is a helper to convert between old and new type systems
type PlacementAdapter struct {
	strategy PlacementStrategy
}

// NewPlacementAdapter creates a new adapter
func NewPlacementAdapter(strategy PlacementStrategy) *PlacementAdapter {
	return &PlacementAdapter{strategy: strategy}
}

func (a *PlacementAdapter) Name() string {
	return a.strategy.Name()
}

func (a *PlacementAdapter) Deploy(ctx context.Context, workload *types.Workload) (*types.Deployment, error) {
	// Convert Workload to PlacementRequest
	req := workloadToPlacementRequest(workload)

	// Call old interface
	placement, err := a.strategy.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	// Convert Placement to Deployment
	return placementToDeployment(placement, workload.ID), nil
}

func (a *PlacementAdapter) Status(ctx context.Context, deploymentID string) (*types.Deployment, error) {
	placement, err := a.strategy.Get(ctx, deploymentID)
	if err != nil {
		return nil, err
	}
	return placementToDeployment(placement, ""), nil
}

func (a *PlacementAdapter) Stop(ctx context.Context, deploymentID string) error {
	return a.strategy.Stop(ctx, deploymentID)
}

func (a *PlacementAdapter) Delete(ctx context.Context, deploymentID string) error {
	return a.strategy.Delete(ctx, deploymentID)
}

func (a *PlacementAdapter) List(ctx context.Context, filter *DeploymentFilter) ([]*types.Deployment, error) {
	placements, err := a.strategy.List(ctx)
	if err != nil {
		return nil, err
	}

	deployments := make([]*types.Deployment, 0, len(placements))
	for _, p := range placements {
		deployments = append(deployments, placementToDeployment(p, ""))
	}
	return deployments, nil
}

func (a *PlacementAdapter) Capabilities() Capabilities {
	caps := a.strategy.Capabilities()
	return Capabilities{
		GPU:       caps.SupportsGPU,
		MaxCPU:    int64(caps.MaxCPU),
		MaxMemory: int64(caps.MaxMemoryGB * 1024), // Convert GB to MB
	}
}

// Helper functions for conversion

func workloadToPlacementRequest(w *types.Workload) *PlacementRequest {
	spec := &container.Spec{
		Command: w.Command,
		Env:     w.Env,
	}

	// Set image
	if w.Image != nil {
		if w.Image.Image != "" {
			spec.Image = w.Image.Image
			if w.Image.Tag != "" {
				spec.Image = fmt.Sprintf("%s:%s", w.Image.Image, w.Image.Tag)
			}
		}
	}

	// Set GPU
	if w.GPU != nil {
		spec.GPU = w.GPU.Count
	}

	// Set ports
	for _, port := range w.Ports {
		spec.Ports = append(spec.Ports, container.PortSpec{
			Container: port.Container,
			Protocol:  port.Protocol,
		})
	}

	return &PlacementRequest{
		ID:   w.ID,
		Spec: spec,
	}
}

func placementToDeployment(p *Placement, workloadID string) *types.Deployment {
	deployment := &types.Deployment{
		ID:         p.ID,
		WorkloadID: workloadID,
		Provider:   p.Provider,
		Status:     types.DeploymentStatus(p.Status),
		Metadata:   make(map[string]string),
		CreatedAt:  time.Now(), // Use current time for now
	}

	// Copy metadata
	for k, v := range p.Metadata {
		if str, ok := v.(string); ok {
			deployment.Metadata[k] = str
		}
	}

	// Convert ports to endpoints
	if p.Address != "" {
		for containerPort, hostPort := range p.Ports {
			endpoint := types.Endpoint{
				Type:    "tcp",
				Address: p.Address,
				Port:    hostPort,
				Public:  true,
			}

			// Try to construct HTTP URL
			if containerPort == 80 || containerPort == 8000 || containerPort == 8080 {
				endpoint.Type = "http"
				endpoint.URL = fmt.Sprintf("http://%s:%d", p.Address, hostPort)
			}

			deployment.Endpoints = append(deployment.Endpoints, endpoint)
		}
	}

	// Extract resource info from spec if available
	if p.Spec != nil {
		deployment.Resources = &types.AllocatedResources{
			GPU: p.Spec.GPU,
		}
	}

	return deployment
}
