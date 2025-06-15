package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"time"
	weaver "weaver/internal/grpc"
	"weaver/internal/scheduler"
	"weaver/internal/state"
	"weaver/internal/workload"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	weaver.UnimplementedWeaverServiceServer
	appState   *state.State
	logger     *logrus.Logger
	grpcServer *grpc.Server
}

func NewServer(appState *state.State, logger *logrus.Logger) *Server {
	return &Server{
		appState: appState,
		logger:   logger,
	}
}

func (s *Server) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s.grpcServer = grpc.NewServer()
	weaver.RegisterWeaverServiceServer(s.grpcServer, s)

	s.logger.Infof("Starting gRPC server on %s", address)
	return s.grpcServer.Serve(lis)
}

func (s *Server) Stop() {
	if s.grpcServer != nil {
		s.logger.Info("Stopping gRPC server...")
		s.grpcServer.GracefulStop()
		s.logger.Info("gRPC server stopped")
	}
}

// generateID creates a random ID for workloads
func generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Workload management
func (s *Server) CreateWorkload(ctx context.Context, req *weaver.CreateWorkloadRequest) (*weaver.CreateWorkloadResponse, error) {
	now := time.Now()

	// Create workload with proper structure
	w := &workload.Workload{
		ID:          generateID(),
		Name:        req.Name,
		Namespace:   req.Namespace,
		Labels:      req.Labels,
		Annotations: req.Annotations,
		Spec:        convertWorkloadSpec(req.Spec),
		Status: workload.Status{
			Phase: workload.PhasePending,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Store workload in repository
	if s.appState.Repository != nil {
		if err := s.appState.Repository.CreateWorkload(ctx, w); err != nil {
			return nil, fmt.Errorf("failed to store workload: %v", err)
		}
	}

	// Schedule workload to get actual deployment details
	if s.appState.Scheduler != nil {
		placement, err := s.appState.Scheduler.Schedule(ctx, w)
		if err != nil {
			return nil, fmt.Errorf("failed to schedule workload: %v", err)
		}

		// Update workload status with placement information
		w.Status.Provider = placement.Provider
		if placement.Placement != nil {
			w.Status.NodeID = placement.Placement.NodeID
		}
		w.Status.Phase = workload.PhaseScheduled

		// Update workload in repository with placement info
		if s.appState.Repository != nil {
			if err := s.appState.Repository.UpdateWorkload(ctx, w); err != nil {
				s.logger.Warnf("Failed to update workload with placement info: %v", err)
			}
		}

		// Add proxy route if proxy is enabled and workload has ports
		if s.appState.Proxy != nil && len(w.Spec.Ports) > 0 && placement.Placement != nil {
			// Construct endpoint URL from placement information
			var endpoint string
			if placement.Placement.NodeID != "" {
				port := w.Spec.Ports[0].ContainerPort
				endpoint = fmt.Sprintf("http://%s:%d", placement.Placement.NodeID, port)
			}

			if endpoint != "" {
				if err := s.appState.Proxy.AddRoute(w, endpoint); err != nil {
					return nil, fmt.Errorf("failed to add proxy route: %v", err)
				}
			}
		}
	}

	return &weaver.CreateWorkloadResponse{
		Id:        w.ID,
		Name:      w.Name,
		Namespace: w.Namespace,
		Status:    convertWorkloadStatus(&w.Status),
		CreatedAt: timestamppb.New(w.CreatedAt),
	}, nil
}

func (s *Server) GetWorkload(ctx context.Context, req *weaver.GetWorkloadRequest) (*weaver.GetWorkloadResponse, error) {
	if s.appState.Repository == nil {
		return nil, fmt.Errorf("repository not configured")
	}

	w, err := s.appState.Repository.GetWorkload(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workload: %v", err)
	}

	return &weaver.GetWorkloadResponse{
		Workload: &weaver.Workload{
			Id:          w.ID,
			Name:        w.Name,
			Namespace:   w.Namespace,
			Labels:      w.Labels,
			Annotations: w.Annotations,
			Spec:        convertWorkloadSpecToProto(&w.Spec),
			Status:      convertWorkloadStatus(&w.Status),
			CreatedAt:   timestamppb.New(w.CreatedAt),
			UpdatedAt:   timestamppb.New(w.UpdatedAt),
		},
	}, nil
}

func (s *Server) ListWorkloads(ctx context.Context, req *weaver.ListWorkloadsRequest) (*weaver.ListWorkloadsResponse, error) {
	if s.appState.Repository == nil {
		return nil, fmt.Errorf("repository not configured")
	}

	workloads, err := s.appState.Repository.ListWorkloads(ctx, req.Namespace, req.LabelSelector)
	if err != nil {
		return nil, fmt.Errorf("failed to list workloads: %v", err)
	}

	var protoWorkloads []*weaver.Workload
	for _, w := range workloads {
		protoWorkloads = append(protoWorkloads, &weaver.Workload{
			Id:          w.ID,
			Name:        w.Name,
			Namespace:   w.Namespace,
			Labels:      w.Labels,
			Annotations: w.Annotations,
			Spec:        convertWorkloadSpecToProto(&w.Spec),
			Status:      convertWorkloadStatus(&w.Status),
			CreatedAt:   timestamppb.New(w.CreatedAt),
			UpdatedAt:   timestamppb.New(w.UpdatedAt),
		})
	}

	return &weaver.ListWorkloadsResponse{
		Workloads: protoWorkloads,
		Total:     int32(len(protoWorkloads)),
	}, nil
}

func (s *Server) DeleteWorkload(ctx context.Context, req *weaver.DeleteWorkloadRequest) (*emptypb.Empty, error) {
	if s.appState.Repository == nil {
		return nil, fmt.Errorf("repository not configured")
	}

	// Get workload first to remove proxy route
	w, err := s.appState.Repository.GetWorkload(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workload: %v", err)
	}

	// Remove proxy route if proxy is enabled
	if s.appState.Proxy != nil {
		s.appState.Proxy.RemoveRoute(w)
	}

	// Delete workload from repository
	if err := s.appState.Repository.DeleteWorkload(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("failed to delete workload: %v", err)
	}

	return &emptypb.Empty{}, nil
}

// Provider management
func (s *Server) ListProviders(ctx context.Context, req *emptypb.Empty) (*weaver.ListProvidersResponse, error) {
	providers := make([]string, 0, len(s.appState.Providers))
	for name := range s.appState.Providers {
		providers = append(providers, name)
	}
	return &weaver.ListProvidersResponse{Providers: providers}, nil
}

func (s *Server) GetProviderRegions(ctx context.Context, req *weaver.GetProviderRegionsRequest) (*weaver.GetProviderRegionsResponse, error) {
	provider, exists := s.appState.Providers[req.Provider]
	if !exists {
		return nil, fmt.Errorf("provider not found")
	}

	// Get available resources which includes region information
	resources, err := provider.GetAvailableResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider resources: %v", err)
	}

	var regions []string
	for _, region := range resources.Regions {
		if region.Available {
			regions = append(regions, region.Name)
		}
	}

	return &weaver.GetProviderRegionsResponse{
		Regions: regions,
	}, nil
}

func (s *Server) GetProviderMachineTypes(ctx context.Context, req *weaver.GetProviderMachineTypesRequest) (*weaver.GetProviderMachineTypesResponse, error) {
	provider, exists := s.appState.Providers[req.Provider]
	if !exists {
		return nil, fmt.Errorf("provider not found")
	}

	// Get pricing information to determine machine types
	pricing, err := provider.GetPricing(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider pricing: %v", err)
	}

	// Get available resources for GPU information
	resources, err := provider.GetAvailableResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider resources: %v", err)
	}

	var machineTypes []*weaver.MachineType

	// Standard CPU/Memory machine types
	standardTypes := []struct {
		name   string
		cpu    string
		memory string
		cpuNum float64
		memGB  float64
	}{
		{"micro", "0.5", "1Gi", 0.5, 1.0},
		{"small", "1", "2Gi", 1.0, 2.0},
		{"medium", "2", "4Gi", 2.0, 4.0},
		{"large", "4", "8Gi", 4.0, 8.0},
		{"xlarge", "8", "16Gi", 8.0, 16.0},
		{"2xlarge", "16", "32Gi", 16.0, 32.0},
		{"4xlarge", "32", "64Gi", 32.0, 64.0},
		{"8xlarge", "64", "128Gi", 64.0, 128.0},
		{"16xlarge", "128", "256Gi", 128.0, 256.0},
	}

	for _, mt := range standardTypes {
		price := mt.cpuNum*pricing.CPU.Amount + mt.memGB*pricing.Memory.Amount
		machineTypes = append(machineTypes, &weaver.MachineType{
			Name:         mt.name,
			Cpu:          mt.cpu,
			Memory:       mt.memory,
			Gpu:          "",
			PricePerHour: price,
		})
	}

	// GPU machine types
	for gpuType, gpuInfo := range resources.GPU.Types {
		if gpuInfo.Available > 0 {
			machineTypes = append(machineTypes, &weaver.MachineType{
				Name:         fmt.Sprintf("gpu-%s", gpuType),
				Cpu:          "8", // Standard 8 vCPU for GPU instances
				Memory:       "32Gi",
				Gpu:          gpuType,
				PricePerHour: gpuInfo.PricePerHour,
			})
		}
	}

	return &weaver.GetProviderMachineTypesResponse{
		MachineTypes: machineTypes,
	}, nil
}

