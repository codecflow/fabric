package gateway

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/codecflow/fabric/pkg/types"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Server represents the gateway HTTP server
type Server struct {
	echo         *echo.Echo
	scheduler    *Scheduler
	workerClient *WorkerClient
	port         int
}

// NewServer creates a new gateway server
func NewServer(port int) *Server {
	scheduler := NewScheduler()
	workerClient := NewWorkerClient()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	server := &Server{
		echo:         e,
		scheduler:    scheduler,
		workerClient: workerClient,
		port:         port,
	}

	// Routes
	e.GET("/health", server.HealthCheck)
	e.POST("/workers/register", server.RegisterWorker)
	e.GET("/workers", server.ListWorkers)
	e.POST("/containers", server.CreateContainer)
	e.GET("/containers/:id", server.GetContainer)
	e.GET("/containers", server.ListContainers)
	e.DELETE("/containers/:id", server.StopContainer)

	return server
}

// HealthCheck handles GET /health
func (s *Server) HealthCheck(ctx echo.Context) error {
	workers := s.scheduler.ListWorkers()
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"status":        "healthy",
		"worker_count":  len(workers),
	})
}

// RegisterWorker handles POST /workers/register
func (s *Server) RegisterWorker(ctx echo.Context) error {
	var req types.RegisterWorkerRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Validate required fields
	if req.ID == "" || req.Address == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "ID and Address are required")
	}

	// Create worker
	worker := &types.Worker{
		ID:           req.ID,
		Address:      req.Address,
		Capabilities: req.Capabilities,
	}

	// Register with scheduler
	s.scheduler.RegisterWorker(worker)

	fmt.Printf("✅ Worker registered: %s at %s\n", worker.ID, worker.Address)

	return ctx.JSON(http.StatusCreated, types.RegisterWorkerResponse{
		ID:      worker.ID,
		Message: "Worker registered successfully",
	})
}

// ListWorkers handles GET /workers
func (s *Server) ListWorkers(ctx echo.Context) error {
	workers := s.scheduler.ListWorkers()
	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"workers": workers,
		"count":   len(workers),
	})
}

// CreateContainer handles POST /containers
func (s *Server) CreateContainer(ctx echo.Context) error {
	var req types.CreateContainerRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Select a worker
	worker, err := s.scheduler.SelectWorker()
	if err != nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, err.Error())
	}

	// Forward request to worker
	response, err := s.workerClient.CreateContainer(ctx.Request().Context(), worker.Address, &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create container on worker: %v", err))
	}

	// Add worker ID to response
	response.WorkerID = worker.ID

	fmt.Printf("📦 Container %s scheduled to worker %s\n", response.ID, worker.ID)

	return ctx.JSON(http.StatusCreated, response)
}

// GetContainer handles GET /containers/:id
func (s *Server) GetContainer(ctx echo.Context) error {
	containerID := ctx.Param("id")
	if containerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	// Get worker ID from query parameter or search all workers
	workerID := ctx.QueryParam("worker_id")

	if workerID != "" {
		// Query specific worker
		worker, err := s.scheduler.GetWorker(workerID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		result, err := s.workerClient.GetContainer(ctx.Request().Context(), worker.Address, containerID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		return ctx.JSON(http.StatusOK, result)
	}

	// Search all workers for the container
	workers := s.scheduler.ListWorkers()
	for _, worker := range workers {
		result, err := s.workerClient.GetContainer(ctx.Request().Context(), worker.Address, containerID)
		if err == nil {
			return ctx.JSON(http.StatusOK, result)
		}
	}

	return echo.NewHTTPError(http.StatusNotFound, "Container not found on any worker")
}

// ListContainers handles GET /containers
func (s *Server) ListContainers(ctx echo.Context) error {
	workers := s.scheduler.ListWorkers()
	allContainers := make([]interface{}, 0)

	for _, worker := range workers {
		result, err := s.workerClient.ListContainers(ctx.Request().Context(), worker.Address)
		if err != nil {
			continue // Skip workers that are unavailable
		}

		if containers, ok := result["containers"].([]interface{}); ok {
			allContainers = append(allContainers, containers...)
		}
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"containers": allContainers,
		"count":      len(allContainers),
	})
}

// StopContainer handles DELETE /containers/:id
func (s *Server) StopContainer(ctx echo.Context) error {
	containerID := ctx.Param("id")
	if containerID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	workerID := ctx.QueryParam("worker_id")

	if workerID != "" {
		// Stop on specific worker
		worker, err := s.scheduler.GetWorker(workerID)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, err.Error())
		}

		if err := s.workerClient.StopContainer(ctx.Request().Context(), worker.Address, containerID); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		return ctx.JSON(http.StatusOK, map[string]string{
			"id":      containerID,
			"message": "Container stopped",
		})
	}

	// Try all workers
	workers := s.scheduler.ListWorkers()
	for _, worker := range workers {
		if err := s.workerClient.StopContainer(ctx.Request().Context(), worker.Address, containerID); err == nil {
			return ctx.JSON(http.StatusOK, map[string]string{
				"id":      containerID,
				"message": "Container stopped",
			})
		}
	}

	return echo.NewHTTPError(http.StatusNotFound, "Container not found on any worker")
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("🚀 Gateway server starting on %s\n", addr)
	fmt.Printf("📋 API endpoints:\n")
	fmt.Printf("   POST   /workers/register - Register worker\n")
	fmt.Printf("   GET    /workers          - List workers\n")
	fmt.Printf("   POST   /containers       - Create container\n")
	fmt.Printf("   GET    /containers/:id   - Get container status\n")
	fmt.Printf("   GET    /containers       - List containers\n")
	fmt.Printf("   DELETE /containers/:id   - Stop container\n")
	fmt.Printf("   GET    /health           - Health check\n")
	fmt.Println()

	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	fmt.Println("🛑 Shutting down gateway server...")
	return s.echo.Shutdown(ctx)
}

// GracefulShutdown sets up a graceful shutdown with timeout
func (s *Server) GracefulShutdown(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		fmt.Printf("Error during shutdown: %v\n", err)
	}
}
