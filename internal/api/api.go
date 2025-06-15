package api

import (
	"fabric/internal/state"

	"github.com/gin-gonic/gin"
)

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

		// Scheduler status
		v1.GET("/scheduler/status", getSchedulerStatus(appState))
	}

	return r
}

func createWorkload(appState *state.State) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement workload creation
		c.JSON(501, gin.H{"error": "not implemented"})
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
		c.JSON(200, gin.H{
			"status":          "running",
			"providers_count": len(appState.Providers),
		})
	}
}
