package shuttle

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"shuttle/internal/config"
	"shuttle/internal/containerd"
	"shuttle/internal/grpc"
	"shuttle/internal/metrics"
	"shuttle/internal/tailscale"
)

// Shuttle represents the node runner
type Shuttle struct {
	config *config.Config

	// Core components
	tailscale  *tailscale.Client
	runtime    *containerd.Runtime
	grpcClient *grpc.Client
	metrics    *metrics.Server

	// State management
	mu        sync.RWMutex
	workloads map[string]*WorkloadInstance
	stopping  bool
}

// WorkloadInstance represents a running workload
type WorkloadInstance struct {
	ID          string
	Name        string
	Namespace   string
	ContainerID string
	Status      WorkloadStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// WorkloadStatus represents the status of a workload
type WorkloadStatus string

const (
	WorkloadStatusPending WorkloadStatus = "Pending"
	WorkloadStatusRunning WorkloadStatus = "Running"
	WorkloadStatusStopped WorkloadStatus = "Stopped"
	WorkloadStatusFailed  WorkloadStatus = "Failed"
	WorkloadStatusUnknown WorkloadStatus = "Unknown"
)

// New creates a new Shuttle instance
func New(cfg *config.Config) (*Shuttle, error) {
	s := &Shuttle{
		config:    cfg,
		workloads: make(map[string]*WorkloadInstance),
	}

	// Initialize Tailscale client
	if cfg.Tailscale.Enabled {
		ts, err := tailscale.New(&cfg.Tailscale)
		if err != nil {
			return nil, fmt.Errorf("failed to create Tailscale client: %w", err)
		}
		s.tailscale = ts
	}

	// Initialize container runtime
	runtime, err := containerd.New(&cfg.Runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to create container runtime: %w", err)
	}
	s.runtime = runtime

	// Initialize gRPC client to Weaver
	grpcClient, err := grpc.New(&cfg.Weaver)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC client: %w", err)
	}
	s.grpcClient = grpcClient

	// Initialize metrics server
	if cfg.Metrics.Enabled {
		metrics, err := metrics.New(&cfg.Metrics)
		if err != nil {
			return nil, fmt.Errorf("failed to create metrics server: %w", err)
		}
		s.metrics = metrics
	}

	return s, nil
}

// Run starts the shuttle and runs until context is cancelled
func (s *Shuttle) Run(ctx context.Context) error {
	log.Printf("Starting Shuttle node: %s", s.config.Node.ID)

	// Start Tailscale if enabled
	if s.tailscale != nil {
		log.Println("Starting Tailscale...")
		if err := s.tailscale.Start(ctx); err != nil {
			return fmt.Errorf("failed to start Tailscale: %w", err)
		}
		defer s.tailscale.Stop()
	}

	// Start container runtime
	log.Println("Starting container runtime...")
	if err := s.runtime.Start(ctx); err != nil {
		return fmt.Errorf("failed to start container runtime: %w", err)
	}
	defer s.runtime.Stop()

	// Start metrics server
	if s.metrics != nil {
		log.Printf("Starting metrics server on port %d...", s.config.Metrics.Port)
		if err := s.metrics.Start(ctx); err != nil {
			return fmt.Errorf("failed to start metrics server: %w", err)
		}
		defer s.metrics.Stop()
	}

	// Register with Weaver
	log.Println("Registering with Weaver...")
	if err := s.registerWithWeaver(ctx); err != nil {
		return fmt.Errorf("failed to register with Weaver: %w", err)
	}

	// Start workload management loop
	log.Println("Starting workload management...")
	go s.workloadLoop(ctx)

	// Start health reporting
	go s.healthLoop(ctx)

	// Wait for shutdown
	<-ctx.Done()
	log.Println("Shutting down Shuttle...")

	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()

	// Stop all workloads
	if err := s.stopAllWorkloads(ctx); err != nil {
		log.Printf("Error stopping workloads: %v", err)
	}

	// Unregister from Weaver
	if err := s.unregisterFromWeaver(ctx); err != nil {
		log.Printf("Error unregistering from Weaver: %v", err)
	}

	return nil
}

// registerWithWeaver registers this node with the Weaver control plane
func (s *Shuttle) registerWithWeaver(ctx context.Context) error {
	nodeInfo := &grpc.NodeInfo{
		ID:       s.config.Node.ID,
		Name:     s.config.Node.Name,
		Region:   s.config.Node.Region,
		Zone:     s.config.Node.Zone,
		Labels:   s.config.Node.Labels,
		Capacity: s.config.Node.Capacity,
	}

	return s.grpcClient.RegisterNode(ctx, nodeInfo)
}

