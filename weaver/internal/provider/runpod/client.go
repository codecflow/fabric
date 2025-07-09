package runpod

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents a RunPod API client
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClient creates a new RunPod client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.runpod.io/v2",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreatePodRequest represents a pod creation request
type CreatePodRequest struct {
	Name          string            `json:"name"`
	ImageName     string            `json:"imageName"`
	GPUTypeID     string            `json:"gpuTypeId"`
	GPUCount      int               `json:"gpuCount"`
	VCPUCount     int               `json:"vcpuCount"`
	MemoryInGB    int               `json:"memoryInGb"`
	ContainerDisk int               `json:"containerDiskInGb"`
	Env           map[string]string `json:"env"`
	Ports         string            `json:"ports"`
}

// CreatePod creates a new pod
func (c *Client) CreatePod(ctx context.Context, req *CreatePodRequest) (*Pod, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/pods", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Pod *Pod `json:"pod"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Pod, nil
}

// GetPod retrieves a pod by ID
func (c *Client) GetPod(ctx context.Context, id string) (*Pod, error) {
	resp, err := c.makeRequest(ctx, "GET", "/pods/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Pod *Pod `json:"pod"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Pod, nil
}

// GetPods lists all pods
func (c *Client) GetPods(ctx context.Context) ([]*Pod, error) {
	resp, err := c.makeRequest(ctx, "GET", "/pods", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Pods []*Pod `json:"pods"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Pods, nil
}

// TerminatePod terminates a pod
func (c *Client) TerminatePod(ctx context.Context, id string) error {
	_, err := c.makeRequest(ctx, "POST", "/pods/"+id+"/terminate", nil)
	return err
}

// GetGPUTypes retrieves available GPU types
func (c *Client) GetGPUTypes(ctx context.Context) ([]*GPUType, error) {
	resp, err := c.makeRequest(ctx, "GET", "/gpu-types", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		GPUTypes []*GPUType `json:"gpuTypes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.GPUTypes, nil
}

// GetUser retrieves user information (for health checks)
func (c *Client) GetUser(ctx context.Context) (*User, error) {
	resp, err := c.makeRequest(ctx, "GET", "/user", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		User *User `json:"user"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.User, nil
}

// User represents a RunPod user
type User struct {
	ID string `json:"id"`
}

// makeRequest makes an HTTP request to the RunPod API
func (c *Client) makeRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	url := c.baseURL + path

	var reqBody io.Reader
	if body != nil {
		reqBody = bytes.NewBuffer(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
