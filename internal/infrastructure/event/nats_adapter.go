package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NATSAdapter implements event publishing using NATS
type NATSAdapter struct {
	conn *nats.Conn
	js   nats.JetStreamContext
}

// Event represents a domain event
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// NewNATSAdapter creates a new NATS event publisher
func NewNATSAdapter(url, user, password string) (*NATSAdapter, error) {
	// Connect to NATS with resilient options
	options := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectHandler(func(c *nats.Conn) {
			log.Printf("[NATS] Warn: disconnected from NATS server")
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			log.Printf("[NATS] Success: reconnected to NATS server at %s", c.ConnectedUrl())
		}),
		nats.ClosedHandler(func(c *nats.Conn) {
			log.Printf("[NATS] Critical: NATS connection closed permanently: %v", c.LastError())
		}),
	}

	if user != "" && password != "" {
		options = append(options, nats.UserInfo(user, password))
	}

	conn, err := nats.Connect(url, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := conn.JetStream()
	if err != nil {
		log.Printf("[NATS] Warn: failed to create JetStream context (will retry lazy): %v", err)
		// We DON'T call conn.Close() here anymore, so the base connection survives
	}

	adapter := &NATSAdapter{
		conn: conn,
		js:   js,
	}

	// Initialize streams
	if err := adapter.initializeStreams(); err != nil {
		log.Printf("Warning: failed to initialize streams: %v", err)
	}

	return adapter, nil
}

// initializeStreams creates necessary JetStream streams
func (n *NATSAdapter) initializeStreams() error {
	streams := []struct {
		name     string
		subjects []string
	}{
		{
			name:     "S3_EVENTS",
			subjects: []string{"s3.events.>"},
		},
		{
			name:     "FILE_EVENTS",
			subjects: []string{"s3.files.>"},
		},
		{
			name:     "BUCKET_EVENTS",
			subjects: []string{"s3.buckets.>"},
		},
	}

	for _, stream := range streams {
		// Check if stream exists
		_, err := n.js.StreamInfo(stream.name)
		if err == nil {
			continue // Stream already exists
		}

		// Create stream
		_, err = n.js.AddStream(&nats.StreamConfig{
			Name:     stream.name,
			Subjects: stream.subjects,
			MaxAge:   7 * 24 * time.Hour, // Retain events for 7 days
			Storage:  nats.FileStorage,
		})
		if err != nil {
			return fmt.Errorf("failed to create stream %s: %w", stream.name, err)
		}
		log.Printf("Created JetStream stream: %s", stream.name)
	}

	return nil
}

// Publish publishes an event to NATS
func (n *NATSAdapter) Publish(ctx context.Context, topic string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Publish to JetStream for persistence
	_, err = n.js.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// PublishEvent publishes a structured event
func (n *NATSAdapter) PublishEvent(ctx context.Context, event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	topic := fmt.Sprintf("s3.events.%s", event.Type)
	return n.Publish(ctx, topic, event)
}

// PublishFileEvent publishes a file-related event
func (n *NATSAdapter) PublishFileEvent(ctx context.Context, eventType, bucketID, fileID, key string, metadata map[string]interface{}) error {
	event := Event{
		Type:      eventType,
		Source:    "s3-service",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"bucket_id": bucketID,
			"file_id":   fileID,
			"key":       key,
			"metadata":  metadata,
		},
	}

	topic := fmt.Sprintf("s3.files.%s", eventType)
	return n.Publish(ctx, topic, event)
}

// PublishBucketEvent publishes a bucket-related event
func (n *NATSAdapter) PublishBucketEvent(ctx context.Context, eventType, bucketID, bucketName string, metadata map[string]interface{}) error {
	event := Event{
		Type:      eventType,
		Source:    "s3-service",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"bucket_id":   bucketID,
			"bucket_name": bucketName,
			"metadata":    metadata,
		},
	}

	topic := fmt.Sprintf("s3.buckets.%s", eventType)
	return n.Publish(ctx, topic, event)
}

// Subscribe subscribes to events on a topic
func (n *NATSAdapter) Subscribe(topic string, handler func([]byte) error) (*nats.Subscription, error) {
	return n.conn.Subscribe(topic, func(msg *nats.Msg) {
		if err := handler(msg.Data); err != nil {
			log.Printf("Error handling message: %v", err)
		}
	})
}

// QueueSubscribe subscribes to events with queue group
func (n *NATSAdapter) QueueSubscribe(topic, queue string, handler func([]byte) error) (*nats.Subscription, error) {
	return n.conn.QueueSubscribe(topic, queue, func(msg *nats.Msg) {
		if err := handler(msg.Data); err != nil {
			log.Printf("Error handling message: %v", err)
		}
	})
}

// CreateConsumer creates a durable consumer
func (n *NATSAdapter) CreateConsumer(streamName, consumerName string, subjects []string) error {
	_, err := n.js.AddConsumer(streamName, &nats.ConsumerConfig{
		Durable:       consumerName,
		FilterSubject: subjects[0],
		AckPolicy:     nats.AckExplicitPolicy,
	})
	return err
}

// Close closes the NATS connection
func (n *NATSAdapter) Close() error {
	if n.conn != nil {
		n.conn.Close()
	}
	return nil
}

// IsConnected checks if NATS connection is active
func (n *NATSAdapter) IsConnected() bool {
	return n.conn != nil && n.conn.IsConnected()
}

// Drain drains the connection gracefully
func (n *NATSAdapter) Drain() error {
	if n.conn != nil {
		return n.conn.Drain()
	}
	return nil
}

// GetConnection returns the underlying NATS connection
func (n *NATSAdapter) GetConnection() *nats.Conn {
	return n.conn
}

type instanceTokenRequest struct {
	InstanceID string `json:"instance_id"`
	UserID     string `json:"user_id"`
}

type instanceTokenResponse struct {
	Token string `json:"token"`
	Error string `json:"error"`
}

// RequestInstanceToken asks the IAM service for a scoped JWT token for the metrics agent.
func (n *NATSAdapter) RequestInstanceToken(ctx context.Context, userID, instanceID string) (string, error) {
	correlationID := uuid.New().String()
	subject := "dev.iam.v1.token.generate"

	req := instanceTokenRequest{
		InstanceID: instanceID,
		UserID:     userID,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal instance token request: %w", err)
	}

	log.Printf("[S3-NATS] [REQUEST] subject=%s correlation_id=%s user_id=%s instance_id=%s status=%s",
		subject, correlationID, userID, instanceID, n.conn.Status())

	msg, err := n.conn.RequestWithContext(ctx, subject, data)
	if err != nil {
		log.Printf("[S3-NATS] [ERROR] RequestInstanceToken failed: correlation_id=%s error=%v last_err=%v status=%s",
			correlationID, err, n.conn.LastError(), n.conn.Status())
		return "", fmt.Errorf("NATS request failed: %w", err)
	}

	var resp instanceTokenResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal instance token response: %w", err)
	}

	if resp.Error != "" {
		log.Printf("[S3-NATS] [FAILURE] RequestInstanceToken: correlation_id=%s error=%s", correlationID, resp.Error)
		return "", fmt.Errorf("IAM service error: %s", resp.Error)
	}

	if resp.Token == "" {
		log.Printf("[S3-NATS] [FAILURE] RequestInstanceToken: correlation_id=%s error=empty_token", correlationID)
		return "", fmt.Errorf("IAM service returned an empty token")
	}

	log.Printf("[S3-NATS] [SUCCESS] Instance token received: correlation_id=%s instance_id=%s", correlationID, instanceID)
	return resp.Token, nil
}