// unregisterFromWeaver unregisters this node from the Weaver control plane
func (s *Shuttle) unregisterFromWeaver(ctx context.Context) error {
	return s.grpcClient.UnregisterNode(ctx, s.config.Node.ID)
}

// workloadLoop manages workload lifecycle
func (s *Shuttle) workloadLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.syncWorkloads(ctx); err != nil {
				log.Printf("Error syncing workloads: %v", err)
			}
		}
	}
}

// syncWorkloads synchronizes workloads with Weaver
func (s *Shuttle) syncWorkloads(ctx context.Context) error {
	// Get assigned workloads from Weaver
	workloads, err := s.grpcClient.GetAssignedWorkloads(ctx, s.config.Node.ID)
	if err != nil {
		return fmt.Errorf("failed to get assigned workloads: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Track which workloads should be running
	shouldRun := make(map[string]*grpc.WorkloadSpec)
	for _, workload := range workloads {
		shouldRun[workload.ID] = workload
	}

	// Stop workloads that should no longer run
	for id, instance := range s.workloads {
		if _, exists := shouldRun[id]; !exists {
			log.Printf("Stopping workload %s", id)
			if err := s.stopWorkload(ctx, instance); err != nil {
				log.Printf("Error stopping workload %s: %v", id, err)
			}
			delete(s.workloads, id)
		}
	}

	// Start new workloads
	for id, spec := range shouldRun {
		if _, exists := s.workloads[id]; !exists {
			log.Printf("Starting workload %s", id)
			if err := s.startWorkload(ctx, spec); err != nil {
				log.Printf("Error starting workload %s: %v", id, err)
			}
		}
	}

	return nil
}

// startWorkload starts a new workload
func (s *Shuttle) startWorkload(ctx context.Context, spec *grpc.WorkloadSpec) error {
	// Create workload instance
	instance := &WorkloadInstance{
		ID:        spec.ID,
		Name:      spec.Name,
		Namespace: spec.Namespace,
		Status:    WorkloadStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Start container
	containerID, err := s.runtime.StartContainer(ctx, spec)
	if err != nil {
		instance.Status = WorkloadStatusFailed
		s.workloads[spec.ID] = instance
		return fmt.Errorf("failed to start container: %w", err)
	}

	instance.ContainerID = containerID
	instance.Status = WorkloadStatusRunning
	instance.UpdatedAt = time.Now()

	s.workloads[spec.ID] = instance

	// Report status to Weaver
	go s.reportWorkloadStatus(context.Background(), instance)

	return nil
}

// stopWorkload stops a running workload
func (s *Shuttle) stopWorkload(ctx context.Context, instance *WorkloadInstance) error {
	if instance.ContainerID != "" {
		if err := s.runtime.StopContainer(ctx, instance.ContainerID); err != nil {
			log.Printf("Error stopping container %s: %v", instance.ContainerID, err)
		}
	}

	instance.Status = WorkloadStatusStopped
	instance.UpdatedAt = time.Now()

	// Report status to Weaver
	go s.reportWorkloadStatus(context.Background(), instance)

	return nil
}

// stopAllWorkloads stops all running workloads
func (s *Shuttle) stopAllWorkloads(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, instance := range s.workloads {
		if err := s.stopWorkload(ctx, instance); err != nil {
			log.Printf("Error stopping workload %s: %v", instance.ID, err)
		}
	}

	return nil
}

// reportWorkloadStatus reports workload status to Weaver
func (s *Shuttle) reportWorkloadStatus(ctx context.Context, instance *WorkloadInstance) {
	status := &grpc.WorkloadStatus{
		WorkloadID: instance.ID,
		NodeID:     s.config.Node.ID,
		Status:     string(instance.Status),
		UpdatedAt:  instance.UpdatedAt,
	}

	if err := s.grpcClient.ReportWorkloadStatus(ctx, status); err != nil {
		log.Printf("Error reporting workload status: %v", err)
	}
}

// healthLoop reports node health to Weaver
func (s *Shuttle) healthLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.reportHealth(ctx); err != nil {
				log.Printf("Error reporting health: %v", err)
			}
		}
	}
}

// reportHealth reports node health to Weaver
func (s *Shuttle) reportHealth(ctx context.Context) error {
	s.mu.RLock()
	workloadCount := len(s.workloads)
	s.mu.RUnlock()

	health := &grpc.NodeHealth{
		NodeID:        s.config.Node.ID,
		Status:        "healthy",
		WorkloadCount: workloadCount,
		Timestamp:     time.Now(),
	}

	return s.grpcClient.ReportHealth(ctx, health)
}

// GetWorkloads returns current workloads
func (s *Shuttle) GetWorkloads() map[string]*WorkloadInstance {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*WorkloadInstance)
	for k, v := range s.workloads {
		result[k] = v
	}
	return result
}
