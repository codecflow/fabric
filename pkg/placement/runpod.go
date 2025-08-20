package placement

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/codecflow/fabric/pkg/container"
	"github.com/google/uuid"
)

// RunPodStrategy implements Strategy using RunPod as the backend
type RunPodStrategy struct {
	apiKey     string
	baseURL    string
	placements map[string]*Placement
	mu         sync.RWMutex
}

// NewRunPodStrategy creates a new RunPod placement strategy
func NewRunPodStrategy(apiKey string) (*RunPodStrategy, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("RunPod API key is required")
	}

	return &RunPodStrategy{
		apiKey:     apiKey,
		baseURL:    "https://api.runpod.io/graphql",
		placements: make(map[string]*Placement),
	}, nil
}

// Name returns the name of this strategy
func (r *RunPodStrategy) Name() string {
	return "runpod"
}

// GraphQL request/response types
type graphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables,omitempty"`
}

type graphQLResponse struct {
	Data   json.RawMessage        `json:"data"`
	Errors []graphQLError         `json:"errors,omitempty"`
}

type graphQLError struct {
	Message string `json:"message"`
}

// Create creates a pod on RunPod
func (r *RunPodStrategy) Create(ctx context.Context, req *Request) (*Placement, error) {
	// Generate unique placement ID
	id := uuid.New().String()
	podName := fmt.Sprintf("fabric-%s", id[:8])

	// Create placement record
	placement := &Placement{
		ID:       id,
		Provider: "runpod",
		Spec:     req.Spec,
		Status:   "created",
		Metadata: make(map[string]interface{}),
	}

	// Store placement
	r.mu.Lock()
	r.placements[id] = placement
	r.mu.Unlock()

	// Determine GPU type
	gpuTypeId := "NVIDIA GeForce RTX 3090" // Default
	if req.Spec.GPU > 0 {
		// Map GPU requirements to RunPod GPU types
		gpuTypeId = r.mapGPUType(req.Spec.GPU)
	}

	// Create pod mutation
	mutation := `
	mutation CreatePod($input: PodFindAndDeployOnDemandInput!) {
		podFindAndDeployOnDemand(input: $input) {
			id
			desiredStatus
			imageName
			costPerHr
		}
	}`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"cloudType":          "SECURE",
			"gpuCount":           req.Spec.GPU,
			"volumeInGb":         10,
			"containerDiskInGb":  10,
			"gpuTypeId":          gpuTypeId,
			"name":               podName,
			"imageName":          req.Spec.Image,
			"dockerArgs":         r.buildDockerArgs(req.Spec),
			"volumeMountPath":    "/workspace",
			"env": r.buildEnvVars(req.Spec),
		},
	}

	// Add port configuration if specified
	if len(req.Spec.Ports) > 0 {
		ports := fmt.Sprintf("%d/http", req.Spec.Ports[0].Container)
		variables["input"].(map[string]interface{})["ports"] = ports
	}

	// Execute GraphQL mutation
	respData, err := r.executeGraphQL(ctx, mutation, variables)
	if err != nil {
		r.mu.Lock()
		placement.Status = "failed"
		placement.Metadata["error"] = err.Error()
		r.mu.Unlock()
		return placement, fmt.Errorf("failed to create RunPod pod: %w", err)
	}

	// Parse response
	var result struct {
		PodFindAndDeployOnDemand struct {
			ID            string  `json:"id"`
			DesiredStatus string  `json:"desiredStatus"`
			ImageName     string  `json:"imageName"`
			CostPerHr     float64 `json:"costPerHr"`
		} `json:"podFindAndDeployOnDemand"`
	}

	if err := json.Unmarshal(respData, &result); err != nil {
		return placement, fmt.Errorf("failed to parse response: %w", err)
	}

	// Update placement
	r.mu.Lock()
	placement.ContainerID = result.PodFindAndDeployOnDemand.ID
	placement.Status = "running"
	placement.Metadata["pod_id"] = result.PodFindAndDeployOnDemand.ID
	placement.Metadata["cost_per_hr"] = result.PodFindAndDeployOnDemand.CostPerHr
	r.mu.Unlock()

	return placement, nil
}

