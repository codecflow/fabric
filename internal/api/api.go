package api

import (
	"crypto/rand"
	"encoding/hex"
	"fabric/internal/state"
	"fabric/internal/types"
	"time"

	"github.com/gin-gonic/gin"
)

// generateID creates a random ID for workloads
func generateID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func SetupRoutes(appState *state.State) *gin.Engine {
	r := gin.New()

	// Middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "weaver",
		})
	})

	// API v1 routes
	v1 := r.Group("/v1")
	{
		// Workload management
		v1.POST("/workloads", createWorkload(appState))
		v1.GET("/workloads/:id", getWorkload(appState))
		v1.DELETE("/workloads/:id", deleteWorkload(appState))
		v1.GET("/workloads", listWorkloads(appState))

		// Provider information
		v1.GET("/providers", listProviders(appState))
		v1.GET("/providers/:name/regions", listProviderRegions(appState))
		v1.GET("/providers/:name/machine-types", listProviderMachineTypes(appState))

		// Scheduler endpoints
		v1.GET("/scheduler/status", getSchedulerStatus(appState))
		v1.POST("/scheduler/schedule", scheduleWorkload(appState))
		v1.GET("/scheduler/recommendations", getRecommendations(appState))
		v1.GET("/scheduler/stats", getSchedulerStats(appState))
	}

	return r
}

