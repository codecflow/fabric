package nosana

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents a Nosana API client
type Client struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewClient creates a new Nosana client
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey:  apiKey,
		baseURL: "https://api.nosana.io/v1",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CreateJob creates a new job on Nosana
func (c *Client) CreateJob(ctx context.Context, req *JobRequest) (*Job, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := c.makeRequest(ctx, "POST", "/jobs", body)
	if err != nil {
		return nil, err
	}

	var result struct {
		Job *Job `json:"job"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Job, nil
}

// GetJob retrieves a job by ID
func (c *Client) GetJob(ctx context.Context, id string) (*Job, error) {
	resp, err := c.makeRequest(ctx, "GET", "/jobs/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Job *Job `json:"job"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Job, nil
}

// ListJobs lists all jobs
func (c *Client) ListJobs(ctx context.Context) ([]*Job, error) {
	resp, err := c.makeRequest(ctx, "GET", "/jobs", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Jobs []*Job `json:"jobs"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Jobs, nil
}

// CancelJob cancels a job
func (c *Client) CancelJob(ctx context.Context, id string) error {
	_, err := c.makeRequest(ctx, "POST", "/jobs/"+id+"/cancel", nil)
	return err
}

// GetJobLogs retrieves job logs
func (c *Client) GetJobLogs(ctx context.Context, id string) (string, error) {
	resp, err := c.makeRequest(ctx, "GET", "/jobs/"+id+"/logs", nil)
	if err != nil {
		return "", err
	}

	var result struct {
		Logs string `json:"logs"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Logs, nil
}

// ListNodes lists available compute nodes
func (c *Client) ListNodes(ctx context.Context) ([]*Node, error) {
	resp, err := c.makeRequest(ctx, "GET", "/nodes", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Nodes []*Node `json:"nodes"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Nodes, nil
}

// GetNode retrieves a node by ID
func (c *Client) GetNode(ctx context.Context, id string) (*Node, error) {
	resp, err := c.makeRequest(ctx, "GET", "/nodes/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Node *Node `json:"node"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Node, nil
}

// ListMarkets lists available markets
func (c *Client) ListMarkets(ctx context.Context) ([]*Market, error) {
	resp, err := c.makeRequest(ctx, "GET", "/markets", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Markets []*Market `json:"markets"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Markets, nil
}

// GetMarket retrieves a market by ID
func (c *Client) GetMarket(ctx context.Context, id string) (*Market, error) {
	resp, err := c.makeRequest(ctx, "GET", "/markets/"+id, nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Market *Market `json:"market"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result.Market, nil
}

// GetHealth checks API health
func (c *Client) GetHealth(ctx context.Context) error {
	_, err := c.makeRequest(ctx, "GET", "/health", nil)
	return err
}

// makeRequest makes an HTTP request to the Nosana API
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
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

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
