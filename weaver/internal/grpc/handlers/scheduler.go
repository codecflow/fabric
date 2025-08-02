package handlers

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/codecflow/fabric/pkg/workload"
	"github.com/codecflow/fabric/weaver/internal/state"
	"github.com/codecflow/fabric/weaver/weaver/proto/weaver"
)

type SchedulerHandler struct {
	appState *state.State
	logger   *logrus.Logger
}

func NewSchedulerHandler(appState *state.State, logger *logrus.Logger) *SchedulerHandler {
	return &SchedulerHandler{
		appState: appState,
		logger:   logger,
	}
}

func (h *SchedulerHandler) GetStatus(ctx context.Context, req *emptypb.Empty) (*weaver.GetSchedulerStatusResponse, error) {
	response := &weaver.GetSchedulerStatusResponse{
		Status:         "running",
		ProvidersCount: int32(len(h.appState.Providers)), // nolint:gosec
	}

	if h.appState.Scheduler != nil {
		if err := h.appState.Scheduler.HealthCheck(ctx); err != nil {
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

func (h *SchedulerHandler) Schedule(ctx context.Context, req *weaver.ScheduleWorkloadRequest) (*weaver.ScheduleWorkloadResponse, error) {
	if h.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	w := &workload.Workload{
		ID:   generateID(),
		Spec: convertWorkloadSpec(req.Spec),
	}

	result, err := h.appState.Scheduler.Schedule(ctx, w)
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

func (h *SchedulerHandler) GetRecommendations(ctx context.Context, req *weaver.GetRecommendationsRequest) (*weaver.GetRecommendationsResponse, error) {
	if h.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	w := &workload.Workload{
		ID:   generateID(),
		Spec: convertWorkloadSpec(req.Spec),
	}

	recommendations, err := h.appState.Scheduler.GetRecommendations(ctx, w)
	if err != nil {
		return nil, fmt.Errorf("failed to get recommendations: %v", err)
	}

	var protoRecommendations []*weaver.ScheduleRecommendation
	for _, rec := range recommendations {
		protoRecommendations = append(protoRecommendations, &weaver.ScheduleRecommendation{
			Provider:         rec.Provider,
			Region:           rec.Region,
			Zone:             "",
			CostPerHour:      rec.EstimatedCost.HourlyCost,
			PerformanceScore: rec.Score,
			Reason:           fmt.Sprintf("Score: %.2f, Confidence: %.2f", rec.Score, rec.Confidence),
		})
	}

	return &weaver.GetRecommendationsResponse{
		Recommendations: protoRecommendations,
	}, nil
}

func (h *SchedulerHandler) GetStats(ctx context.Context, req *emptypb.Empty) (*weaver.GetSchedulerStatsResponse, error) {
	if h.appState.Scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}

	stats, err := h.appState.Scheduler.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	var pendingWorkloads int32
	workloads, err := h.appState.Repository.Workload.List(ctx, "", nil)
	if err == nil {
		for _, w := range workloads {
			if w.Status.Phase == workload.PhasePending ||
				w.Status.Phase == workload.PhaseScheduled {
				pendingWorkloads++
			}
		}
	}

	return &weaver.GetSchedulerStatsResponse{
		TotalWorkloads:      int32(stats.TotalScheduled),      // nolint:gosec
		RunningWorkloads:    int32(stats.SuccessfulSchedules), // nolint:gosec
		PendingWorkloads:    pendingWorkloads,
		FailedWorkloads:     int32(stats.FailedSchedules), // nolint:gosec
		WorkloadsByProvider: convertProviderStats(stats.ProviderStats),
		TotalCostPerHour:    calculateTotalCost(stats.ProviderStats),
	}, nil
}
