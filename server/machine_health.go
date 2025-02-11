package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) GetMachineHealthHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	health, err := s.GetMachineHealth(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) ListMachineHealthHandler(w http.ResponseWriter, r *http.Request) {
	healthList := s.health.List()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(healthList)
}
