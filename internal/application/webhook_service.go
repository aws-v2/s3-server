package application

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"time"

	"github.com/google/uuid"
)

type WebhookService struct {
	repo domain.RepositoryPort
}

func NewWebhookService(repo domain.RepositoryPort) *WebhookService {
	return &WebhookService{repo: repo}
}

func (s *WebhookService) CreateWebhook(ctx context.Context, input dto.CreateWebhookInput) (*dto.CreateWebhookOutput, error) {
	secret := input.Secret
	if secret == "" {
		secret = generateSecret()
	}

	webhook := &domain.Webhook{
		ID:        uuid.New().String(),
		BucketID:  input.BucketID,
		Name:      input.Name,
		URL:       input.URL,
		Events:    input.Events,
		Secret:    secret,
		Active:    true,
		Headers:   input.Headers,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.SaveWebhook(ctx, webhook); err != nil {
		return nil, fmt.Errorf("failed to create webhook: %w", err)
	}

	return &dto.CreateWebhookOutput{
		ID:        webhook.ID,
		Name:      webhook.Name,
		URL:       webhook.URL,
		Secret:    webhook.Secret,
		CreatedAt: webhook.CreatedAt,
	}, nil
}

func (s *WebhookService) ListWebhooks(ctx context.Context, bucketID string) (*dto.ListWebhooksOutput, error) {
	webhooks, err := s.repo.ListWebhooksByBucket(ctx, bucketID)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	webhookInfos := make([]dto.WebhookInfo, len(webhooks))
	for i, wh := range webhooks {
		webhookInfos[i] = dto.WebhookInfo{
			ID:        wh.ID,
			Name:      wh.Name,
			URL:       wh.URL,
			Events:    wh.Events,
			Active:    wh.Active,
			CreatedAt: wh.CreatedAt,
		}
	}

	return &dto.ListWebhooksOutput{
		Webhooks: webhookInfos,
		Total:    len(webhooks),
	}, nil
}

func (s *WebhookService) GetWebhook(ctx context.Context, webhookID string) (*dto.GetWebhookOutput, error) {
	webhook, err := s.repo.GetWebhookByID(ctx, webhookID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found: %w", err)
	}

	return &dto.GetWebhookOutput{
		ID:        webhook.ID,
		BucketID:  webhook.BucketID,
		Name:      webhook.Name,
		URL:       webhook.URL,
		Events:    webhook.Events,
		Active:    webhook.Active,
		Headers:   webhook.Headers,
		CreatedAt: webhook.CreatedAt,
		UpdatedAt: webhook.UpdatedAt,
	}, nil
}

func (s *WebhookService) UpdateWebhook(ctx context.Context, webhookID string, input dto.UpdateWebhookInput) error {
	webhook, err := s.repo.GetWebhookByID(ctx, webhookID)
	if err != nil {
		return fmt.Errorf("webhook not found: %w", err)
	}

	if input.Name != nil {
		webhook.Name = *input.Name
	}
	if input.URL != nil {
		webhook.URL = *input.URL
	}
	if input.Events != nil {
		webhook.Events = input.Events
	}
	if input.Active != nil {
		webhook.Active = *input.Active
	}
	if input.Headers != nil {
		webhook.Headers = input.Headers
	}

	webhook.UpdatedAt = time.Now()

	if err := s.repo.UpdateWebhook(ctx, webhook); err != nil {
		return fmt.Errorf("failed to update webhook: %w", err)
	}

	return nil
}

func (s *WebhookService) DeleteWebhook(ctx context.Context, webhookID string) error {
	if err := s.repo.DeleteWebhook(ctx, webhookID); err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	return nil
}

func (s *WebhookService) TestWebhook(ctx context.Context, webhookID string) (*dto.TestWebhookOutput, error) {
	webhook, err := s.repo.GetWebhookByID(ctx, webhookID)
	if err != nil {
		return nil, fmt.Errorf("webhook not found: %w", err)
	}

	testPayload := map[string]interface{}{
		"event": "webhook.test",
		"timestamp": time.Now().Unix(),
		"data": map[string]string{
			"message": "This is a test webhook delivery",
		},
	}

	delivery, err := s.deliverWebhook(ctx, webhook, "webhook.test", testPayload)
	if err != nil {
		return &dto.TestWebhookOutput{
			Success:      false,
			ErrorMessage: err.Error(),
			TestedAt:     time.Now(),
		}, nil
	}

	return &dto.TestWebhookOutput{
		Success:    delivery.Success,
		StatusCode: delivery.StatusCode,
		Response:   delivery.Response,
		TestedAt:   delivery.DeliveredAt,
	}, nil
}

func (s *WebhookService) GetWebhookDeliveries(ctx context.Context, webhookID string) (*dto.WebhookDeliveriesOutput, error) {
	deliveries, err := s.repo.ListWebhookDeliveries(ctx, webhookID, 50)
	if err != nil {
		return nil, fmt.Errorf("failed to get deliveries: %w", err)
	}

	deliveryInfos := make([]dto.WebhookDeliveryInfo, len(deliveries))
	for i, d := range deliveries {
		deliveryInfos[i] = dto.WebhookDeliveryInfo{
			ID:           d.ID,
			Event:        d.Event,
			StatusCode:   d.StatusCode,
			Success:      d.Success,
			ErrorMessage: d.ErrorMessage,
			DeliveredAt:  d.DeliveredAt,
		}
	}

	return &dto.WebhookDeliveriesOutput{
		Deliveries: deliveryInfos,
		Total:      len(deliveries),
	}, nil
}

func (s *WebhookService) TriggerWebhook(ctx context.Context, bucketID, event string, payload interface{}) {
	webhooks, err := s.repo.ListWebhooksByBucket(ctx, bucketID)
	if err != nil {
		return
	}

	for _, webhook := range webhooks {
		if !webhook.Active {
			continue
		}

		if !containsEvent(webhook.Events, event) {
			continue
		}

		go func(wh domain.Webhook) {
			s.deliverWebhook(context.Background(), &wh, event, payload)
		}(webhook)
	}
}

func (s *WebhookService) deliverWebhook(ctx context.Context, webhook *domain.Webhook, event string, payload interface{}) (*domain.WebhookDelivery, error) {
	payloadBytes, _ := json.Marshal(payload)
	payloadStr := string(payloadBytes)

	signature := generateSignature(webhook.Secret, payloadBytes)

	req, err := http.NewRequestWithContext(ctx, "POST", webhook.URL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return s.saveFailedDelivery(ctx, webhook.ID, event, payloadStr, err.Error()), err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Signature", signature)
	req.Header.Set("X-Webhook-Event", event)

	for k, v := range webhook.Headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return s.saveFailedDelivery(ctx, webhook.ID, event, payloadStr, err.Error()), err
	}
	defer resp.Body.Close()

	responseBody, _ := io.ReadAll(resp.Body)

	delivery := &domain.WebhookDelivery{
		ID:          uuid.New().String(),
		WebhookID:   webhook.ID,
		Event:       event,
		Payload:     payloadStr,
		StatusCode:  resp.StatusCode,
		Response:    string(responseBody),
		Success:     resp.StatusCode >= 200 && resp.StatusCode < 300,
		DeliveredAt: time.Now(),
	}

	if !delivery.Success {
		delivery.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}

	s.repo.SaveWebhookDelivery(ctx, delivery)
	return delivery, nil
}

func (s *WebhookService) saveFailedDelivery(ctx context.Context, webhookID, event, payload, errorMsg string) *domain.WebhookDelivery {
	delivery := &domain.WebhookDelivery{
		ID:           uuid.New().String(),
		WebhookID:    webhookID,
		Event:        event,
		Payload:      payload,
		Success:      false,
		ErrorMessage: errorMsg,
		DeliveredAt:  time.Now(),
	}
	s.repo.SaveWebhookDelivery(ctx, delivery)
	return delivery
}

func generateSecret() string {
	return uuid.New().String()
}

func generateSignature(secret string, payload []byte) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

func containsEvent(events []string, event string) bool {
	for _, e := range events {
		if e == event {
			return true
		}
	}
	return false
}