func createWorkload(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name      string            `json:"name" binding:"required"`
			Namespace string            `json:"namespace" binding:"required"`
			Image     string            `json:"image" binding:"required"`
			Port      int32             `json:"port"`
			Env       map[string]string `json:"env"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		now := time.Now()

		// Create workload with proper structure
		workload := &types.Workload{
			ID:        generateID(),
			Name:      req.Name,
			Namespace: req.Namespace,
			Spec: types.WorkloadSpec{
				Image: req.Image,
				Env:   req.Env,
			},
			Status: types.WorkloadStatus{
				Phase: types.WorkloadPhasePending,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Add port if specified
		if req.Port > 0 {
			workload.Spec.Ports = []types.Port{
				{
					ContainerPort: req.Port,
					Protocol:      "TCP",
				},
			}
		}

		// Store workload in repository
		if appState.Repository != nil {
			if err := appState.Repository.CreateWorkload(c.Request.Context(), workload); err != nil {
				c.JSON(500, gin.H{"error": "failed to store workload: " + err.Error()})
				return
			}
		}

		// Schedule workload if scheduler is available
		if appState.Scheduler != nil {
			placement, err := appState.Scheduler.Schedule(c.Request.Context(), workload)
			if err != nil {
				c.JSON(500, gin.H{"error": "failed to schedule workload: " + err.Error()})
				return
			}

			// Update workload status with placement information
			workload.Status.Provider = placement.Provider
			if placement.Placement != nil {
				workload.Status.NodeID = placement.Placement.NodeID
			}
			workload.Status.Phase = types.WorkloadPhaseScheduled

			// Update workload in repository with placement info
			if appState.Repository != nil {
				if err := appState.Repository.UpdateWorkload(c.Request.Context(), workload); err != nil {
					// Log warning but don't fail the request
					// In production, you might want to handle this differently
				}
			}
		}

		// Add proxy route if proxy is enabled and workload has a port
		if appState.Proxy != nil && req.Port > 0 {
			// Simulate target URL (in real implementation, this would come from the provider)
			targetURL := "http://localhost:8080" // This would be the actual workload endpoint
			if err := appState.Proxy.AddRoute(workload, targetURL); err != nil {
				c.JSON(500, gin.H{"error": "failed to add proxy route: " + err.Error()})
				return
			}
		}

		c.JSON(201, gin.H{
			"id":        workload.ID,
			"name":      workload.Name,
			"namespace": workload.Namespace,
			"status":    workload.Status.Phase,
		})
	}
}

func getWorkload(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(400, gin.H{"error": "workload id is required"})
			return
		}

		if appState.Repository == nil {
			c.JSON(503, gin.H{"error": "repository not configured"})
			return
		}

		workload, err := appState.Repository.GetWorkload(c.Request.Context(), id)
		if err != nil {
			c.JSON(404, gin.H{"error": "workload not found"})
			return
		}

		c.JSON(200, gin.H{
			"id":         workload.ID,
			"name":       workload.Name,
			"namespace":  workload.Namespace,
			"image":      workload.Spec.Image,
			"status":     workload.Status.Phase,
			"provider":   workload.Status.Provider,
			"node_id":    workload.Status.NodeID,
			"created_at": workload.CreatedAt,
			"updated_at": workload.UpdatedAt,
		})
	}
}

func deleteWorkload(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(400, gin.H{"error": "workload id is required"})
			return
		}

		if appState.Repository == nil {
			c.JSON(503, gin.H{"error": "repository not configured"})
			return
		}

		// Get workload first to remove proxy route
		workload, err := appState.Repository.GetWorkload(c.Request.Context(), id)
		if err != nil {
			c.JSON(404, gin.H{"error": "workload not found"})
			return
		}

		// Remove proxy route if proxy is enabled
		if appState.Proxy != nil {
			appState.Proxy.RemoveRoute(workload)
		}

		// Delete workload from repository
		if err := appState.Repository.DeleteWorkload(c.Request.Context(), id); err != nil {
			c.JSON(500, gin.H{"error": "failed to delete workload: " + err.Error()})
			return
		}

		c.JSON(200, gin.H{"message": "workload deleted successfully"})
	}
}

func listWorkloads(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if appState.Repository == nil {
			c.JSON(503, gin.H{"error": "repository not configured"})
			return
		}

		namespace := c.Query("namespace")
		labelSelectorStr := c.Query("labelSelector")

		// Parse label selector string into map (simplified parsing)
		var labelSelector map[string]string
		if labelSelectorStr != "" {
			labelSelector = make(map[string]string)
			// For now, just pass empty map - in production you'd parse "key=value,key2=value2" format
		}

		workloads, err := appState.Repository.ListWorkloads(c.Request.Context(), namespace, labelSelector)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to list workloads: " + err.Error()})
			return
		}

		var result []gin.H
		for _, workload := range workloads {
			result = append(result, gin.H{
				"id":         workload.ID,
				"name":       workload.Name,
				"namespace":  workload.Namespace,
				"image":      workload.Spec.Image,
				"status":     workload.Status.Phase,
				"provider":   workload.Status.Provider,
				"node_id":    workload.Status.NodeID,
				"created_at": workload.CreatedAt,
				"updated_at": workload.UpdatedAt,
			})
		}

		c.JSON(200, gin.H{
			"workloads": result,
			"total":     len(result),
		})
	}
}

func listProviders(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		providers := make([]string, 0, len(appState.Providers))
		for name := range appState.Providers {
			providers = append(providers, name)
		}
		c.JSON(200, gin.H{"providers": providers})
	}
}

func listProviderRegions(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerName := c.Param("name")
		provider, exists := appState.Providers[providerName]
		if !exists {
			c.JSON(404, gin.H{"error": "provider not found"})
			return
		}

		// Get available resources which includes region information
		resources, err := provider.GetAvailableResources(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to get provider resources: " + err.Error()})
			return
		}

		// Extract regions from the resource availability
		var regions []gin.H
		for _, region := range resources.Regions {
			regions = append(regions, gin.H{
				"name":         region.Name,
				"display_name": region.DisplayName,
				"available":    region.Available,
			})
		}

		c.JSON(200, gin.H{
			"provider": providerName,
			"regions":  regions,
		})
	}
}

func listProviderMachineTypes(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerName := c.Param("name")
		provider, exists := appState.Providers[providerName]
		if !exists {
			c.JSON(404, gin.H{"error": "provider not found"})
			return
		}

		// Get pricing information which includes machine type details
		pricing, err := provider.GetPricing(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to get provider pricing: " + err.Error()})
			return
		}

		// Get available resources for capacity information
		resources, err := provider.GetAvailableResources(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to get provider resources: " + err.Error()})
			return
		}

		// Generate machine types based on available resources and pricing
		var machineTypes []gin.H

		// Basic CPU/Memory configurations
		cpuConfigs := []struct {
			name   string
			cpu    string
			memory string
		}{
			{"small", "1", "2Gi"},
			{"medium", "2", "4Gi"},
			{"large", "4", "8Gi"},
			{"xlarge", "8", "16Gi"},
			{"2xlarge", "16", "32Gi"},
		}

		for _, config := range cpuConfigs {
			machineType := gin.H{
				"name":           config.name,
				"cpu":            config.cpu,
				"memory":         config.memory,
				"price_per_hour": pricing.CPU.Amount + pricing.Memory.Amount,
				"currency":       pricing.Currency,
				"available":      true,
			}
			machineTypes = append(machineTypes, machineType)
		}

		// Add GPU machine types if GPUs are available
		if len(resources.GPU.Types) > 0 {
			for gpuType, gpuInfo := range resources.GPU.Types {
				if gpuInfo.Available > 0 {
					machineType := gin.H{
						"name":           "gpu-" + gpuType,
						"cpu":            "8",
						"memory":         "32Gi",
						"gpu":            gpuType,
						"gpu_count":      1,
						"price_per_hour": gpuInfo.PricePerHour,
						"currency":       pricing.Currency,
						"available":      gpuInfo.Available > 0,
					}
					machineTypes = append(machineTypes, machineType)
				}
			}
		}

		c.JSON(200, gin.H{
			"provider":      providerName,
			"machine_types": machineTypes,
		})
	}
}

func getSchedulerStatus(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := gin.H{
			"status":          "running",
			"providers_count": len(appState.Providers),
		}

		if appState.Scheduler != nil {
			if err := appState.Scheduler.HealthCheck(c.Request.Context()); err != nil {
				status["scheduler_status"] = "unhealthy"
				status["scheduler_error"] = err.Error()
			} else {
				status["scheduler_status"] = "healthy"
			}
		} else {
			status["scheduler_status"] = "not_configured"
		}

		c.JSON(200, status)
	}
}

func scheduleWorkload(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if appState.Scheduler == nil {
			c.JSON(503, gin.H{"error": "scheduler not configured"})
			return
		}

		var req struct {
			Name      string            `json:"name" binding:"required"`
			Namespace string            `json:"namespace" binding:"required"`
			Image     string            `json:"image" binding:"required"`
			CPU       string            `json:"cpu"`
			Memory    string            `json:"memory"`
			GPU       string            `json:"gpu"`
			Port      int32             `json:"port"`
			Env       map[string]string `json:"env"`
			Region    string            `json:"region"`
			Provider  string            `json:"provider"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}

		now := time.Now()

		// Create workload for scheduling
		workload := &types.Workload{
			ID:        generateID(),
			Name:      req.Name,
			Namespace: req.Namespace,
			Spec: types.WorkloadSpec{
				Image: req.Image,
				Env:   req.Env,
			},
			Status: types.WorkloadStatus{
				Phase: types.WorkloadPhasePending,
			},
			CreatedAt: now,
			UpdatedAt: now,
		}

		// Set resource requirements
		if req.CPU != "" || req.Memory != "" || req.GPU != "" {
			workload.Spec.Resources = types.ResourceRequests{
				CPU:    req.CPU,
				Memory: req.Memory,
				GPU:    req.GPU,
			}
		}

		// Add port if specified
		if req.Port > 0 {
			workload.Spec.Ports = []types.Port{
				{
					ContainerPort: req.Port,
					Protocol:      "TCP",
				},
			}
		}

		// Set placement preferences
		if req.Region != "" || req.Provider != "" {
			workload.Spec.Placement = types.PlacementSpec{
				Region:   req.Region,
				Provider: req.Provider,
			}
		}

		// Schedule the workload
		placement, err := appState.Scheduler.Schedule(c.Request.Context(), workload)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to schedule workload: " + err.Error()})
			return
		}

		// Update workload with placement information
		workload.Status.Provider = placement.Provider
		if placement.Placement != nil {
			workload.Status.NodeID = placement.Placement.NodeID
		}
		workload.Status.Phase = types.WorkloadPhaseScheduled

		// Store workload in repository if available
		if appState.Repository != nil {
			if err := appState.Repository.CreateWorkload(c.Request.Context(), workload); err != nil {
				c.JSON(500, gin.H{"error": "failed to store workload: " + err.Error()})
				return
			}
		}

		response := gin.H{
			"id":        workload.ID,
			"name":      workload.Name,
			"namespace": workload.Namespace,
			"status":    workload.Status.Phase,
			"provider":  placement.Provider,
			"node_id":   workload.Status.NodeID,
		}

		// Add cost information if available
		if placement.EstimatedCost != nil {
			response["estimated_cost"] = placement.EstimatedCost
		}

		// Add score information if placement details are available
		if placement.Placement != nil {
			response["score"] = placement.Placement.Score
		}

		c.JSON(200, response)
	}
}