// Get retrieves a placement by ID
func (r *RunPodStrategy) Get(ctx context.Context, id string) (*Placement, error) {
	r.mu.RLock()
	placement, ok := r.placements[id]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("placement not found: %s", id)
	}

	// Query RunPod for current status
	podID, ok := placement.Metadata["pod_id"].(string)
	if !ok || podID == "" {
		return placement, nil
	}

	query := `
	query Pod($input: PodFilter!) {
		pod(input: $input) {
			id
			desiredStatus
			runtime {
				ports {
					ip
					privatePort
					publicPort
				}
			}
		}
	}`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"podId": podID,
		},
	}

	respData, err := r.executeGraphQL(ctx, query, variables)
	if err != nil {
		return placement, nil // Return cached placement on error
	}

	var result struct {
		Pod struct {
			ID            string `json:"id"`
			DesiredStatus string `json:"desiredStatus"`
		} `json:"pod"`
	}

	if err := json.Unmarshal(respData, &result); err == nil {
		r.mu.Lock()
		placement.Status = result.Pod.DesiredStatus
		r.mu.Unlock()
	}

	return placement, nil
}

// Stop stops a running placement
func (r *RunPodStrategy) Stop(ctx context.Context, id string) error {
	r.mu.RLock()
	placement, ok := r.placements[id]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("placement not found: %s", id)
	}

	podID, ok := placement.Metadata["pod_id"].(string)
	if !ok || podID == "" {
		return fmt.Errorf("pod ID not found in placement metadata")
	}

	// Stop pod mutation
	mutation := `
	mutation StopPod($input: PodStopInput!) {
		podStop(input: $input) {
			id
			desiredStatus
		}
	}`

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"podId": podID,
		},
	}

	_, err := r.executeGraphQL(ctx, mutation, variables)
	if err != nil {
		return fmt.Errorf("failed to stop pod: %w", err)
	}

	r.mu.Lock()
	placement.Status = "stopped"
	r.mu.Unlock()

	return nil
}

// Delete removes a placement
func (r *RunPodStrategy) Delete(ctx context.Context, id string) error {
	// Stop first
	if err := r.Stop(ctx, id); err != nil {
		return err
	}

	r.mu.RLock()
	placement, ok := r.placements[id]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("placement not found: %s", id)
	}

	podID, ok := placement.Metadata["pod_id"].(string)
	if ok && podID != "" {
		// Terminate pod
		mutation := `
		mutation TerminatePod($input: PodTerminateInput!) {
			podTerminate(input: $input) {
				id
			}
		}`

		variables := map[string]interface{}{
			"input": map[string]interface{}{
				"podId": podID,
			},
		}

		_, err := r.executeGraphQL(ctx, mutation, variables)
		if err != nil {
			return fmt.Errorf("failed to terminate pod: %w", err)
		}
	}

	r.mu.Lock()
	delete(r.placements, id)
	r.mu.Unlock()

	return nil
}

// List returns all placements
func (r *RunPodStrategy) List(ctx context.Context) ([]*Placement, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	placements := make([]*Placement, 0, len(r.placements))
	for _, p := range r.placements {
		placements = append(placements, p)
	}

	return placements, nil
}

// Helper methods

func (r *RunPodStrategy) executeGraphQL(ctx context.Context, query string, variables map[string]interface{}) (json.RawMessage, error) {
	reqBody := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", r.baseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.apiKey))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var gqlResp graphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", gqlResp.Errors[0].Message)
	}

	return gqlResp.Data, nil
}

func (r *RunPodStrategy) mapGPUType(gpuCount int) string {
	// Default to RTX 3090 for now
	// TODO: Make this configurable or map based on requirements
	return "NVIDIA GeForce RTX 3090"
}

func (r *RunPodStrategy) buildDockerArgs(spec *container.Spec) string {
	if len(spec.Command) == 0 {
		return ""
	}
	// Join command arguments
	args := ""
	for i, arg := range spec.Command {
		if i > 0 {
			args += " "
		}
		args += arg
	}
	return args
}

func (r *RunPodStrategy) buildEnvVars(spec *container.Spec) []map[string]string {
	envVars := make([]map[string]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		envVars = append(envVars, map[string]string{
			"key":   k,
			"value": v,
		})
	}
	return envVars
}
