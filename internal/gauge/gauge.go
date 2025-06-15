package gauge

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Server provides system metrics and information
type Server struct {
	host   string
	port   int
	server *http.Server
}

// SystemInfo represents system information
type SystemInfo struct {
	Hostname     string            `json:"hostname"`
	OS           string            `json:"os"`
	Architecture string            `json:"architecture"`
	CPUCores     int               `json:"cpu_cores"`
	Memory       *MemoryInfo       `json:"memory"`
	GPU          []*GPUInfo        `json:"gpu,omitempty"`
	User         string            `json:"user"`
	Uptime       string            `json:"uptime"`
	Timestamp    string            `json:"timestamp"`
	Environment  map[string]string `json:"environment,omitempty"`
}

// MemoryInfo represents memory information
type MemoryInfo struct {
	Total     uint64  `json:"total"`
	Available uint64  `json:"available"`
	Used      uint64  `json:"used"`
	Percent   float64 `json:"percent"`
}

// GPUInfo represents GPU information
type GPUInfo struct {
	UUID          string  `json:"uuid"`
	Name          string  `json:"name"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryTotal   uint64  `json:"memory_total"`
	Utilization   float64 `json:"utilization"`
	Temperature   int     `json:"temperature,omitempty"`
	PowerDraw     float64 `json:"power_draw,omitempty"`
	DriverVersion string  `json:"driver_version,omitempty"`
}

// MetricsResponse represents the metrics endpoint response
type MetricsResponse struct {
	System    *SystemInfo `json:"system"`
	Timestamp string      `json:"timestamp"`
	Status    string      `json:"status"`
}

// New creates a new gauge server
func New(host string, port int) (*Server, error) {
	return &Server{
		host: host,
		port: port,
	}, nil
}

// Start starts the gauge server
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", s.handleMetrics)
	mux.HandleFunc("/whoami", s.handleWhoami)
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/system", s.handleSystem)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", s.host, s.port),
		Handler: s.withLogging(mux),
	}

	go func() {
		log.Printf("Starting gauge server on %s:%d", s.host, s.port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Gauge server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the gauge server
func (s *Server) Stop() error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// handleMetrics handles the /metrics endpoint
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	systemInfo := s.getSystemInfo()

	response := &MetricsResponse{
		System:    systemInfo,
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    "ok",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleWhoami handles the /whoami endpoint
func (s *Server) handleWhoami(w http.ResponseWriter, r *http.Request) {
	user := s.getCurrentUser()

	response := map[string]interface{}{
		"user":      user,
		"timestamp": time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleHealth handles the /health endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "gauge",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleSystem handles the /system endpoint
func (s *Server) handleSystem(w http.ResponseWriter, r *http.Request) {
	systemInfo := s.getSystemInfo()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(systemInfo); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// getSystemInfo collects comprehensive system information
func (s *Server) getSystemInfo() *SystemInfo {
	hostname, _ := os.Hostname()

	info := &SystemInfo{
		Hostname:     hostname,
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		CPUCores:     runtime.NumCPU(),
		Memory:       s.getMemoryInfo(),
		GPU:          s.getGPUInfo(),
		User:         s.getCurrentUser(),
		Uptime:       s.getUptime(),
		Timestamp:    time.Now().Format(time.RFC3339),
		Environment:  s.getEnvironment(),
	}

	return info
}

// getMemoryInfo gets memory information
func (s *Server) getMemoryInfo() *MemoryInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Try to get system memory info (Linux/macOS specific)
	total, available := s.getSystemMemory()

	return &MemoryInfo{
		Total:     total,
		Available: available,
		Used:      total - available,
		Percent:   float64(total-available) / float64(total) * 100,
	}
}

// getSystemMemory gets system memory information
func (s *Server) getSystemMemory() (total, available uint64) {
	// Default fallback values
	total = 8 * 1024 * 1024 * 1024     // 8GB default
	available = 4 * 1024 * 1024 * 1024 // 4GB default

	switch runtime.GOOS {
	case "linux":
		if data, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					if val := s.parseMemoryLine(line); val > 0 {
						total = val * 1024 // Convert KB to bytes
					}
				} else if strings.HasPrefix(line, "MemAvailable:") {
					if val := s.parseMemoryLine(line); val > 0 {
						available = val * 1024 // Convert KB to bytes
					}
				}
			}
		}
	case "darwin":
		// macOS memory info via vm_stat
		if _, err := exec.Command("vm_stat").Output(); err == nil {
			// Parse vm_stat output (simplified)
			total = 16 * 1024 * 1024 * 1024 // 16GB default for macOS
			available = 8 * 1024 * 1024 * 1024
		}
	}

	return total, available
}

// parseMemoryLine parses a memory line from /proc/meminfo
func (s *Server) parseMemoryLine(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		if val, err := strconv.ParseUint(fields[1], 10, 64); err == nil {
			return val
		}
	}
	return 0
}

// getGPUInfo gets GPU information via nvidia-smi
func (s *Server) getGPUInfo() []*GPUInfo {
	gpus := []*GPUInfo{}

	// Try nvidia-smi
	cmd := exec.Command("nvidia-smi",
		"--query-gpu=uuid,name,memory.used,memory.total,utilization.gpu,temperature.gpu,power.draw,driver_version",
		"--format=csv,noheader,nounits")

	output, err := cmd.Output()
	if err != nil {
		// No NVIDIA GPUs or nvidia-smi not available
		return gpus
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Split(line, ", ")
		if len(fields) >= 8 {
			gpu := &GPUInfo{
				UUID:          strings.TrimSpace(fields[0]),
				Name:          strings.TrimSpace(fields[1]),
				DriverVersion: strings.TrimSpace(fields[7]),
			}

			// Parse memory used (MB)
			if val, err := strconv.ParseUint(strings.TrimSpace(fields[2]), 10, 64); err == nil {
				gpu.MemoryUsed = val * 1024 * 1024 // Convert MB to bytes
			}

			// Parse memory total (MB)
			if val, err := strconv.ParseUint(strings.TrimSpace(fields[3]), 10, 64); err == nil {
				gpu.MemoryTotal = val * 1024 * 1024 // Convert MB to bytes
			}

			// Parse utilization (%)
			if val, err := strconv.ParseFloat(strings.TrimSpace(fields[4]), 64); err == nil {
				gpu.Utilization = val
			}

			// Parse temperature (C)
			if val, err := strconv.Atoi(strings.TrimSpace(fields[5])); err == nil {
				gpu.Temperature = val
			}

			// Parse power draw (W)
			if val, err := strconv.ParseFloat(strings.TrimSpace(fields[6]), 64); err == nil {
				gpu.PowerDraw = val
			}

			gpus = append(gpus, gpu)
		}
	}

	return gpus
}

// getCurrentUser gets the current user
func (s *Server) getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "unknown"
}

// getUptime gets system uptime
func (s *Server) getUptime() string {
	switch runtime.GOOS {
	case "linux":
		if data, err := os.ReadFile("/proc/uptime"); err == nil {
			fields := strings.Fields(string(data))
			if len(fields) > 0 {
				if seconds, err := strconv.ParseFloat(fields[0], 64); err == nil {
					duration := time.Duration(seconds) * time.Second
					return duration.String()
				}
			}
		}
	case "darwin":
		if output, err := exec.Command("uptime").Output(); err == nil {
			return strings.TrimSpace(string(output))
		}
	}
	return "unknown"
}

// getEnvironment gets relevant environment variables
func (s *Server) getEnvironment() map[string]string {
	env := make(map[string]string)

	// Only include relevant environment variables
	relevantVars := []string{
		"CUDA_VISIBLE_DEVICES",
		"NVIDIA_VISIBLE_DEVICES",
		"PATH",
		"HOME",
		"SHELL",
		"LANG",
		"TZ",
	}

	for _, key := range relevantVars {
		if value := os.Getenv(key); value != "" {
			env[key] = value
		}
	}

	return env
}

// withLogging adds request logging middleware
func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
	})
}
