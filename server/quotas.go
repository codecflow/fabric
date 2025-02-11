package server

import (
	"captain/k8s"
	"encoding/json"
	"net/http"
	"sync"
)

type ResourceQuota struct {
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Disk   string `json:"disk"`
}

type UserQuota struct {
	MaxMachines int           `json:"maxMachines"`
	Resources   ResourceQuota `json:"resources"`
	UserID      string        `json:"userId"`
}

type QuotaStore struct {
	quotas map[string]UserQuota
	mu     sync.RWMutex
}

func NewQuotaStore() *QuotaStore {
	store := &QuotaStore{
		quotas: make(map[string]UserQuota),
	}

	// Add default quota
	store.quotas["default"] = UserQuota{
		MaxMachines: 5,
		Resources: ResourceQuota{
			CPU:    "4",
			Memory: "8Gi",
			Disk:   "20Gi",
		},
		UserID: "default",
	}

	return store
}

func (s *QuotaStore) Get(userID string) (UserQuota, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	quota, ok := s.quotas[userID]
	return quota, ok
}

func (s *QuotaStore) Set(userID string, quota UserQuota) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.quotas[userID] = quota
}

func (s *QuotaStore) Delete(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.quotas, userID)
}

func (s *Server) GetQuota(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Missing userId parameter", http.StatusBadRequest)
		return
	}

	quota, ok := s.quotas.Get(userID)
	if !ok {
		// Return default quota if user-specific quota not found
		quota, _ = s.quotas.Get("default")
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(quota)
}

func (s *Server) SetQuota(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	var quota UserQuota
	if err := JSON(r, &quota); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if quota.UserID == "" {
		http.Error(w, "Missing userId in request", http.StatusBadRequest)
		return
	}

	s.quotas.Set(quota.UserID, quota)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) DeleteQuota(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodDelete) {
		return
	}

	userID := r.URL.Query().Get("userId")
	if userID == "" {
		http.Error(w, "Missing userId parameter", http.StatusBadRequest)
		return
	}

	s.quotas.Delete(userID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) ApplyResourceLimits(machine *k8s.Machine, userID string) {
	quota, ok := s.quotas.Get(userID)
	if !ok {
		quota, _ = s.quotas.Get("default")
	}

	// Apply resource limits from quota
	if machine.Resources == nil {
		machine.Resources = &k8s.Resources{}
	}

	// Only apply limits if not already set
	if machine.Resources.CPU == "" {
		machine.Resources.CPU = quota.Resources.CPU
	}
	if machine.Resources.Memory == "" {
		machine.Resources.Memory = quota.Resources.Memory
	}
	if machine.Resources.Disk == "" {
		machine.Resources.Disk = quota.Resources.Disk
	}
}
