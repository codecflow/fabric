package server

import (
	"captain/k8s"
	"fmt"
	"net/http"
)

type Server struct {
	client *k8s.Client
}

func NewServer(prefix, namespace, entrypoint, image string) (*Server, error) {
	client, err := k8s.NewClient(prefix, namespace, entrypoint, image)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	return &Server{client: client}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/create":
		s.Create(w, r)
	case "/delete":
		s.Delete(w, r)
	case "/connect":
		s.Connect(w, r)
	default:
		http.NotFound(w, r)
	}
}
