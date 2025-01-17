package server

import (
	"captain/k8s"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type Server struct {
	client *k8s.Client
	logger *logrus.Logger
}

func NewServer(prefix, namespace, entrypoint, image string) (*Server, error) {
	client, err := k8s.NewClient(prefix, namespace, entrypoint, image)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	logger.SetLevel(logrus.InfoLevel)

	return &Server{client: client, logger: logger}, nil
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
