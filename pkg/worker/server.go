package worker

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/codecflow/fabric/pkg/placement"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Server represents the worker HTTP server
type Server struct {
	echo     *echo.Echo
	handler  *Handler
	strategy placement.Strategy
	port     int
}

// NewServer creates a new worker server
func NewServer(strategy placement.Strategy, port int) *Server {
	handler := NewHandler(strategy)

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

	// Routes
	e.GET("/health", handler.HealthCheck)
	e.POST("/containers", handler.CreateContainer)
	e.GET("/containers/:id", handler.GetContainer)
	e.GET("/containers", handler.ListContainers)
	e.DELETE("/containers/:id", handler.StopContainer)

	return &Server{
		echo:     e,
		handler:  handler,
		strategy: strategy,
		port:     port,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("🚀 Worker server starting on %s\n", addr)
	fmt.Printf("📋 API endpoints:\n")
	fmt.Printf("   POST   /containers      - Create container\n")
	fmt.Printf("   GET    /containers/:id  - Get container status\n")
	fmt.Printf("   GET    /containers      - List containers\n")
	fmt.Printf("   DELETE /containers/:id  - Stop container\n")
	fmt.Printf("   GET    /health          - Health check\n")
	fmt.Println()

	return s.echo.Start(addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	fmt.Println("🛑 Shutting down worker server...")
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
