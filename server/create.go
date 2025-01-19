package server

import (
	"captain/k8s"
	"fmt"
	"net/http"
)

type CreateRequest struct {
	ID    string            `json:"id"`
	Image string            `json:"image"`
	Tools []string          `json:"tools"`
	Env   map[string]string `json:"env"`
}

func (s *Server) Create(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodPost) {
		return
	}

	if r.Header.Get("Content-Type") != "application/json" {
		http.Error(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return
	}

	var req CreateRequest

	if err := JSON(r, &req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	fmt.Printf("Received ID: %s, Image: %s\n", req.ID, req.Image)

	ctx := r.Context()

	_, err := s.client.Create(ctx, k8s.Machine{
		ID:    req.ID,
		Image: req.Image,
		Tools: req.Tools,
		Env:   req.Env,
	})

	if err != nil {
		http.Error(w, "Failed to create resource", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
