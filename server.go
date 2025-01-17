package main

import (
	"captain/k8s"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

type Server struct {
	client *k8s.Client
}

func NewServer() (*Server, error) {
	client, err := k8s.NewClient("streamer", "default", "entrypoint", "ghcr.io/codecflow/captain:1.0.0")
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &Server{client: client}, nil
}

type CreateRequest struct {
	ID   string `json:"id"`
	URL  string `json:"url"`
	RTMP string `json:"rtmp"`
}

func JSON[T any](r *http.Request, dest *T) error {
	if r.Body == nil {
		return errors.New("request body is empty")
	}
	defer r.Body.Close()

	if contentType := r.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		return errors.New("Content-Type must be application/json")
	}

	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(dest)
}

func Method(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	return true
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

func (s *Server) Delete(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodDelete) {
		return
	}

	id := r.URL.Query().Get("id")

	if id == "" {
		http.Error(w, "Missing id parameter", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	err := s.client.Delete(ctx, id)

	if err != nil {
		http.Error(w, "Failed to delete resource", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/create":
		s.Create(w, r)
	case "/delete":
		s.Delete(w, r)
	default:
		http.NotFound(w, r)
	}
}
