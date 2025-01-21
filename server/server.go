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
	case strings.HasPrefix(r.URL.Path, "/exec"):
		s.Exec(w, r)
	case strings.HasPrefix(r.URL.Path, "/status"):
		s.Status(w, r)
	case strings.HasPrefix(r.URL.Path, "/logs"):
		s.Logs(w, r)
	case strings.HasPrefix(r.URL.Path, "/upload"):
		s.Upload(w, r)
	case strings.HasPrefix(r.URL.Path, "/download"):
		s.Download(w, r)
	case strings.HasPrefix(r.URL.Path, "/metrics") && strings.Contains(r.URL.Path, "/stream"):
		s.MetricsStream(w, r)
	case strings.HasPrefix(r.URL.Path, "/metrics"):
		s.Metrics(w, r)
	case strings.HasPrefix(r.URL.Path, "/snapshot/create"):
		s.CreateSnapshot(w, r)
	case strings.HasPrefix(r.URL.Path, "/snapshot/restore"):
		s.RestoreSnapshot(w, r)
	case strings.HasPrefix(r.URL.Path, "/snapshot/delete"):
		s.DeleteSnapshot(w, r)
	case strings.HasPrefix(r.URL.Path, "/snapshot/list"):
		s.ListSnapshots(w, r)
	default:
		http.NotFound(w, r)
	}
}
