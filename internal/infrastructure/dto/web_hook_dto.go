package dto

import "time"

type CreateWebhookInput struct {
	BucketID string            `json:"bucket_id" binding:"required"`
	Name     string            `json:"name" binding:"required"`
	URL      string            `json:"url" binding:"required,url"`
	Events   []string          `json:"events" binding:"required,min=1"`
	Secret   string            `json:"secret"`
	Headers  map[string]string `json:"headers"`
}

type CreateWebhookOutput struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret"`
	CreatedAt time.Time `json:"created_at"`
}

type ListWebhooksOutput struct {
	Webhooks []WebhookInfo `json:"webhooks"`
	Total    int           `json:"total"`
}

type WebhookInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
}

type GetWebhookOutput struct {
	ID        string            `json:"id"`
	BucketID  string            `json:"bucket_id"`
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Events    []string          `json:"events"`
	Active    bool              `json:"active"`
	Headers   map[string]string `json:"headers"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type UpdateWebhookInput struct {
	Name    *string            `json:"name"`
	URL     *string            `json:"url"`
	Events  []string           `json:"events"`
	Active  *bool              `json:"active"`
	Headers map[string]string  `json:"headers"`
}

type TestWebhookOutput struct {
	Success      bool      `json:"success"`
	StatusCode   int       `json:"status_code"`
	Response     string    `json:"response"`
	ErrorMessage string    `json:"error_message,omitempty"`
	TestedAt     time.Time `json:"tested_at"`
}

type WebhookDeliveriesOutput struct {
	Deliveries []WebhookDeliveryInfo `json:"deliveries"`
	Total      int                   `json:"total"`
}

type WebhookDeliveryInfo struct {
	ID           string    `json:"id"`
	Event        string    `json:"event"`
	StatusCode   int       `json:"status_code"`
	Success      bool      `json:"success"`
	ErrorMessage string    `json:"error_message,omitempty"`
	DeliveredAt  time.Time `json:"delivered_at"`
}