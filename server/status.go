package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	core "k8s.io/api/core/v1"
)

type StatusRequest struct {
	ID string `json:"id"`
}

type StatusResponse struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Status        string            `json:"status"`
	PodIP         string            `json:"podIP,omitempty"`
	StartTime     *time.Time        `json:"startTime,omitempty"`
	RestartCount  int32             `json:"restartCount"`
	Ready         bool              `json:"ready"`
	Image         string            `json:"image"`
	Labels        map[string]string `json:"labels"`
	Error         string            `json:"error,omitempty"`
	ContainerInfo []ContainerStatus `json:"containers,omitempty"`
}

type ContainerStatus struct {
	Name         string     `json:"name"`
	Ready        bool       `json:"ready"`
	Started      bool       `json:"started"`
	RestartCount int32      `json:"restartCount"`
	State        string     `json:"state"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
}

func (s *Server) Status(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	resp := buildStatusResponse(pod, id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func buildStatusResponse(pod *core.Pod, id string) StatusResponse {
	resp := StatusResponse{
		ID:           id,
		Name:         pod.Name,
		Status:       string(pod.Status.Phase),
		PodIP:        pod.Status.PodIP,
		RestartCount: 0,
		Ready:        isPodReady(pod),
		Image:        pod.Spec.Containers[0].Image,
		Labels:       pod.Labels,
	}

	if pod.Status.StartTime != nil {
		startTime := pod.Status.StartTime.Time
		resp.StartTime = &startTime
	}

	resp.ContainerInfo = make([]ContainerStatus, 0, len(pod.Status.ContainerStatuses))
	for _, cs := range pod.Status.ContainerStatuses {
		containerStatus := ContainerStatus{
			Name:         cs.Name,
			Ready:        cs.Ready,
			Started:      *cs.Started,
			RestartCount: cs.RestartCount,
		}

		resp.RestartCount += cs.RestartCount

		if cs.State.Running != nil {
			containerStatus.State = "running"
			startTime := cs.State.Running.StartedAt.Time
			containerStatus.StartedAt = &startTime
		} else if cs.State.Waiting != nil {
			containerStatus.State = "waiting"
		} else if cs.State.Terminated != nil {
			containerStatus.State = "terminated"
		}

		resp.ContainerInfo = append(resp.ContainerInfo, containerStatus)
	}

	return resp
}

func isPodReady(pod *core.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == core.PodReady {
			return condition.Status == core.ConditionTrue
		}
	}
	return false
}
