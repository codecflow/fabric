package nats

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/codecflow/fabric/weaver/services/stream"
)

// NATSStream implements the Stream interface using NATS JetStream
type NATSStream struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// New creates a new NATS stream client
func New(url string) (*NATSStream, error) {
	conn, err := nats.Connect(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	s := &NATSStream{
		conn: conn,
		js:   js,
	}

	// Initialize default streams
	if err := s.initStreams(); err != nil {
		return nil, fmt.Errorf("failed to initialize streams: %w", err)
	}

	return s, nil
}

// initStreams creates the necessary JetStream streams
func (s *NATSStream) initStreams() error {
	streams := []nats.StreamConfig{
		{
			Name:     "FABRIC_EVENTS",
			Subjects: []string{"fabric.events.*", "fabric.workloads.*", "fabric.providers.*"},
			Storage:  nats.FileStorage,
			MaxAge:   24 * time.Hour,
		},
	}

	for _, streamConfig := range streams {
		_, err := s.js.StreamInfo(streamConfig.Name)
		if err != nil {
			// Stream doesn't exist, create it
			_, err = s.js.AddStream(&streamConfig)
			if err != nil {
				return fmt.Errorf("failed to create stream %s: %w", streamConfig.Name, err)
			}
		}
	}

	return nil
}

// Publish publishes a message to a subject
func (s *NATSStream) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := s.js.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}
	return nil
}

// PublishWithReply publishes a message and waits for a reply
func (s *NATSStream) PublishWithReply(ctx context.Context, subject string, data []byte, timeout time.Duration) ([]byte, error) {
	msg, err := s.conn.RequestWithContext(ctx, subject, data)
	if err != nil {
		return nil, fmt.Errorf("failed to publish with reply: %w", err)
	}
	return msg.Data, nil
}

// Subscribe subscribes to a subject
func (s *NATSStream) Subscribe(ctx context.Context, subject string, handler stream.MessageHandler) (stream.Subscription, error) {
	sub, err := s.conn.Subscribe(subject, func(msg *nats.Msg) {
		streamMsg := &stream.Message{
			Subject:   msg.Subject,
			Data:      msg.Data,
			Reply:     msg.Reply,
			Timestamp: time.Now(),
		}

		if err := handler(streamMsg); err != nil {
			log.Printf("failed to handle message: %v", err)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe: %w", err)
	}

	return &natsSubscription{sub: sub}, nil
}

// QueueSubscribe subscribes to a subject with queue semantics
func (s *NATSStream) QueueSubscribe(ctx context.Context, subject, queue string, handler stream.MessageHandler) (stream.Subscription, error) {
	sub, err := s.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		streamMsg := &stream.Message{
			Subject:   msg.Subject,
			Data:      msg.Data,
			Reply:     msg.Reply,
			Timestamp: time.Now(),
		}

		if err := handler(streamMsg); err != nil {
			log.Printf("failed to handle message: %v", err)
		}
	})
	if err != nil {
		return nil, fmt.Errorf("failed to queue subscribe: %w", err)
	}

	return &natsSubscription{sub: sub}, nil
}

// CreateStream creates a JetStream stream
func (s *NATSStream) CreateStream(ctx context.Context, config stream.StreamConfig) error {
	natsConfig := &nats.StreamConfig{
		Name:        config.Name,
		Description: config.Description,
		Subjects:    config.Subjects,
		MaxAge:      config.MaxAge,
		MaxBytes:    config.MaxBytes,
		MaxMsgs:     config.MaxMsgs,
		Replicas:    config.Replicas,
	}

	// Convert retention type
	switch config.Retention {
	case stream.RetentionLimits:
		natsConfig.Retention = nats.LimitsPolicy
	case stream.RetentionInterest:
		natsConfig.Retention = nats.InterestPolicy
	case stream.RetentionWorkQueue:
		natsConfig.Retention = nats.WorkQueuePolicy
	default:
		natsConfig.Retention = nats.LimitsPolicy
	}

	_, err := s.js.AddStream(natsConfig)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	return nil
}

// DeleteStream deletes a JetStream stream
func (s *NATSStream) DeleteStream(ctx context.Context, name string) error {
	err := s.js.DeleteStream(name)
	if err != nil {
		return fmt.Errorf("failed to delete stream: %w", err)
	}
	return nil
}

// AddConsumer adds a consumer to a stream
func (s *NATSStream) AddConsumer(ctx context.Context, streamName string, config stream.ConsumerConfig) error {
	natsConfig := &nats.ConsumerConfig{
		Name:           config.Name,
		Description:    config.Description,
		DeliverSubject: config.DeliverSubject,
		DeliverGroup:   config.DeliverGroup,
		AckWait:        config.AckWait,
		MaxDeliver:     config.MaxDeliver,
		FilterSubject:  config.FilterSubject,
	}

	// Convert deliver policy
	switch config.DeliverPolicy {
	case stream.DeliverAll:
		natsConfig.DeliverPolicy = nats.DeliverAllPolicy
	case stream.DeliverLast:
		natsConfig.DeliverPolicy = nats.DeliverLastPolicy
	case stream.DeliverNew:
		natsConfig.DeliverPolicy = nats.DeliverNewPolicy
	case stream.DeliverByStartSeq:
		natsConfig.DeliverPolicy = nats.DeliverByStartSequencePolicy
	case stream.DeliverByStartTime:
		natsConfig.DeliverPolicy = nats.DeliverByStartTimePolicy
	default:
		natsConfig.DeliverPolicy = nats.DeliverAllPolicy
	}

	// Convert ack policy
	switch config.AckPolicy {
	case stream.AckNone:
		natsConfig.AckPolicy = nats.AckNonePolicy
	case stream.AckAll:
		natsConfig.AckPolicy = nats.AckAllPolicy
	case stream.AckExplicit:
		natsConfig.AckPolicy = nats.AckExplicitPolicy
	default:
		natsConfig.AckPolicy = nats.AckExplicitPolicy
	}

	// Convert replay policy
	switch config.ReplayPolicy {
	case stream.ReplayInstant:
		natsConfig.ReplayPolicy = nats.ReplayInstantPolicy
	case stream.ReplayOriginal:
		natsConfig.ReplayPolicy = nats.ReplayOriginalPolicy
	default:
		natsConfig.ReplayPolicy = nats.ReplayInstantPolicy
	}

	_, err := s.js.AddConsumer(streamName, natsConfig)
	if err != nil {
		return fmt.Errorf("failed to add consumer: %w", err)
	}

	return nil
}

// Health check
func (s *NATSStream) HealthCheck(ctx context.Context) error {
	if s.conn.Status() != nats.CONNECTED {
		return fmt.Errorf("NATS connection not healthy")
	}

	// Test JetStream connectivity
	_, err := s.js.AccountInfo()
	if err != nil {
		return fmt.Errorf("JetStream not available: %w", err)
	}

	return nil
}

// Close closes the NATS connection
func (s *NATSStream) Close() error {
	if s.conn != nil {
		s.conn.Close()
	}
	return nil
}

// natsSubscription implements the Subscription interface
type natsSubscription struct {
	sub *nats.Subscription
}

func (ns *natsSubscription) Unsubscribe() error {
	return ns.sub.Unsubscribe()
}

func (ns *natsSubscription) IsValid() bool {
	return ns.sub.IsValid()
}

func (ns *natsSubscription) Subject() string {
	return ns.sub.Subject
}

func (ns *natsSubscription) Queue() string {
	return ns.sub.Queue
}