// Scheduler
func (s *Server) GetSchedulerStatus(ctx context.Context, req *emptypb.Empty) (*weaver.GetSchedulerStatusResponse, error) {
	response := &weaver.GetSchedulerStatusResponse{
		Status:         "running",
		ProvidersCount: int32(len(s.appState.Providers)),
	}

	if s.appState.Scheduler != nil {
		if err := s.appState.Scheduler.HealthCheck(ctx); err != nil {
			response.SchedulerStatus = "unhealthy"
			response.SchedulerError = err.Error()
		} else {
			response.SchedulerStatus = "healthy"
		}
	} else {
		response.SchedulerStatus = "not_configured"
	}

	return response, nil
}

func (s *Server) ScheduleWorkload(ctx context.Context, req *weaver.ScheduleWorkloadRequest) (*weaver.ScheduleWorkloadResponse, error) {
	if s.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	// Create a temporary workload for scheduling
	w := &workload.Workload{
		ID:   generateID(),
		Spec: convertWorkloadSpec(req.Spec),
	}

	// Schedule the workload
	result, err := s.appState.Scheduler.Schedule(ctx, w)
	if err != nil {
		return nil, fmt.Errorf("failed to schedule workload: %v", err)
	}

	response := &weaver.ScheduleWorkloadResponse{
		Provider:      result.Provider,
		Region:        result.Region,
		EstimatedCost: result.EstimatedCost.HourlyCost,
	}

	if result.Placement != nil {
		response.Zone = result.Placement.Zone
		response.NodeId = result.Placement.NodeID
	}

	return response, nil
}

