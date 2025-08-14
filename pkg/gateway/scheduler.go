package gateway

import (
	"fmt"
	"sync"
	"time"

	"github.com/codecflow/fabric/pkg/types"
)

// Scheduler manages worker selection for container placement
type Scheduler struct {
	workers map[string]*types.Worker
	index   int
	mu      sync.RWMutex
}

// NewScheduler creates a new scheduler
func NewScheduler() *Scheduler {
	return &Scheduler{
		workers: make(map[string]*types.Worker),
		index:   0,
	}
}

// RegisterWorker adds a worker to the scheduler
func (s *Scheduler) RegisterWorker(worker *types.Worker) {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker.RegisteredAt = time.Now()
	worker.LastHeartbeat = time.Now()
	s.workers[worker.ID] = worker
}

// UnregisterWorker removes a worker from the scheduler
func (s *Scheduler) UnregisterWorker(workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.workers[workerID]; !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	delete(s.workers, workerID)
	return nil
}

// SelectWorker selects a worker using round-robin algorithm
func (s *Scheduler) SelectWorker() (*types.Worker, error) {
	return s.SelectWorkerWithGPU(0)
}

// SelectWorkerWithGPU selects a worker with at least the specified number of GPUs
func (s *Scheduler) SelectWorkerWithGPU(gpuCount int) (*types.Worker, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.workers) == 0 {
		return nil, fmt.Errorf("no workers available")
	}

	// Filter workers that have sufficient GPUs
	var eligibleWorkers []*types.Worker
	for _, worker := range s.workers {
		if worker.Capabilities.GPUCount >= gpuCount {
			eligibleWorkers = append(eligibleWorkers, worker)
		}
	}

	if len(eligibleWorkers) == 0 {
		if gpuCount > 0 {
			return nil, fmt.Errorf("no workers with %d GPUs available", gpuCount)
		}
		return nil, fmt.Errorf("no workers available")
	}

	// Round-robin selection from eligible workers
	worker := eligibleWorkers[s.index%len(eligibleWorkers)]
	s.index++

	return worker, nil
}

// GetWorker retrieves a worker by ID
func (s *Scheduler) GetWorker(workerID string) (*types.Worker, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	worker, exists := s.workers[workerID]
	if !exists {
		return nil, fmt.Errorf("worker not found: %s", workerID)
	}

	return worker, nil
}

// ListWorkers returns all registered workers
func (s *Scheduler) ListWorkers() []*types.Worker {
	s.mu.RLock()
	defer s.mu.RUnlock()

	workers := make([]*types.Worker, 0, len(s.workers))
	for _, worker := range s.workers {
		workers = append(workers, worker)
	}

	return workers
}

// UpdateHeartbeat updates the last heartbeat time for a worker
func (s *Scheduler) UpdateHeartbeat(workerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	worker, exists := s.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	worker.LastHeartbeat = time.Now()
	return nil
}
