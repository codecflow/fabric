package worker

import (
	"net/http"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for the worker API
type Handler struct {
	manager *Manager
}

// NewHandler creates a new API handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// CreateContainerRequest represents the request body for creating a container
type CreateContainerRequest struct {
	Image   string            `json:"image" validate:"required"`
	Command []string          `json:"command" validate:"required"`
	Env     map[string]string `json:"env,omitempty"`
	WorkDir string            `json:"workdir,omitempty"`
}

// CreateContainerResponse represents the response for creating a container
type CreateContainerResponse struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// CreateContainer handles POST /containers
func (h *Handler) CreateContainer(ctx echo.Context) error {
	var req CreateContainerRequest
	if err := ctx.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Validate required fields
	if req.Image == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Image is required")
	}
	if len(req.Command) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Command is required")
	}

	// Create container spec
	spec := &container.Spec{
		Image:   req.Image,
		Command: req.Command,
		Env:     req.Env,
		WorkDir: req.WorkDir,
	}

	// Create container
	state, err := h.manager.Create(ctx.Request().Context(), spec)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusCreated, CreateContainerResponse{
		ID:     state.ID,
		Status: state.Status,
	})
}

// GetContainer handles GET /containers/:id
func (h *Handler) GetContainer(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	state, err := h.manager.Get(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return ctx.JSON(http.StatusOK, state)
}

// ListContainersResponse represents the response for listing containers
type ListContainersResponse struct {
	Containers []*ContainerState `json:"containers"`
	Count      int               `json:"count"`
}

// ListContainers handles GET /containers
func (h *Handler) ListContainers(ctx echo.Context) error {
	states := h.manager.List()

	return ctx.JSON(http.StatusOK, ListContainersResponse{
		Containers: states,
		Count:      len(states),
	})
}

// StopContainer handles DELETE /containers/:id
func (h *Handler) StopContainer(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	err := h.manager.Stop(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]string{
		"id":      id,
		"message": "Container stopped",
	})
}

// HealthCheck handles GET /health
func (h *Handler) HealthCheck(ctx echo.Context) error {
	return ctx.JSON(http.StatusOK, map[string]string{
		"status": "healthy",
	})
}
