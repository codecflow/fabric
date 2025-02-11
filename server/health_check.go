package server

import (
	"captain/k8s"
	"context"
	"fmt"
	"sync"
	"time"

	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
)

type HealthStatus string

const (
	StatusHealthy   HealthStatus = "healthy"
	StatusUnhealthy HealthStatus = "unhealthy"
	StatusRecovery  HealthStatus = "recovery"
	StatusUnknown   HealthStatus = "unknown"
)

type MachineHealth struct {
	ID               string       `json:"id"`
	Status           HealthStatus `json:"status"`
	LastChecked      time.Time    `json:"lastChecked"`
	RecoveryAttempts int          `json:"recoveryAttempts"`
	Message          string       `json:"message"`
}

type HealthStore struct {
	health map[string]MachineHealth
	mu     sync.RWMutex
}

func NewHealthStore() *HealthStore {
	return &HealthStore{
		health: make(map[string]MachineHealth),
	}
}

func (s *HealthStore) Get(id string) (MachineHealth, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	health, ok := s.health[id]
	return health, ok
}

func (s *HealthStore) Set(id string, health MachineHealth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.health[id] = health
}

func (s *HealthStore) Delete(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.health, id)
}

func (s *HealthStore) List() []MachineHealth {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]MachineHealth, 0, len(s.health))
	for _, health := range s.health {
		result = append(result, health)
	}
	return result
}

func (s *Server) StartHealthChecker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CheckAllMachines(ctx)
		}
	}
}

func (s *Server) CheckAllMachines(ctx context.Context) {
	pods, err := s.client.ListAllPods(ctx)
	if err != nil {
		s.logger.Errorf("Failed to list pods: %v", err)
		return
	}

	for _, pod := range pods.Items {
		if pod.Labels["app"] != "machine" {
			continue
		}

		id := pod.Labels["machine-id"]
		if id == "" {
			continue
		}

		go s.CheckMachineHealth(ctx, id, &pod)
	}
}

func (s *Server) CheckMachineHealth(ctx context.Context, id string, pod *core.Pod) {
	health, exists := s.health.Get(id)
	if !exists {
		health = MachineHealth{
			ID:               id,
			Status:           StatusUnknown,
			LastChecked:      time.Now(),
			RecoveryAttempts: 0,
		}
	}

	health.LastChecked = time.Now()

	// Check pod status
	if pod.Status.Phase == core.PodRunning {
		// Check container status
		allContainersReady := true
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if !containerStatus.Ready {
				allContainersReady = false
				break
			}
		}

		if allContainersReady {
			health.Status = StatusHealthy
			health.Message = "All containers are running and ready"
			health.RecoveryAttempts = 0
		} else {
			health.Status = StatusUnhealthy
			health.Message = "Not all containers are ready"
		}
	} else if pod.Status.Phase == core.PodPending {
		health.Status = StatusUnhealthy
		health.Message = fmt.Sprintf("Pod is in pending state: %s", pod.Status.Reason)
	} else if pod.Status.Phase == core.PodFailed || pod.Status.Phase == core.PodUnknown {
		health.Status = StatusUnhealthy
		health.Message = fmt.Sprintf("Pod is in %s state: %s", pod.Status.Phase, pod.Status.Reason)
	}

	// If machine is unhealthy, attempt recovery
	if health.Status == StatusUnhealthy {
		if health.RecoveryAttempts < 3 {
			health.Status = StatusRecovery
			health.RecoveryAttempts++
			health.Message = fmt.Sprintf("Attempting recovery (%d/3)", health.RecoveryAttempts)
			s.health.Set(id, health)

			// Attempt recovery
			go s.RecoverMachine(ctx, id)
		} else {
			health.Message = "Maximum recovery attempts reached"
		}
	}

	s.health.Set(id, health)
}

func (s *Server) RecoverMachine(ctx context.Context, id string) {
	s.logger.Infof("Attempting to recover machine %s", id)

	// Get the current pod
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		if errors.IsNotFound(err) {
			s.logger.Errorf("Machine %s not found, cannot recover", id)
			return
		}
		s.logger.Errorf("Failed to get machine %s: %v", id, err)
		return
	}

	// Delete the pod
	err = s.client.DeletePod(ctx, pod.Name)
	if err != nil {
		s.logger.Errorf("Failed to delete pod %s: %v", pod.Name, err)
		return
	}

	// Wait for pod to be deleted
	for i := 0; i < 10; i++ {
		_, err = s.client.Find(ctx, id)
		if errors.IsNotFound(err) {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Recreate the pod
	image := pod.Spec.Containers[0].Image
	env := make(map[string]string)
	for _, envVar := range pod.Spec.Containers[0].Env {
		env[envVar.Name] = envVar.Value
	}

	_, err = s.client.Create(ctx, k8s.Machine{
		ID:    id,
		Image: image,
		Env:   env,
	})

	if err != nil {
		s.logger.Errorf("Failed to recreate machine %s: %v", id, err)
		return
	}

	s.logger.Infof("Successfully recovered machine %s", id)
}

func (s *Server) GetMachineHealth(ctx context.Context, id string) (MachineHealth, error) {
	health, exists := s.health.Get(id)
	if !exists {
		// If health record doesn't exist, check the machine now
		pod, err := s.client.Find(ctx, id)
		if err != nil {
			return MachineHealth{}, err
		}

		s.CheckMachineHealth(ctx, id, pod)
		health, _ = s.health.Get(id)
	}

	return health, nil
}