func (s *Server) GetRecommendations(ctx context.Context, req *weaver.GetRecommendationsRequest) (*weaver.GetRecommendationsResponse, error) {
	if s.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	// Create a temporary workload for getting recommendations
	w := &workload.Workload{
		ID:   generateID(),
		Spec: convertWorkloadSpec(req.Spec),
	}

	// Get recommendations from scheduler
	recommendations, err := s.appState.Scheduler.GetRecommendations(ctx, w)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %v", err)
	}

	var protoRecommendations []*weaver.ScheduleRecommendation
	for _, rec := range recommendations {
		protoRecommendations = append(protoRecommendations, &weaver.ScheduleRecommendation{
			Provider:         rec.Provider,
			Region:           rec.Region,
			Zone:             "", // Zone not available in current recommendation structure
			CostPerHour:      rec.EstimatedCost.HourlyCost,
			PerformanceScore: rec.Score,
			Reason:           fmt.Sprintf("Score: %.2f, Confidence: %.2f", rec.Score, rec.Confidence),
		})
	}

	return &weaver.GetRecommendationsResponse{
		Recommendations: protoRecommendations,
	}, nil
}

func (s *Server) GetSchedulerStats(ctx context.Context, req *emptypb.Empty) (*weaver.GetSchedulerStatsResponse, error) {
	if s.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	stats, err := s.appState.Scheduler.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate pending workloads from repository if available
	var pendingWorkloads int32
	if s.appState.Repository != nil {
		// Get all workloads and count those in pending state
		workloads, err := s.appState.Repository.ListWorkloads(ctx, "", nil)
		if err == nil {
			for _, w := range workloads {
				if w.Status.Phase == workload.PhasePending ||
					w.Status.Phase == workload.PhaseScheduled {
					pendingWorkloads++
				}
			}
		}
	}

	return &weaver.GetSchedulerStatsResponse{
		TotalWorkloads:      int32(stats.TotalScheduled),
		RunningWorkloads:    int32(stats.SuccessfulSchedules),
		PendingWorkloads:    pendingWorkloads,
		FailedWorkloads:     int32(stats.FailedSchedules),
		WorkloadsByProvider: convertProviderStats(stats.ProviderStats),
		TotalCostPerHour:    calculateTotalCost(stats.ProviderStats),
	}, nil
}

