package server

import (
	"captain/k8s"
	"fmt"
	"net/http"
)

type CreateRequest struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	RTMP string `json:"rtmp"`
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

	fmt.Printf("Received ID: %s, URL: %s, RTMP: %s\n", req.ID, req.URL, req.RTMP)

	ctx := r.Context()

	_, err := s.client.Create(ctx, k8s.Streamer{
		ID:   req.ID,
		URL:  req.URL,
		RTMP: req.RTMP,
	})

	if err != nil {
		http.Error(w, "Failed to create resource", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
