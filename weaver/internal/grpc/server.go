package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/codecflow/fabric/weaver/internal/grpc/handlers"
	"github.com/codecflow/fabric/weaver/internal/state"
	"github.com/codecflow/fabric/weaver/weaver/proto/weaver"
)

// Server implements the gRPC server for Weaver
type Server struct {
	weaver.UnimplementedWeaverServiceServer
	appState *state.State
	logger   *logrus.Logger
	server   *grpc.Server

	// Handlers
	workload  *handlers.WorkloadHandler
	provider  *handlers.ProviderHandler
	scheduler *handlers.SchedulerHandler
}

// NewServer creates a new gRPC server instance
func NewServer(appState *state.State, logger *logrus.Logger) *Server {
	return &Server{
		appState:  appState,
		logger:    logger,
		workload:  handlers.NewWorkloadHandler(appState, logger),
		provider:  handlers.NewProviderHandler(appState, logger),
		scheduler: handlers.NewSchedulerHandler(appState, logger),
	}
}

// Start starts the gRPC server
func (s *Server) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s.server = grpc.NewServer()
	weaver.RegisterWeaverServiceServer(s.server, s)

	s.logger.Infof("Starting gRPC server on %s", address)
	return s.server.Serve(lis)
}

// Stop gracefully stops the gRPC server
func (s *Server) Stop() {
	if s.server != nil {
		s.logger.Info("Stopping gRPC server...")
		s.server.GracefulStop()
		s.logger.Info("gRPC server stopped")
	}
}

// Workload management methods
func (s *Server) CreateWorkload(ctx context.Context, req *weaver.CreateWorkloadRequest) (*weaver.CreateWorkloadResponse, error) {
	return s.workload.Create(ctx, req)
}

func (s *Server) GetWorkload(ctx context.Context, req *weaver.GetWorkloadRequest) (*weaver.GetWorkloadResponse, error) {
	return s.workload.Get(ctx, req)
}

func (s *Server) ListWorkloads(ctx context.Context, req *weaver.ListWorkloadsRequest) (*weaver.ListWorkloadsResponse, error) {
	return s.workload.List(ctx, req)
}

func (s *Server) DeleteWorkload(ctx context.Context, req *weaver.DeleteWorkloadRequest) (*emptypb.Empty, error) {
	return s.workload.Delete(ctx, req)
}

// Provider management methods
func (s *Server) ListProviders(ctx context.Context, req *emptypb.Empty) (*weaver.ListProvidersResponse, error) {
	return s.provider.List(ctx, req)
}

func (s *Server) GetProviderRegions(ctx context.Context, req *weaver.GetProviderRegionsRequest) (*weaver.GetProviderRegionsResponse, error) {
	return s.provider.GetRegions(ctx, req)
}

func (s *Server) GetProviderMachineTypes(ctx context.Context, req *weaver.GetProviderMachineTypesRequest) (*weaver.GetProviderMachineTypesResponse, error) {
	return s.provider.GetMachineTypes(ctx, req)
}

// Scheduler methods
func (s *Server) GetSchedulerStatus(ctx context.Context, req *emptypb.Empty) (*weaver.GetSchedulerStatusResponse, error) {
	return s.scheduler.GetStatus(ctx, req)
}

func (s *Server) ScheduleWorkload(ctx context.Context, req *weaver.ScheduleWorkloadRequest) (*weaver.ScheduleWorkloadResponse, error) {
	return s.scheduler.Schedule(ctx, req)
}

func (s *Server) GetRecommendations(ctx context.Context, req *weaver.GetRecommendationsRequest) (*weaver.GetRecommendationsResponse, error) {
	return s.scheduler.GetRecommendations(ctx, req)
}

func (s *Server) GetSchedulerStats(ctx context.Context, req *emptypb.Empty) (*weaver.GetSchedulerStatsResponse, error) {
	return s.scheduler.GetStats(ctx, req)
}

// Health check
func (s *Server) HealthCheck(ctx context.Context, req *emptypb.Empty) (*weaver.HealthCheckResponse, error) {
	return &weaver.HealthCheckResponse{
		Status:    "ok",
		Service:   "weaver",
		Timestamp: timestamppb.New(time.Now()),
	}, nil
}
