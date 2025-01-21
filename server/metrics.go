package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/metrics/pkg/client/clientset/versioned"
)

type MetricsResponse struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	CPU         CPUMetrics         `json:"cpu"`
	Memory      MemoryMetrics      `json:"memory"`
	Containers  []ContainerMetrics `json:"containers"`
	CollectedAt time.Time          `json:"collectedAt"`
	Error       string             `json:"error,omitempty"`
}

type CPUMetrics struct {
	UsageNanoCores    int64   `json:"usageNanoCores"`
	UsageMilliCores   int64   `json:"usageMilliCores"`
	UsageCorePercent  float64 `json:"usageCorePercent"`
	LimitMilliCores   int64   `json:"limitMilliCores,omitempty"`
	RequestMilliCores int64   `json:"requestMilliCores,omitempty"`
}

type MemoryMetrics struct {
	UsageBytes        int64   `json:"usageBytes"`
	UsageMB           int64   `json:"usageMB"`
	LimitBytes        int64   `json:"limitBytes,omitempty"`
	LimitMB           int64   `json:"limitMB,omitempty"`
	RequestBytes      int64   `json:"requestBytes,omitempty"`
	RequestMB         int64   `json:"requestMB,omitempty"`
	UsagePercentLimit float64 `json:"usagePercentLimit,omitempty"`
}

type ContainerMetrics struct {
	Name   string        `json:"name"`
	CPU    CPUMetrics    `json:"cpu"`
	Memory MemoryMetrics `json:"memory"`
}