func getRecommendations(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if appState.Scheduler == nil {
			c.JSON(503, gin.H{"error": "scheduler not configured"})
			return
		}

		// Parse workload specification from query parameters
		name := c.Query("name")
		if name == "" {
			name = "recommendation-workload"
		}

		namespace := c.Query("namespace")
		if namespace == "" {
			namespace = "default"
		}

		image := c.Query("image")
		if image == "" {
			c.JSON(400, gin.H{"error": "image parameter is required"})
			return
		}

		cpu := c.Query("cpu")
		memory := c.Query("memory")
		gpu := c.Query("gpu")
		region := c.Query("region")
		provider := c.Query("provider")

		// Create temporary workload for recommendations
		workload := &types.Workload{
			ID:        "temp-" + generateID(),
			Name:      name,
			Namespace: namespace,
			Spec: types.WorkloadSpec{
				Image: image,
			},
			Status: types.WorkloadStatus{
				Phase: types.WorkloadPhasePending,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Set resource requirements if specified
		if cpu != "" || memory != "" || gpu != "" {
			workload.Spec.Resources = types.ResourceRequests{
				CPU:    cpu,
				Memory: memory,
				GPU:    gpu,
			}
		}

		// Set placement preferences if specified
		if region != "" || provider != "" {
			workload.Spec.Placement = types.PlacementSpec{
				Region:   region,
				Provider: provider,
			}
		}

		// Get recommendations from scheduler
		recommendations, err := appState.Scheduler.GetRecommendations(c.Request.Context(), workload)
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to get recommendations: " + err.Error()})
			return
		}

		// Convert recommendations to response format
		var result []gin.H
		for _, rec := range recommendations {
			recommendation := gin.H{
				"provider":     rec.Provider,
				"region":       rec.Region,
				"machine_type": rec.MachineType,
				"score":        rec.Score,
				"confidence":   rec.Confidence,
				"pros":         rec.Pros,
				"cons":         rec.Cons,
			}

			// Add cost information if available
			if rec.EstimatedCost != nil {
				recommendation["estimated_cost"] = rec.EstimatedCost
			}

			result = append(result, recommendation)
		}

		c.JSON(200, gin.H{
			"workload": gin.H{
				"name":      workload.Name,
				"namespace": workload.Namespace,
				"image":     workload.Spec.Image,
				"resources": workload.Spec.Resources,
				"placement": workload.Spec.Placement,
			},
			"recommendations": result,
			"total":           len(result),
		})
	}
}

func getSchedulerStats(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if appState.Scheduler == nil {
			c.JSON(503, gin.H{"error": "scheduler not configured"})
			return
		}

		stats, err := appState.Scheduler.GetStats(c.Request.Context())
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}

		c.JSON(200, stats)
	}
}
