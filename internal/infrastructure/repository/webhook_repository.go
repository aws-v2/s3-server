package repository

import (
	"context"
	"encoding/json"
	"s3/internal/domain"
)

func (r *PostgresRepository) SaveWebhook(ctx context.Context, webhook *domain.Webhook) error {
	eventsJSON, _ := json.Marshal(webhook.Events)
	headersJSON, _ := json.Marshal(webhook.Headers)

	query := `INSERT INTO webhooks (id, bucket_id, name, url, events, secret, active, headers, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	_, err := r.db.ExecContext(ctx, query, webhook.ID, webhook.BucketID, webhook.Name,
		webhook.URL, eventsJSON, webhook.Secret, webhook.Active, headersJSON,
		webhook.CreatedAt, webhook.UpdatedAt)
	return err
}

func (r *PostgresRepository) GetWebhookByID(ctx context.Context, id string) (*domain.Webhook, error) {
	query := `SELECT id, bucket_id, name, url, events, secret, active, headers, created_at, updated_at
		FROM webhooks WHERE id = $1`

	var webhook domain.Webhook
	var eventsJSON, headersJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&webhook.ID, &webhook.BucketID, &webhook.Name, &webhook.URL,
		&eventsJSON, &webhook.Secret, &webhook.Active, &headersJSON,
		&webhook.CreatedAt, &webhook.UpdatedAt)

	if err != nil {
		return nil, err
	}

	if len(eventsJSON) > 0 {
		json.Unmarshal(eventsJSON, &webhook.Events)
	}
	if len(headersJSON) > 0 {
		json.Unmarshal(headersJSON, &webhook.Headers)
	}
	return &webhook, nil
}

func (r *PostgresRepository) ListWebhooksByBucket(ctx context.Context, bucketID string) ([]domain.Webhook, error) {
	query := `SELECT id, bucket_id, name, url, events, secret, active, headers, created_at, updated_at
		FROM webhooks WHERE bucket_id = $1 ORDER BY created_at DESC`

	rows, err := r.db.QueryContext(ctx, query, bucketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	webhooks := []domain.Webhook{}
	for rows.Next() {
		var wh domain.Webhook
		var eventsJSON, headersJSON []byte

		rows.Scan(&wh.ID, &wh.BucketID, &wh.Name, &wh.URL, &eventsJSON,
			&wh.Secret, &wh.Active, &headersJSON, &wh.CreatedAt, &wh.UpdatedAt)

		if len(eventsJSON) > 0 {
			json.Unmarshal(eventsJSON, &wh.Events)
		}
		if len(headersJSON) > 0 {
			json.Unmarshal(headersJSON, &wh.Headers)
		}
		webhooks = append(webhooks, wh)
	}
	return webhooks, nil
}

func (r *PostgresRepository) UpdateWebhook(ctx context.Context, webhook *domain.Webhook) error {
	eventsJSON, _ := json.Marshal(webhook.Events)
	headersJSON, _ := json.Marshal(webhook.Headers)

	query := `UPDATE webhooks SET name=$2, url=$3, events=$4, active=$5, headers=$6, updated_at=$7 WHERE id=$1`

	_, err := r.db.ExecContext(ctx, query, webhook.ID, webhook.Name, webhook.URL,
		eventsJSON, webhook.Active, headersJSON, webhook.UpdatedAt)
	return err
}

func (r *PostgresRepository) DeleteWebhook(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM webhooks WHERE id=$1", id)
	return err
}

func (r *PostgresRepository) SaveWebhookDelivery(ctx context.Context, delivery *domain.WebhookDelivery) error {
	query := `INSERT INTO webhook_deliveries (id, webhook_id, event, payload, status_code, response, success, error_message, delivered_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query, delivery.ID, delivery.WebhookID, delivery.Event,
		delivery.Payload, delivery.StatusCode, delivery.Response, delivery.Success,
		delivery.ErrorMessage, delivery.DeliveredAt)
	return err
}

func (r *PostgresRepository) ListWebhookDeliveries(ctx context.Context, webhookID string, limit int) ([]domain.WebhookDelivery, error) {
	query := `SELECT id, webhook_id, event, payload, status_code, response, success, error_message, delivered_at
		FROM webhook_deliveries WHERE webhook_id=$1 ORDER BY delivered_at DESC LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, webhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := []domain.WebhookDelivery{}
	for rows.Next() {
		var d domain.WebhookDelivery
		rows.Scan(&d.ID, &d.WebhookID, &d.Event, &d.Payload, &d.StatusCode,
			&d.Response, &d.Success, &d.ErrorMessage, &d.DeliveredAt)
		deliveries = append(deliveries, d)
	}
	return deliveries, nil
}
