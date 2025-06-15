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

		// TODO: Store workload in repository
		// For now, just simulate successful creation

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
		// TODO: Implement workload retrieval
		c.JSON(501, gin.H{"error": "not implemented"})
	}
}

func deleteWorkload(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement workload deletion
		c.JSON(501, gin.H{"error": "not implemented"})
	}
}

func listWorkloads(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement workload listing
		c.JSON(501, gin.H{"error": "not implemented"})
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
		_, exists := appState.Providers[providerName]
		if !exists {
			c.JSON(404, gin.H{"error": "provider not found"})
			return
		}

		// TODO: Update to use new provider interface
		c.JSON(501, gin.H{"error": "not implemented"})
	}
}

func listProviderMachineTypes(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		providerName := c.Param("name")
		_, exists := appState.Providers[providerName]
		if !exists {
			c.JSON(404, gin.H{"error": "provider not found"})
			return
		}

		// TODO: Update to use new provider interface
		c.JSON(501, gin.H{"error": "not implemented"})
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

		// TODO: Parse workload from request body and schedule it
		c.JSON(501, gin.H{"error": "not implemented"})
	}
}

func getRecommendations(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		if appState.Scheduler == nil {
			c.JSON(503, gin.H{"error": "scheduler not configured"})
			return
		}

		// TODO: Parse workload from query params and get recommendations
		c.JSON(501, gin.H{"error": "not implemented"})
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
