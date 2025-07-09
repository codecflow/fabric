package metrics

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/codecflow/fabric/shuttle/internal/config"
)

// Server manages metrics collection and exposure
type Server struct {
	config *config.MetricsConfig
	server *http.Server
}

// New creates a new metrics server
func New(cfg *config.MetricsConfig) (*Server, error) {
	return &Server{
		config: cfg,
	}, nil
}

// Start starts the metrics server
func (s *Server) Start(ctx context.Context) error {
	if !s.config.Enabled {
		return nil
	}

	mux := http.NewServeMux()

	// Prometheus metrics endpoint
	mux.HandleFunc(s.config.Path, s.handleMetrics)

	// Health endpoint
	mux.HandleFunc("/health", s.handleHealth)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%d", s.config.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	go func() {
		log.Printf("Starting metrics server on port %d", s.config.Port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Metrics server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the metrics server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleMetrics handles the metrics endpoint
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// In a real implementation, this would expose Prometheus metrics
	// For now, return basic metrics
	metrics := `# HELP shuttle_workloads_total Total number of workloads
# TYPE shuttle_workloads_total counter
shuttle_workloads_total 0

# HELP shuttle_uptime_seconds Shuttle uptime in seconds
# TYPE shuttle_uptime_seconds gauge
shuttle_uptime_seconds 0

# HELP shuttle_memory_usage_bytes Memory usage in bytes
# TYPE shuttle_memory_usage_bytes gauge
shuttle_memory_usage_bytes 0

# HELP shuttle_cpu_usage_percent CPU usage percentage
# TYPE shuttle_cpu_usage_percent gauge
shuttle_cpu_usage_percent 0
`

	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(metrics))
}

// handleHealth handles the health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
}

// RecordWorkloadStart records a workload start event
func (s *Server) RecordWorkloadStart(workloadID string) {
	// In a real implementation, increment workload counter
	log.Printf("Metrics: Workload started - %s", workloadID)
}

// RecordWorkloadStop records a workload stop event
func (s *Server) RecordWorkloadStop(workloadID string) {
	// In a real implementation, decrement workload counter
	log.Printf("Metrics: Workload stopped - %s", workloadID)
}

// RecordResourceUsage records resource usage metrics
func (s *Server) RecordResourceUsage(cpu, memory float64) {
	// In a real implementation, update resource usage gauges
	log.Printf("Metrics: Resource usage - CPU: %.2f%%, Memory: %.2f%%", cpu, memory)
}

// SendToOpenMeter sends usage data to OpenMeter
func (s *Server) SendToOpenMeter(ctx context.Context, usage *UsageData) error {
	if !s.config.OpenMeter.Enabled {
		return nil
	}

	// In a real implementation, send HTTP request to OpenMeter API
	log.Printf("Sending usage data to OpenMeter: %+v", usage)
	return nil
}

// UsageData represents usage data for OpenMeter
type UsageData struct {
	WorkloadID string    `json:"workload_id"`
	NodeID     string    `json:"node_id"`
	CPU        float64   `json:"cpu"`
	Memory     float64   `json:"memory"`
	Duration   int64     `json:"duration"`
	Timestamp  time.Time `json:"timestamp"`
}
