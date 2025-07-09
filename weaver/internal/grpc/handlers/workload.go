package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/codecflow/fabric/weaver/internal/state"
	"github.com/codecflow/fabric/weaver/internal/workload"
	"github.com/codecflow/fabric/weaver/weaver/proto/weaver"
)

type WorkloadHandler struct {
	appState *state.State
	logger   *logrus.Logger
}

func NewWorkloadHandler(appState *state.State, logger *logrus.Logger) *WorkloadHandler {
	return &WorkloadHandler{
		appState: appState,
		logger:   logger,
	}
}

func (h *WorkloadHandler) Create(ctx context.Context, req *weaver.CreateWorkloadRequest) (*weaver.CreateWorkloadResponse, error) {
	now := time.Now()

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

	if err := h.appState.Repository.Workload.Create(ctx, w); err != nil {
		return nil, fmt.Errorf("failed to store workload: %v", err)
	}

	// Schedule workload
	if h.appState.Scheduler != nil {
		placement, err := h.appState.Scheduler.Schedule(ctx, w)
		if err != nil {
			return nil, fmt.Errorf("failed to schedule workload: %v", err)
		}

		w.Status.Provider = placement.Provider
		if placement.Placement != nil {
			w.Status.NodeID = placement.Placement.NodeID
		}
		w.Status.Phase = workload.PhaseScheduled

		if err := h.appState.Repository.Workload.Update(ctx, w); err != nil {
			h.logger.Warnf("Failed to update workload with placement info: %v", err)
		}

		// Add proxy route
		if h.appState.Proxy != nil && len(w.Spec.Ports) > 0 && placement.Placement != nil {
			var endpoint string
			if placement.Placement.NodeID != "" {
				port := w.Spec.Ports[0].ContainerPort
				endpoint = fmt.Sprintf("http://%s:%d", placement.Placement.NodeID, port)
			}

			if endpoint != "" {
				if err := h.appState.Proxy.AddRoute(w, endpoint); err != nil {
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

func (h *WorkloadHandler) Get(ctx context.Context, req *weaver.GetWorkloadRequest) (*weaver.GetWorkloadResponse, error) {
	if h.appState.Repository.Workload == nil {
		return nil, fmt.Errorf("workload repository not available")
	}

	w, err := h.appState.Repository.Workload.Get(ctx, req.Id)
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

func (h *WorkloadHandler) List(ctx context.Context, req *weaver.ListWorkloadsRequest) (*weaver.ListWorkloadsResponse, error) {
	if h.appState.Repository.Workload == nil {
		return nil, fmt.Errorf("workload repository not available")
	}

	workloads, err := h.appState.Repository.Workload.List(ctx, req.Namespace, req.LabelSelector)
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
		Total:     int32(len(protoWorkloads)), // nolint:gosec
	}, nil
}

func (h *WorkloadHandler) Delete(ctx context.Context, req *weaver.DeleteWorkloadRequest) (*emptypb.Empty, error) {
	if h.appState.Repository.Workload == nil {
		return nil, fmt.Errorf("workload repository not available")
	}

	w, err := h.appState.Repository.Workload.Get(ctx, req.Id)
	if err != nil {
		return nil, fmt.Errorf("failed to get workload: %v", err)
	}

	if h.appState.Proxy != nil {
		h.appState.Proxy.RemoveRoute(w)
	}

	if err := h.appState.Repository.Workload.Delete(ctx, req.Id); err != nil {
		return nil, fmt.Errorf("failed to delete workload: %v", err)
	}

	return &emptypb.Empty{}, nil
}