func (s *Server) Metrics(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	pod, err := s.client.Find(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to find machine: %v", err), http.StatusNotFound)
		return
	}

	// Create metrics client
	metricsClient, err := versioned.NewForConfig(s.client.GetConfig())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create metrics client: %v", err), http.StatusInternalServerError)
		return
	}

	// Get pod metrics
	podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(s.client.GetNamespace()).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		s.logger.WithFields(logrus.Fields{
			"machine_id": id,
			"error":      err,
		}).Error("Failed to get pod metrics")

		// Return a response with error but don't fail the request
		resp := MetricsResponse{
			ID:          id,
			Name:        pod.Name,
			CollectedAt: time.Now(),
			Error:       fmt.Sprintf("Failed to get metrics: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Calculate total CPU and memory usage
	var totalCPUUsage int64
	var totalMemoryUsage int64
	containers := make([]ContainerMetrics, 0, len(podMetrics.Containers))

	for _, container := range podMetrics.Containers {
		cpuQuantity := container.Usage.Cpu()
		memoryQuantity := container.Usage.Memory()

		cpuUsageNanoCores := cpuQuantity.MilliValue() * 1000000
		memoryUsageBytes := memoryQuantity.Value()

		totalCPUUsage += cpuUsageNanoCores
		totalMemoryUsage += memoryUsageBytes

		// Get container resource limits and requests
		var cpuLimit, cpuRequest, memoryLimit, memoryRequest int64
		for _, c := range pod.Spec.Containers {
			if c.Name == container.Name {
				if c.Resources.Limits != nil {
					if cpu, ok := c.Resources.Limits["cpu"]; ok {
						cpuLimit = cpu.MilliValue()
					}
					if memory, ok := c.Resources.Limits["memory"]; ok {
						memoryLimit = memory.Value()
					}
				}
				if c.Resources.Requests != nil {
					if cpu, ok := c.Resources.Requests["cpu"]; ok {
						cpuRequest = cpu.MilliValue()
					}
					if memory, ok := c.Resources.Requests["memory"]; ok {
						memoryRequest = memory.Value()
					}
				}
				break
			}
		}

		// Calculate CPU usage percentage
		cpuUsagePercent := 0.0
		if cpuLimit > 0 {
			cpuUsagePercent = float64(cpuUsageNanoCores) / float64(cpuLimit*1000000) * 100
		}

		// Calculate memory usage percentage
		memoryUsagePercent := 0.0
		if memoryLimit > 0 {
			memoryUsagePercent = float64(memoryUsageBytes) / float64(memoryLimit) * 100
		}

		containerMetrics := ContainerMetrics{
			Name: container.Name,
			CPU: CPUMetrics{
				UsageNanoCores:    cpuUsageNanoCores,
				UsageMilliCores:   cpuUsageNanoCores / 1000000,
				UsageCorePercent:  cpuUsagePercent,
				LimitMilliCores:   cpuLimit,
				RequestMilliCores: cpuRequest,
			},
			Memory: MemoryMetrics{
				UsageBytes:        memoryUsageBytes,
				UsageMB:           memoryUsageBytes / (1024 * 1024),
				LimitBytes:        memoryLimit,
				LimitMB:           memoryLimit / (1024 * 1024),
				RequestBytes:      memoryRequest,
				RequestMB:         memoryRequest / (1024 * 1024),
				UsagePercentLimit: memoryUsagePercent,
			},
		}
		containers = append(containers, containerMetrics)
	}

	// Calculate total CPU usage percentage
	totalCPULimit := int64(0)
	totalCPURequest := int64(0)
	totalMemoryLimit := int64(0)
	totalMemoryRequest := int64(0)

	for _, c := range pod.Spec.Containers {
		if c.Resources.Limits != nil {
			if cpu, ok := c.Resources.Limits["cpu"]; ok {
				totalCPULimit += cpu.MilliValue()
			}
			if memory, ok := c.Resources.Limits["memory"]; ok {
				totalMemoryLimit += memory.Value()
			}
		}
		if c.Resources.Requests != nil {
			if cpu, ok := c.Resources.Requests["cpu"]; ok {
				totalCPURequest += cpu.MilliValue()
			}
			if memory, ok := c.Resources.Requests["memory"]; ok {
				totalMemoryRequest += memory.Value()
			}
		}
	}

	totalCPUUsagePercent := 0.0
	if totalCPULimit > 0 {
		totalCPUUsagePercent = float64(totalCPUUsage) / float64(totalCPULimit*1000000) * 100
	}

	totalMemoryUsagePercent := 0.0
	if totalMemoryLimit > 0 {
		totalMemoryUsagePercent = float64(totalMemoryUsage) / float64(totalMemoryLimit) * 100
	}

	// Create response
	resp := MetricsResponse{
		ID:   id,
		Name: pod.Name,
		CPU: CPUMetrics{
			UsageNanoCores:    totalCPUUsage,
			UsageMilliCores:   totalCPUUsage / 1000000,
			UsageCorePercent:  totalCPUUsagePercent,
			LimitMilliCores:   totalCPULimit,
			RequestMilliCores: totalCPURequest,
		},
		Memory: MemoryMetrics{
			UsageBytes:        totalMemoryUsage,
			UsageMB:           totalMemoryUsage / (1024 * 1024),
			LimitBytes:        totalMemoryLimit,
			LimitMB:           totalMemoryLimit / (1024 * 1024),
			RequestBytes:      totalMemoryRequest,
			RequestMB:         totalMemoryRequest / (1024 * 1024),
			UsagePercentLimit: totalMemoryUsagePercent,
		},
		Containers:  containers,
		CollectedAt: podMetrics.Timestamp.Time,
	}

	// Log metrics collection
	s.logger.WithFields(logrus.Fields{
		"machine_id":      id,
		"cpu_usage_mc":    resp.CPU.UsageMilliCores,
		"memory_usage_mb": resp.Memory.UsageMB,
	}).Info("Metrics collected")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// MetricsStream handles streaming metrics for a machine
func (s *Server) MetricsStream(w http.ResponseWriter, r *http.Request) {
	if !Method(w, r, http.MethodGet) {
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing machine ID", http.StatusBadRequest)
		return
	}

	intervalStr := r.URL.Query().Get("interval")
	interval := 5 // Default to 5 seconds
	if intervalStr != "" {
		i, err := strconv.Atoi(intervalStr)
		if err == nil && i > 0 {
			interval = i
		}
	}

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Create a channel to signal when the client disconnects
	ctx := r.Context()
	done := ctx.Done()

	// Stream metrics at the specified interval
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Get pod
			pod, err := s.client.Find(ctx, id)
			if err != nil {
				fmt.Fprintf(w, "data: {\"error\": \"Failed to find machine: %v\"}\n\n", err)
				flusher.Flush()
				return
			}

			// Create metrics client
			metricsClient, err := versioned.NewForConfig(s.client.GetConfig())
			if err != nil {
				fmt.Fprintf(w, "data: {\"error\": \"Failed to create metrics client: %v\"}\n\n", err)
				flusher.Flush()
				return
			}

			// Get pod metrics
			podMetrics, err := metricsClient.MetricsV1beta1().PodMetricses(s.client.GetNamespace()).Get(ctx, pod.Name, metav1.GetOptions{})
			if err != nil {
				fmt.Fprintf(w, "data: {\"error\": \"Failed to get metrics: %v\"}\n\n", err)
				flusher.Flush()
				continue
			}

			// Calculate metrics (similar to the non-streaming version)
			var totalCPUUsage int64
			var totalMemoryUsage int64
			containers := make([]ContainerMetrics, 0, len(podMetrics.Containers))

			for _, container := range podMetrics.Containers {
				cpuQuantity := container.Usage.Cpu()
				memoryQuantity := container.Usage.Memory()

				cpuUsageNanoCores := cpuQuantity.MilliValue() * 1000000
				memoryUsageBytes := memoryQuantity.Value()

				totalCPUUsage += cpuUsageNanoCores
				totalMemoryUsage += memoryUsageBytes

				// Get container resource limits and requests
				var cpuLimit, cpuRequest, memoryLimit, memoryRequest int64
				for _, c := range pod.Spec.Containers {
					if c.Name == container.Name {
						if c.Resources.Limits != nil {
							if cpu, ok := c.Resources.Limits["cpu"]; ok {
								cpuLimit = cpu.MilliValue()
							}
							if memory, ok := c.Resources.Limits["memory"]; ok {
								memoryLimit = memory.Value()
							}
						}
						if c.Resources.Requests != nil {
							if cpu, ok := c.Resources.Requests["cpu"]; ok {
								cpuRequest = cpu.MilliValue()
							}
							if memory, ok := c.Resources.Requests["memory"]; ok {
								memoryRequest = memory.Value()
							}
						}
						break
					}
				}

				// Calculate CPU usage percentage
				cpuUsagePercent := 0.0
				if cpuLimit > 0 {
					cpuUsagePercent = float64(cpuUsageNanoCores) / float64(cpuLimit*1000000) * 100
				}

				// Calculate memory usage percentage
				memoryUsagePercent := 0.0
				if memoryLimit > 0 {
					memoryUsagePercent = float64(memoryUsageBytes) / float64(memoryLimit) * 100
				}

				containerMetrics := ContainerMetrics{
					Name: container.Name,
					CPU: CPUMetrics{
						UsageNanoCores:    cpuUsageNanoCores,
						UsageMilliCores:   cpuUsageNanoCores / 1000000,
						UsageCorePercent:  cpuUsagePercent,
						LimitMilliCores:   cpuLimit,
						RequestMilliCores: cpuRequest,
					},
					Memory: MemoryMetrics{
						UsageBytes:        memoryUsageBytes,
						UsageMB:           memoryUsageBytes / (1024 * 1024),
						LimitBytes:        memoryLimit,
						LimitMB:           memoryLimit / (1024 * 1024),
						RequestBytes:      memoryRequest,
						RequestMB:         memoryRequest / (1024 * 1024),
						UsagePercentLimit: memoryUsagePercent,
					},
				}
				containers = append(containers, containerMetrics)
			}

			// Calculate total CPU usage percentage
			totalCPULimit := int64(0)
			totalCPURequest := int64(0)
			totalMemoryLimit := int64(0)
			totalMemoryRequest := int64(0)

			for _, c := range pod.Spec.Containers {
				if c.Resources.Limits != nil {
					if cpu, ok := c.Resources.Limits["cpu"]; ok {
						totalCPULimit += cpu.MilliValue()
					}
					if memory, ok := c.Resources.Limits["memory"]; ok {
						totalMemoryLimit += memory.Value()
					}
				}
				if c.Resources.Requests != nil {
					if cpu, ok := c.Resources.Requests["cpu"]; ok {
						totalCPURequest += cpu.MilliValue()
					}
					if memory, ok := c.Resources.Requests["memory"]; ok {
						totalMemoryRequest += memory.Value()
					}
				}
			}

			totalCPUUsagePercent := 0.0
			if totalCPULimit > 0 {
				totalCPUUsagePercent = float64(totalCPUUsage) / float64(totalCPULimit*1000000) * 100
			}

			totalMemoryUsagePercent := 0.0
			if totalMemoryLimit > 0 {
				totalMemoryUsagePercent = float64(totalMemoryUsage) / float64(totalMemoryLimit) * 100
			}

			// Create response
			resp := MetricsResponse{
				ID:   id,
				Name: pod.Name,
				CPU: CPUMetrics{
					UsageNanoCores:    totalCPUUsage,
					UsageMilliCores:   totalCPUUsage / 1000000,
					UsageCorePercent:  totalCPUUsagePercent,
					LimitMilliCores:   totalCPULimit,
					RequestMilliCores: totalCPURequest,
				},
				Memory: MemoryMetrics{
					UsageBytes:        totalMemoryUsage,
					UsageMB:           totalMemoryUsage / (1024 * 1024),
					LimitBytes:        totalMemoryLimit,
					LimitMB:           totalMemoryLimit / (1024 * 1024),
					RequestBytes:      totalMemoryRequest,
					RequestMB:         totalMemoryRequest / (1024 * 1024),
					UsagePercentLimit: totalMemoryUsagePercent,
				},
				Containers:  containers,
				CollectedAt: podMetrics.Timestamp.Time,
			}

			// Serialize the response
			data, err := json.Marshal(resp)
			if err != nil {
				fmt.Fprintf(w, "data: {\"error\": \"Failed to serialize metrics: %v\"}\n\n", err)
				flusher.Flush()
				continue
			}

			// Send the metrics
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

		case <-done:
			// Client disconnected
			return
		}
	}
}
