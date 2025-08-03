package stream

import (
	"context"
	"time"
)

// Stream defines the interface for event streaming
type Stream interface {
	// Publishing
	Publish(ctx context.Context, subject string, data []byte) error
	PublishWithReply(ctx context.Context, subject string, data []byte, timeout time.Duration) ([]byte, error)

	// Subscribing
	Subscribe(ctx context.Context, subject string, handler MessageHandler) (Subscription, error)
	QueueSubscribe(ctx context.Context, subject, queue string, handler MessageHandler) (Subscription, error)

	// JetStream (persistent messaging)
	CreateStream(ctx context.Context, config StreamConfig) error
	DeleteStream(ctx context.Context, name string) error
	AddConsumer(ctx context.Context, streamName string, config ConsumerConfig) error

	// Health and maintenance
	HealthCheck(ctx context.Context) error
	Close() error
}

// MessageHandler defines the function signature for message handlers
type MessageHandler func(msg *Message) error

// Message represents a stream message
type Message struct {
	Subject   string            `json:"subject"`
	Data      []byte            `json:"data"`
	Headers   map[string]string `json:"headers,omitempty"`
	Reply     string            `json:"reply,omitempty"`
	Timestamp time.Time         `json:"timestamp"`

	// JetStream specific
	StreamName     string `json:"streamName,omitempty"`
	StreamSequence uint64 `json:"streamSequence,omitempty"`
	ConsumerName   string `json:"consumerName,omitempty"`

	// Internal fields for acknowledgment
	ack  func() error
	nack func() error
}

// Ack acknowledges the message
func (m *Message) Ack() error {
	if m.ack != nil {
		return m.ack()
	}
	return nil
}

// Nack negatively acknowledges the message
func (m *Message) Nack() error {
	if m.nack != nil {
		return m.nack()
	}
	return nil
}

// Subscription represents an active subscription
type Subscription interface {
	Unsubscribe() error
	IsValid() bool
	Subject() string
	Queue() string
}

// StreamConfig defines configuration for a JetStream stream
type StreamConfig struct {
	Name        string        `json:"name"`
	Description string        `json:"description,omitempty"`
	Subjects    []string      `json:"subjects"`
	Retention   RetentionType `json:"retention"`
	MaxAge      time.Duration `json:"maxAge,omitempty"`
	MaxBytes    int64         `json:"maxBytes,omitempty"`
	MaxMsgs     int64         `json:"maxMsgs,omitempty"`
	Replicas    int           `json:"replicas,omitempty"`
}

// RetentionType defines how messages are retained
type RetentionType string

const (
	RetentionLimits    RetentionType = "limits"    // Retain based on limits
	RetentionInterest  RetentionType = "interest"  // Retain while there's interest
	RetentionWorkQueue RetentionType = "workqueue" // Work queue semantics
)

// ConsumerConfig defines configuration for a JetStream consumer
type ConsumerConfig struct {
	Name           string        `json:"name"`
	Description    string        `json:"description,omitempty"`
	DeliverSubject string        `json:"deliverSubject,omitempty"`
	DeliverGroup   string        `json:"deliverGroup,omitempty"`
	DeliverPolicy  DeliverPolicy `json:"deliverPolicy"`
	AckPolicy      AckPolicy     `json:"ackPolicy"`
	AckWait        time.Duration `json:"ackWait,omitempty"`
	MaxDeliver     int           `json:"maxDeliver,omitempty"`
	FilterSubject  string        `json:"filterSubject,omitempty"`
	ReplayPolicy   ReplayPolicy  `json:"replayPolicy,omitempty"`
}

// DeliverPolicy defines when to start delivering messages
type DeliverPolicy string

const (
	DeliverAll         DeliverPolicy = "all"               // Deliver all messages
	DeliverLast        DeliverPolicy = "last"              // Deliver starting with last message
	DeliverNew         DeliverPolicy = "new"               // Deliver only new messages
	DeliverByStartSeq  DeliverPolicy = "by_start_sequence" // Deliver starting from sequence
	DeliverByStartTime DeliverPolicy = "by_start_time"     // Deliver starting from time
)

// AckPolicy defines acknowledgment requirements
type AckPolicy string

const (
	AckNone     AckPolicy = "none"     // No acknowledgment required
	AckAll      AckPolicy = "all"      // Acknowledge all messages
	AckExplicit AckPolicy = "explicit" // Explicit acknowledgment required
)

// ReplayPolicy defines how fast to replay messages
type ReplayPolicy string

const (
	ReplayInstant  ReplayPolicy = "instant"  // Replay as fast as possible
	ReplayOriginal ReplayPolicy = "original" // Replay at original speed
)

// EventType defines common event types in Fabric
type EventType string

const (
	// Workload events
	EventWorkloadCreated   EventType = "workload.created"
	EventWorkloadUpdated   EventType = "workload.updated"
	EventWorkloadDeleted   EventType = "workload.deleted"
	EventWorkloadScheduled EventType = "workload.scheduled"
	EventWorkloadStarted   EventType = "workload.started"
	EventWorkloadStopped   EventType = "workload.stopped"
	EventWorkloadFailed    EventType = "workload.failed"

	// Namespace events
	EventNamespaceCreated EventType = "namespace.created"
	EventNamespaceUpdated EventType = "namespace.updated"
	EventNamespaceDeleted EventType = "namespace.deleted"

	// Secret events
	EventSecretCreated EventType = "secret.created"
	EventSecretUpdated EventType = "secret.updated"
	EventSecretDeleted EventType = "secret.deleted"

	// System events
	EventNodeJoined    EventType = "node.joined"
	EventNodeLeft      EventType = "node.left"
	EventNodeHealthy   EventType = "node.healthy"
	EventNodeUnhealthy EventType = "node.unhealthy"

	// Metrics events
	EventMetricsCollected EventType = "metrics.collected"
	EventUsageReported    EventType = "usage.reported"
)

// Event represents a structured event
type Event struct {
	Type     EventType              `json:"type"`
	Source   string                 `json:"source"`
	ID       string                 `json:"id"`
	Time     time.Time              `json:"time"`
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}
