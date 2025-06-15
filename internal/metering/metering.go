package metering

import (
	"context"
	"time"
)

// Meter defines the interface for usage metering
type Meter interface {
	// Record usage metrics
	RecordUsage(ctx context.Context, event UsageEvent) error
	RecordBatch(ctx context.Context, events []UsageEvent) error

	// Query usage data
	GetUsage(ctx context.Context, query UsageQuery) (*UsageReport, error)
	GetUsageByWorkload(ctx context.Context, workloadID string, timeRange TimeRange) (*UsageReport, error)
	GetUsageByNamespace(ctx context.Context, namespace string, timeRange TimeRange) (*UsageReport, error)

	// Health and maintenance
	HealthCheck(ctx context.Context) error
	Close() error
}

// UsageEvent represents a single usage measurement
type UsageEvent struct {
	ID         string                 `json:"id"`
	WorkloadID string                 `json:"workloadId"`
	Namespace  string                 `json:"namespace"`
	Provider   string                 `json:"provider"`
	NodeID     string                 `json:"nodeId,omitempty"`
	Timestamp  time.Time              `json:"timestamp"`
	Type       UsageType              `json:"type"`
	Value      float64                `json:"value"`
	Unit       string                 `json:"unit"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Metadata   map[string]string      `json:"metadata,omitempty"`
}

// UsageType defines the type of usage being measured
type UsageType string

const (
	// Compute usage
	UsageTypeCPUTime     UsageType = "cpu_time"     // CPU seconds
	UsageTypeMemoryTime  UsageType = "memory_time"  // Memory GB-seconds
	UsageTypeGPUTime     UsageType = "gpu_time"     // GPU seconds
	UsageTypeStorageTime UsageType = "storage_time" // Storage GB-seconds

	// Network usage
	UsageTypeNetworkIngress UsageType = "network_ingress" // Bytes in
	UsageTypeNetworkEgress  UsageType = "network_egress"  // Bytes out

	// Application-specific usage
	UsageTypeAPIRequests     UsageType = "api_requests"     // Number of API calls
	UsageTypeTokensGenerated UsageType = "tokens_generated" // AI tokens generated
	UsageTypeTokensProcessed UsageType = "tokens_processed" // AI tokens processed
	UsageTypeInferences      UsageType = "inferences"       // ML inferences

	// Custom usage types
	UsageTypeCustom UsageType = "custom" // Custom metrics
)

// UsageQuery defines parameters for querying usage data
type UsageQuery struct {
	WorkloadIDs []string          `json:"workloadIds,omitempty"`
	Namespaces  []string          `json:"namespaces,omitempty"`
	Providers   []string          `json:"providers,omitempty"`
	UsageTypes  []UsageType       `json:"usageTypes,omitempty"`
	TimeRange   TimeRange         `json:"timeRange"`
	GroupBy     []string          `json:"groupBy,omitempty"` // "workload", "namespace", "provider", "hour", "day"
	Filters     map[string]string `json:"filters,omitempty"`
	Limit       int               `json:"limit,omitempty"`
	Offset      int               `json:"offset,omitempty"`
}

// TimeRange defines a time period
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// UsageReport contains aggregated usage data
type UsageReport struct {
	Query       UsageQuery    `json:"query"`
	GeneratedAt time.Time     `json:"generatedAt"`
	Summary     UsageSummary  `json:"summary"`
	Details     []UsageDetail `json:"details"`
	Pagination  *Pagination   `json:"pagination,omitempty"`
}

// UsageSummary provides high-level usage statistics
type UsageSummary struct {
	TotalEvents   int                   `json:"totalEvents"`
	TotalValue    float64               `json:"totalValue"`
	AverageValue  float64               `json:"averageValue"`
	ByType        map[UsageType]float64 `json:"byType"`
	ByWorkload    map[string]float64    `json:"byWorkload,omitempty"`
	ByNamespace   map[string]float64    `json:"byNamespace,omitempty"`
	ByProvider    map[string]float64    `json:"byProvider,omitempty"`
	EstimatedCost *CostEstimate         `json:"estimatedCost,omitempty"`
}

// UsageDetail provides detailed usage information
type UsageDetail struct {
	GroupKey   string                 `json:"groupKey"` // Based on groupBy fields
	Events     int                    `json:"events"`
	TotalValue float64                `json:"totalValue"`
	Unit       string                 `json:"unit"`
	TimeRange  TimeRange              `json:"timeRange"`
	Properties map[string]interface{} `json:"properties,omitempty"`
}

// Pagination provides pagination information
type Pagination struct {
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	Total   int  `json:"total"`
	HasMore bool `json:"hasMore"`
}

// CostEstimate provides cost estimation based on usage
type CostEstimate struct {
	Currency    string                `json:"currency"`
	TotalCost   float64               `json:"totalCost"`
	ByType      map[UsageType]float64 `json:"byType"`
	ByWorkload  map[string]float64    `json:"byWorkload,omitempty"`
	ByNamespace map[string]float64    `json:"byNamespace,omitempty"`
	ByProvider  map[string]float64    `json:"byProvider,omitempty"`
	Breakdown   []CostBreakdown       `json:"breakdown,omitempty"`
}

// CostBreakdown provides detailed cost breakdown
type CostBreakdown struct {
	Category    string  `json:"category"` // "compute", "storage", "network"
	Description string  `json:"description"`
	Usage       float64 `json:"usage"`
	Unit        string  `json:"unit"`
	Rate        float64 `json:"rate"` // Cost per unit
	Cost        float64 `json:"cost"`
}

// MeterConfig defines configuration for the meter
type MeterConfig struct {
	Provider      string            `json:"provider"` // "openmeter", "prometheus"
	Endpoint      string            `json:"endpoint"`
	APIKey        string            `json:"apiKey,omitempty"`
	BatchSize     int               `json:"batchSize,omitempty"`
	FlushInterval time.Duration     `json:"flushInterval,omitempty"`
	RetryAttempts int               `json:"retryAttempts,omitempty"`
	Timeout       time.Duration     `json:"timeout,omitempty"`
	Properties    map[string]string `json:"properties,omitempty"`
}

// BillingPeriod defines a billing period
type BillingPeriod struct {
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Description string    `json:"description"` // "January 2024", "Q1 2024"
}

// Invoice represents a billing invoice
type Invoice struct {
	ID          string        `json:"id"`
	Namespace   string        `json:"namespace"`
	Period      BillingPeriod `json:"period"`
	GeneratedAt time.Time     `json:"generatedAt"`
	DueDate     time.Time     `json:"dueDate"`
	Status      InvoiceStatus `json:"status"`
	Currency    string        `json:"currency"`
	Subtotal    float64       `json:"subtotal"`
	Tax         float64       `json:"tax"`
	Total       float64       `json:"total"`
	LineItems   []LineItem    `json:"lineItems"`
	Usage       UsageReport   `json:"usage"`
}

// InvoiceStatus defines the status of an invoice
type InvoiceStatus string

const (
	InvoiceStatusDraft     InvoiceStatus = "draft"
	InvoiceStatusSent      InvoiceStatus = "sent"
	InvoiceStatusPaid      InvoiceStatus = "paid"
	InvoiceStatusOverdue   InvoiceStatus = "overdue"
	InvoiceStatusCancelled InvoiceStatus = "cancelled"
)

// LineItem represents a single line item on an invoice
type LineItem struct {
	Description string    `json:"description"`
	UsageType   UsageType `json:"usageType"`
	Quantity    float64   `json:"quantity"`
	Unit        string    `json:"unit"`
	Rate        float64   `json:"rate"`
	Amount      float64   `json:"amount"`
}
