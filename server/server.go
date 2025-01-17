package server

import (
	"captain/k8s"
	"fmt"
	"net/http"
	"strings"
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
	switch {
	case strings.HasPrefix(r.URL.Path, "/create"):
		s.Create(w, r)
	case strings.HasPrefix(r.URL.Path, "/delete"):
		s.Delete(w, r)
	case strings.HasPrefix(r.URL.Path, "/connect"):
		s.Connect(w, r)
	default:
		http.NotFound(w, r)
	}
}
