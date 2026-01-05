package domain

import "time"

type Webhook struct {
	ID          string            `json:"id"`
	BucketID    string            `json:"bucket_id"`
	Name        string            `json:"name"`
	URL         string            `json:"url"`
	Events      []string          `json:"events"` // object.created, object.deleted, etc.
	Secret      string            `json:"secret"`
	Active      bool              `json:"active"`
	Headers     map[string]string `json:"headers,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

type WebhookDelivery struct {
	ID           string    `json:"id"`
	WebhookID    string    `json:"webhook_id"`
	Event        string    `json:"event"`
	Payload      string    `json:"payload"`
	StatusCode   int       `json:"status_code"`
	Response     string    `json:"response"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	DeliveredAt  time.Time `json:"delivered_at"`
}