// Health check
func (s *Server) HealthCheck(ctx context.Context, req *emptypb.Empty) (*weaver.HealthCheckResponse, error) {
	return &weaver.HealthCheckResponse{
		Status:    "ok",
		Service:   "weaver",
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}

// Helper functions to convert between internal types and protobuf types
func convertWorkloadSpec(spec *weaver.WorkloadSpec) workload.Spec {
	if spec == nil {
		return workload.Spec{}
	}

	result := workload.Spec{
		Image:   spec.Image,
		Command: spec.Command,
		Args:    spec.Args,
		Env:     spec.Env,
	}

	if spec.Resources != nil {
		result.Resources = workload.ResourceRequests{
			CPU:    spec.Resources.Cpu,
			Memory: spec.Resources.Memory,
			GPU:    spec.Resources.Gpu,
		}
	}

	for _, volume := range spec.Volumes {
		result.Volumes = append(result.Volumes, workload.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			ReadOnly:  volume.ReadOnly,
			ContentID: volume.ContentId,
		})
	}

	for _, port := range spec.Ports {
		result.Ports = append(result.Ports, workload.Port{
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			Protocol:      port.Protocol,
		})
	}

	for _, sidecar := range spec.Sidecars {
		result.Sidecars = append(result.Sidecars, workload.SidecarSpec{
			Name:    sidecar.Name,
			Image:   sidecar.Image,
			Command: sidecar.Command,
			Args:    sidecar.Args,
			Env:     sidecar.Env,
		})
	}

	result.Restart = workload.RestartPolicy(spec.RestartPolicy)

	if spec.Placement != nil {
		result.Placement = workload.PlacementSpec{
			Provider:   spec.Placement.Provider,
			Region:     spec.Placement.Region,
			Zone:       spec.Placement.Zone,
			NodeLabels: spec.Placement.NodeLabels,
		}

		for _, toleration := range spec.Placement.Tolerations {
			result.Placement.Tolerations = append(result.Placement.Tolerations, workload.Toleration{
				Key:      toleration.Key,
				Operator: toleration.Operator,
				Value:    toleration.Value,
				Effect:   toleration.Effect,
			})
		}
	}

	return result
}

func convertWorkloadStatus(status *workload.Status) *weaver.WorkloadStatus {
	if status == nil {
		return nil
	}

	result := &weaver.WorkloadStatus{
		Phase:        string(status.Phase),
		Message:      status.Message,
		Reason:       status.Reason,
		RestartCount: status.RestartCount,
		NodeId:       status.NodeID,
		Provider:     status.Provider,
		TailscaleIp:  status.TailscaleIP,
		ContainerId:  status.ContainerID,
		SnapshotId:   status.SnapshotID,
	}

	if status.StartTime != nil {
		result.StartTime = timestamppb.New(*status.StartTime)
	}
	if status.FinishTime != nil {
		result.FinishTime = timestamppb.New(*status.FinishTime)
	}
	if status.LastSnapshot != nil {
		result.LastSnapshot = timestamppb.New(*status.LastSnapshot)
	}

	return result
}

func convertWorkloadsByProvider(workloads map[string]int) map[string]int32 {
	result := make(map[string]int32)
	for k, v := range workloads {
		result[k] = int32(v)
	}
	return result
}

func convertProviderStats(providerStats map[string]*scheduler.ProviderStats) map[string]int32 {
	result := make(map[string]int32)
	for provider, stats := range providerStats {
		if stats != nil {
			result[provider] = int32(stats.TotalScheduled)
		}
	}
	return result
}

func calculateTotalCost(providerStats map[string]*scheduler.ProviderStats) float64 {
	var totalCost float64
	for _, stats := range providerStats {
		if stats != nil {
			totalCost += stats.AverageCost
		}
	}
	return totalCost
}

func convertWorkloadSpecToProto(spec *workload.Spec) *weaver.WorkloadSpec {
	if spec == nil {
		return nil
	}

	result := &weaver.WorkloadSpec{
		Image:         spec.Image,
		Command:       spec.Command,
		Args:          spec.Args,
		Env:           spec.Env,
		RestartPolicy: string(spec.Restart),
	}

	// Convert resources
	result.Resources = &weaver.ResourceRequests{
		Cpu:    spec.Resources.CPU,
		Memory: spec.Resources.Memory,
		Gpu:    spec.Resources.GPU,
	}

	// Convert volumes
	for _, volume := range spec.Volumes {
		result.Volumes = append(result.Volumes, &weaver.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			ReadOnly:  volume.ReadOnly,
			ContentId: volume.ContentID,
		})
	}

	// Convert ports
	for _, port := range spec.Ports {
		result.Ports = append(result.Ports, &weaver.Port{
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			Protocol:      port.Protocol,
		})
	}

	// Convert sidecars
	for _, sidecar := range spec.Sidecars {
		result.Sidecars = append(result.Sidecars, &weaver.SidecarSpec{
			Name:    sidecar.Name,
			Image:   sidecar.Image,
			Command: sidecar.Command,
			Args:    sidecar.Args,
			Env:     sidecar.Env,
		})
	}

	// Convert placement
	if spec.Placement.Provider != "" || spec.Placement.Region != "" || spec.Placement.Zone != "" {
		result.Placement = &weaver.PlacementSpec{
			Provider:   spec.Placement.Provider,
			Region:     spec.Placement.Region,
			Zone:       spec.Placement.Zone,
			NodeLabels: spec.Placement.NodeLabels,
		}

		for _, toleration := range spec.Placement.Tolerations {
			result.Placement.Tolerations = append(result.Placement.Tolerations, &weaver.Toleration{
				Key:      toleration.Key,
				Operator: toleration.Operator,
				Value:    toleration.Value,
				Effect:   toleration.Effect,
			})
		}
	}

	return result
}
