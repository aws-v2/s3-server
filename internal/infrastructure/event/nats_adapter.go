package event

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"s3/internal/domain"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

// NATSAdapter implements event publishing using NATS
type NATSAdapter struct {
	conn    *nats.Conn
	js      nats.JetStreamContext
	profile string
	natsPrefix string
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
func NewNATSAdapter(url, user, password string, profile,natsPrefix string) (*NATSAdapter, error) {
	// Connect to NATS with resilient options
	options := []nats.Option{
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1), // Infinite reconnects
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectHandler(func(c *nats.Conn) {
			slog.Warn("[NATS] disconnected from NATS server")
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			slog.Info("[NATS] reconnected to NATS server", slog.String("url", c.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(c *nats.Conn) {
			slog.Error("[NATS] NATS connection closed permanently", slog.Any("error", c.LastError()))
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
		slog.Warn("[NATS] failed to create JetStream context (will retry lazy)", slog.Any("error", err))
	}

	adapter := &NATSAdapter{
		conn:    conn,
		js:      js,
		profile: profile,
		natsPrefix: natsPrefix,
	}

	// Initialize streams
	if err := adapter.initializeStreams(); err != nil {
		slog.Warn("failed to initialize streams", slog.Any("error", err))
	}

	return adapter, nil
}

// BuildSubject constructs a standardized NATS subject: <profile>.<service>.<version>.<domain>.<action>
func (n *NATSAdapter) BuildSubject(version, domain, action string) string {
	return fmt.Sprintf("%s.s3.%s.%s.%s", n.profile, version, domain, action)
}

// initializeStreams creates necessary JetStream streams
func (n *NATSAdapter) initializeStreams() error {
	streams := []struct {
		name     string
		subjects []string
	}{
		{
			name:     "S3_EVENTS",
			subjects: []string{fmt.Sprintf("%s.s3.v1.events.>", n.profile)},
		},
		{
			name:     "FILE_EVENTS",
			subjects: []string{fmt.Sprintf("%s.s3.v1.files.>", n.profile)},
		},
		{
			name:     "BUCKET_EVENTS",
			subjects: []string{fmt.Sprintf("%s.s3.v1.buckets.>", n.profile)},
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
		slog.Info("Created JetStream stream", slog.String("name", stream.name))
	}

	return nil
}

// Publish publishes an event to NATS JetStream
func (n *NATSAdapter) Publish(ctx context.Context, topic string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Publish to JetStream for persistence
	_, err = n.js.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish event to jetstream: %w", err)
	}

	return nil
}

// PublishRaw publishes an event to Core NATS (fire and forget)
func (n *NATSAdapter) PublishRaw(ctx context.Context, topic string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Publish to core NATS
	err = n.conn.Publish(topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish event to core nats: %w", err)
	}

	return nil
}

// PublishEvent publishes a structured event
func (n *NATSAdapter) PublishEvent(ctx context.Context, event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	topic := n.BuildSubject("v1", "events", event.Type)
	return n.Publish(ctx, topic, event)
}

// PublishFileEvent publishes a file-related event
func (n *NATSAdapter) PublishFileEvent(ctx context.Context, eventType, bucketID, fileID, key string, metadata interface{}) error {
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

	topic := fmt.Sprintf("%s.metrics.raw.logs",n.natsPrefix)
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

	topic := n.BuildSubject("v1", "buckets", eventType)
	return n.Publish(ctx, topic, event)
}

// Subscribe subscribes to events on a topic
func (n *NATSAdapter) Subscribe(topic string, handler func([]byte) error) (*nats.Subscription, error) {
	return n.conn.Subscribe(topic, func(msg *nats.Msg) {
		if err := handler(msg.Data); err != nil {
			slog.Error("Error handling message", slog.String("topic", topic), slog.Any("error", err))
		}
	})
}

// QueueSubscribe subscribes to events with queue group
func (n *NATSAdapter) QueueSubscribe(topic, queue string, handler func([]byte) error) (*nats.Subscription, error) {
	return n.conn.QueueSubscribe(topic, queue, func(msg *nats.Msg) {
		if err := handler(msg.Data); err != nil {
			slog.Error("Error handling message", slog.String("topic", topic), slog.String("queue", queue), slog.Any("error", err))
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
	InstanceID string `json:"instanceID"`
	UserID     string `json:"userID"`
	Payload    string `json:"payload"` // base64url-encoded presigned URL payload
}

type instanceTokenResponse struct {
	Token string `json:"token"`
	Error string `json:"error"`
}

// RequestInstanceToken asks the IAM service for a scoped JWT token for the metrics agent.
func (n *NATSAdapter) RequestInstanceToken(ctx context.Context, userID, instanceID, payloadEncoded string) (string, error) {
	correlationID := uuid.New().String()
	subject := fmt.Sprintf("%s.iam.token.generate", n.natsPrefix)

	req := instanceTokenRequest{
		InstanceID: instanceID,
		UserID:     userID,
		Payload:    payloadEncoded,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal instance token request: %w", err)
	}

	slog.Info("[S3-NATS] [REQUEST]", 
		slog.String("subject", subject), 
		slog.String("correlation_id", correlationID), 
		slog.String("user_id", userID), 
		slog.String("instance_id", instanceID))

	msg, err := n.conn.RequestWithContext(ctx, subject, data)
	if err != nil {
		slog.Error("[S3-NATS] [ERROR] RequestInstanceToken failed", 
			slog.String("correlation_id", correlationID), 
			slog.Any("error", err), 
			slog.String("status", n.conn.Status().String()))
		return "", fmt.Errorf("NATS request failed: %w", err)
	}

	var resp instanceTokenResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return "", fmt.Errorf("failed to unmarshal instance token response: %w", err)
	}

	if resp.Error != "" {
		slog.Error("[S3-NATS] [FAILURE] RequestInstanceToken", slog.String("correlation_id", correlationID), slog.String("error", resp.Error))
		return "", fmt.Errorf("IAM service error: %s", resp.Error)
	}

	if resp.Token == "" {
		slog.Error("[S3-NATS] [FAILURE] RequestInstanceToken", slog.String("correlation_id", correlationID), slog.String("error", "empty_token"))
		return "", fmt.Errorf("IAM service returned an empty token")
	}

	slog.Info("[S3-NATS] [SUCCESS] Instance token received", slog.String("correlation_id", correlationID), slog.String("instance_id", instanceID))
	return resp.Token, nil
}

type listVPCsRequest struct {
	CorrelationID string `json:"correlation_id"`
	TenantID      string `json:"tenant_id"`
}

type listVPCsResponse struct {
	CorrelationID string       `json:"correlation_id"`
	TenantID      string       `json:"tenant_id"`
	VPCs          []domain.VPC `json:"vpcs"`
	Error         string       `json:"error,omitempty"`
}

// ListVPCs requests the VPC list from the network service via NATS
func (n *NATSAdapter) ListVPCs(ctx context.Context, tenantID string) ([]domain.VPC, error) {
	correlationID := uuid.New().String()
	subject := fmt.Sprintf("%s.network.v1.vpc.list", n.profile)

	req := listVPCsRequest{
		CorrelationID: correlationID,
		TenantID:      tenantID,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal vpc list request: %w", err)
	}

	slog.Info("[S3-NATS] [REQUEST]", slog.String("subject", subject), slog.String("correlation_id", correlationID), slog.String("tenant_id", tenantID))

	msg, err := n.conn.RequestWithContext(ctx, subject, data)
	if err != nil {
		return nil, fmt.Errorf("NATS request for VPCs failed: %w", err)
	}

	var resp listVPCsResponse
	if err := json.Unmarshal(msg.Data, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal vpc list response: %w", err)
	}

	if resp.Error != "" {
		return nil, fmt.Errorf("network service error: %s", resp.Error)
	}

	return resp.VPCs, nil
}
