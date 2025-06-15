package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fabric/internal/scheduler"
	"fabric/internal/state"
	"fabric/internal/types"
	"fabric/proto/weaver"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	weaver.UnimplementedWeaverServiceServer
	appState *state.State
	logger   *logrus.Logger
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

	grpcServer := grpc.NewServer()
	weaver.RegisterWeaverServiceServer(grpcServer, s)

	s.logger.Infof("Starting gRPC server on %s", address)
	return grpcServer.Serve(lis)
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
	workload := &types.Workload{
		ID:          generateID(),
		Name:        req.Name,
		Namespace:   req.Namespace,
		Labels:      req.Labels,
		Annotations: req.Annotations,
		Spec:        convertWorkloadSpec(req.Spec),
		Status: types.WorkloadStatus{
			Phase: types.WorkloadPhasePending,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// TODO: Store workload in repository
	// For now, just simulate successful creation

	// Add proxy route if proxy is enabled and workload has ports
	if s.appState.Proxy != nil && len(workload.Spec.Ports) > 0 {
		// Simulate target URL (in real implementation, this would come from the provider)
		targetURL := "http://localhost:8080" // This would be the actual workload endpoint
		if err := s.appState.Proxy.AddRoute(workload, targetURL); err != nil {
			return nil, fmt.Errorf("failed to add proxy route: %v", err)
		}
	}

	return &weaver.CreateWorkloadResponse{
		Id:        workload.ID,
		Name:      workload.Name,
		Namespace: workload.Namespace,
		Status:    convertWorkloadStatus(&workload.Status),
		CreatedAt: timestamppb.New(workload.CreatedAt),
	}, nil
}

func (s *Server) GetWorkload(ctx context.Context, req *weaver.GetWorkloadRequest) (*weaver.GetWorkloadResponse, error) {
	// TODO: Implement workload retrieval
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) ListWorkloads(ctx context.Context, req *weaver.ListWorkloadsRequest) (*weaver.ListWorkloadsResponse, error) {
	// TODO: Implement workload listing
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) DeleteWorkload(ctx context.Context, req *weaver.DeleteWorkloadRequest) (*emptypb.Empty, error) {
	// TODO: Implement workload deletion
	return nil, fmt.Errorf("not implemented")
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
	_, exists := s.appState.Providers[req.Provider]
	if !exists {
		return nil, fmt.Errorf("provider not found")
	}

	// TODO: Update to use new provider interface
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetProviderMachineTypes(ctx context.Context, req *weaver.GetProviderMachineTypesRequest) (*weaver.GetProviderMachineTypesResponse, error) {
	_, exists := s.appState.Providers[req.Provider]
	if !exists {
		return nil, fmt.Errorf("provider not found")
	}

	// TODO: Update to use new provider interface
	return nil, fmt.Errorf("not implemented")
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

	// TODO: Parse workload from request body and schedule it
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetRecommendations(ctx context.Context, req *weaver.GetRecommendationsRequest) (*weaver.GetRecommendationsResponse, error) {
	if s.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	// TODO: Parse workload from query params and get recommendations
	return nil, fmt.Errorf("not implemented")
}

func (s *Server) GetSchedulerStats(ctx context.Context, req *emptypb.Empty) (*weaver.GetSchedulerStatsResponse, error) {
	if s.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	stats, err := s.appState.Scheduler.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	return &weaver.GetSchedulerStatsResponse{
		TotalWorkloads:      int32(stats.TotalScheduled),
		RunningWorkloads:    int32(stats.SuccessfulSchedules),
		PendingWorkloads:    0, // TODO: Add pending workloads tracking
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
func convertWorkloadSpec(spec *weaver.WorkloadSpec) types.WorkloadSpec {
	if spec == nil {
		return types.WorkloadSpec{}
	}

	result := types.WorkloadSpec{
		Image:   spec.Image,
		Command: spec.Command,
		Args:    spec.Args,
		Env:     spec.Env,
	}

	if spec.Resources != nil {
		result.Resources = types.ResourceRequests{
			CPU:    spec.Resources.Cpu,
			Memory: spec.Resources.Memory,
			GPU:    spec.Resources.Gpu,
		}
	}

	for _, volume := range spec.Volumes {
		result.Volumes = append(result.Volumes, types.VolumeMount{
			Name:      volume.Name,
			MountPath: volume.MountPath,
			ReadOnly:  volume.ReadOnly,
			ContentID: volume.ContentId,
		})
	}

	for _, port := range spec.Ports {
		result.Ports = append(result.Ports, types.Port{
			Name:          port.Name,
			ContainerPort: port.ContainerPort,
			Protocol:      port.Protocol,
		})
	}

	for _, sidecar := range spec.Sidecars {
		result.Sidecars = append(result.Sidecars, types.SidecarSpec{
			Name:    sidecar.Name,
			Image:   sidecar.Image,
			Command: sidecar.Command,
			Args:    sidecar.Args,
			Env:     sidecar.Env,
		})
	}

	result.Restart = types.RestartPolicy(spec.RestartPolicy)

	if spec.Placement != nil {
		result.Placement = types.PlacementSpec{
			Provider:   spec.Placement.Provider,
			Region:     spec.Placement.Region,
			Zone:       spec.Placement.Zone,
			NodeLabels: spec.Placement.NodeLabels,
		}

		for _, toleration := range spec.Placement.Tolerations {
			result.Placement.Tolerations = append(result.Placement.Tolerations, types.Toleration{
				Key:      toleration.Key,
				Operator: toleration.Operator,
				Value:    toleration.Value,
				Effect:   toleration.Effect,
			})
		}
	}

	return result
}

func convertWorkloadStatus(status *types.WorkloadStatus) *weaver.WorkloadStatus {
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
