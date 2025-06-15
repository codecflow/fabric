package fly

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents a Fly.io API client
type Client struct {
	apiToken string
	baseURL  string
	client   *http.Client
}

// NewClient creates a new Fly.io client
func NewClient(apiToken string) *Client {
	return &Client{
		apiToken: apiToken,
		baseURL:  "https://api.machines.dev/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateApp creates a new Fly.io application
func (c *Client) CreateApp(ctx context.Context, req *CreateAppRequest) (*App, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/apps", body)
	if err != nil {
		return nil, err
	}

	var app App
	if err := json.Unmarshal(resp, &app); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &app, nil
}

// GetApp retrieves an application by name
func (c *Client) GetApp(ctx context.Context, appName string) (*App, error) {
	resp, err := c.makeRequest(ctx, "GET", "/apps/"+appName, nil)
	if err != nil {
		return nil, err
	}

	var app App
	if err := json.Unmarshal(resp, &app); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &app, nil
}

// ListApps lists all applications
func (c *Client) ListApps(ctx context.Context) ([]*App, error) {
	resp, err := c.makeRequest(ctx, "GET", "/apps", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Apps []*App `json:"apps"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Apps, nil
}

// DeleteApp deletes an application
func (c *Client) DeleteApp(ctx context.Context, appName string) error {
	_, err := c.makeRequest(ctx, "DELETE", "/apps/"+appName, nil)
	return err
}

// CreateMachine creates a new machine in an app
func (c *Client) CreateMachine(ctx context.Context, appName string, req *CreateMachineRequest) (*Machine, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/apps/"+appName+"/machines", body)
	if err != nil {
		return nil, err
	}

	var machine Machine
	if err := json.Unmarshal(resp, &machine); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &machine, nil
}

// GetMachine retrieves a machine by ID
func (c *Client) GetMachine(ctx context.Context, appName, machineID string) (*Machine, error) {
	resp, err := c.makeRequest(ctx, "GET", "/apps/"+appName+"/machines/"+machineID, nil)
	if err != nil {
		return nil, err
	}

	var machine Machine
	if err := json.Unmarshal(resp, &machine); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &machine, nil
}

// ListMachines lists all machines in an app
func (c *Client) ListMachines(ctx context.Context, appName string) ([]*Machine, error) {
	resp, err := c.makeRequest(ctx, "GET", "/apps/"+appName+"/machines", nil)
	if err != nil {
		return nil, err
	}

	var machines []*Machine
	if err := json.Unmarshal(resp, &machines); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return machines, nil
}

// UpdateMachine updates a machine configuration
func (c *Client) UpdateMachine(ctx context.Context, appName, machineID string, req *UpdateMachineRequest) (*Machine, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/apps/"+appName+"/machines/"+machineID, body)
	if err != nil {
		return nil, err
	}

	var machine Machine
	if err := json.Unmarshal(resp, &machine); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &machine, nil
}

// StartMachine starts a machine
func (c *Client) StartMachine(ctx context.Context, appName, machineID string) error {
	_, err := c.makeRequest(ctx, "POST", "/apps/"+appName+"/machines/"+machineID+"/start", nil)
	return err
}

// StopMachine stops a machine
func (c *Client) StopMachine(ctx context.Context, appName, machineID string) error {
	_, err := c.makeRequest(ctx, "POST", "/apps/"+appName+"/machines/"+machineID+"/stop", nil)
	return err
}

// DeleteMachine deletes a machine
func (c *Client) DeleteMachine(ctx context.Context, appName, machineID string) error {
	_, err := c.makeRequest(ctx, "DELETE", "/apps/"+appName+"/machines/"+machineID, nil)
	return err
}

// GetMachineLogs retrieves machine logs
func (c *Client) GetMachineLogs(ctx context.Context, appName, machineID string) (string, error) {
	resp, err := c.makeRequest(ctx, "GET", "/apps/"+appName+"/machines/"+machineID+"/logs", nil)
	if err != nil {
		return "", err
	}

	return string(resp), nil
}

// ListRegions lists available Fly.io regions
func (c *Client) ListRegions(ctx context.Context) ([]*Region, error) {
	resp, err := c.makeRequest(ctx, "GET", "/regions", nil)
	if err != nil {
		return nil, err
	}

	var regions []*Region
	if err := json.Unmarshal(resp, &regions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return regions, nil
}

// GetMachineSizes retrieves available machine sizes
func (c *Client) GetMachineSizes(ctx context.Context) ([]*MachineSize, error) {
	resp, err := c.makeRequest(ctx, "GET", "/sizes", nil)
	if err != nil {
		return nil, err
	}

	var sizes []*MachineSize
	if err := json.Unmarshal(resp, &sizes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return sizes, nil
}

// CreateVolume creates a new volume
func (c *Client) CreateVolume(ctx context.Context, appName string, req map[string]interface{}) (*Volume, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/apps/"+appName+"/volumes", body)
	if err != nil {
		return nil, err
	}

	var volume Volume
	if err := json.Unmarshal(resp, &volume); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &volume, nil
}

// ListVolumes lists volumes for an app
func (c *Client) ListVolumes(ctx context.Context, appName string) ([]*Volume, error) {
	resp, err := c.makeRequest(ctx, "GET", "/apps/"+appName+"/volumes", nil)
	if err != nil {
		return nil, err
	}

	var volumes []*Volume
	if err := json.Unmarshal(resp, &volumes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return volumes, nil
}

// DeleteVolume deletes a volume
func (c *Client) DeleteVolume(ctx context.Context, appName, volumeID string) error {
	_, err := c.makeRequest(ctx, "DELETE", "/apps/"+appName+"/volumes/"+volumeID, nil)
	return err
}

// GetHealth checks API health
func (c *Client) GetHealth(ctx context.Context) error {
	_, err := c.makeRequest(ctx, "GET", "/health", nil)
	return err
}

// makeRequest makes an HTTP request to the Fly.io API
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
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
