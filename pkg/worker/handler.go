package worker

import (
	"fmt"
	"net/http"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/codecflow/fabric/pkg/placement"
	"github.com/labstack/echo/v4"
)

// Handler handles HTTP requests for the worker API
type Handler struct {
	strategy placement.Strategy
}

// NewHandler creates a new API handler
func NewHandler(strategy placement.Strategy) *Handler {
	return &Handler{
		strategy: strategy,
	}
}

// CreateContainerRequest represents the request body for creating a container
type CreateContainerRequest struct {
	Image      string            `json:"image" validate:"required"`
	Command    []string          `json:"command" validate:"required"`
	Env        map[string]string `json:"env,omitempty"`
	WorkDir    string            `json:"workdir,omitempty"`
	GPU        int               `json:"gpu,omitempty"`        // Number of GPUs
	Persistent bool              `json:"persistent,omitempty"` // Long-running container
	Ports      []int             `json:"ports,omitempty"`      // Container ports to expose
}

// CreateContainerResponse represents the response for creating a container
type CreateContainerResponse struct {
	ID          string      `json:"id"`
	ContainerID string      `json:"container_id,omitempty"`
	Status      string      `json:"status"`
	URL         string      `json:"url,omitempty"`   // URL to access the container
	Ports       map[int]int `json:"ports,omitempty"` // container port -> host port
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

	// Convert ports to PortSpec
	portSpecs := make([]container.PortSpec, 0, len(req.Ports))
	for _, port := range req.Ports {
		portSpecs = append(portSpecs, container.PortSpec{
			Container: port,
			Protocol:  "tcp", // Default to TCP
		})
	}

	// Create container spec
	spec := &container.Spec{
		Image:      req.Image,
		Command:    req.Command,
		Env:        req.Env,
		WorkDir:    req.WorkDir,
		GPU:        req.GPU,
		Persistent: req.Persistent,
		Ports:      portSpecs,
	}

	// Create placement request
	placementReq := &placement.Request{
		Spec:     spec,
		Provider: h.strategy.Name(),
	}

	// Create placement using strategy
	placementResp, err := h.strategy.Create(ctx.Request().Context(), placementReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	// Build URL from first mapped port
	url := ""
	if len(placementResp.Ports) > 0 {
		for _, hostPort := range placementResp.Ports {
			url = fmt.Sprintf("http://localhost:%d", hostPort)
			break
		}
	}

	response := CreateContainerResponse{
		ID:          placementResp.ID,
		ContainerID: placementResp.ContainerID,
		Status:      placementResp.Status,
		URL:         url,
		Ports:       placementResp.Ports,
	}

	return ctx.JSON(http.StatusCreated, response)
}

// GetContainer handles GET /containers/:id
func (h *Handler) GetContainer(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	placement, err := h.strategy.Get(ctx.Request().Context(), id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}

	return ctx.JSON(http.StatusOK, placement)
}

// ListContainers handles GET /containers
func (h *Handler) ListContainers(ctx echo.Context) error {
	placements, err := h.strategy.List(ctx.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return ctx.JSON(http.StatusOK, map[string]interface{}{
		"placements": placements,
		"count":      len(placements),
	})
}

// StopContainer handles DELETE /containers/:id
func (h *Handler) StopContainer(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Container ID is required")
	}

	err := h.strategy.Stop(ctx.Request().Context(), id)
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
