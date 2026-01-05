package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

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
func NewNATSAdapter(url string) (*NATSAdapter, error) {
	// Connect to NATS
	conn, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(10),
		nats.ReconnectWait(2*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create JetStream context
	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
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
