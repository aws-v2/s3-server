package metrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"sync"
	"time"
)

// MetricsClient handles communication with the central metrics server.
type MetricsClient struct {
	baseURL       string
	httpClient    *http.Client
	apiKey        string
	tokenProvider domain.TokenProvider
	instanceID    string
	tokens        map[string]cachedToken
	mu            sync.RWMutex
}

type cachedToken struct {
	token  string
	expiry time.Time
}

// NewMetricsClient creates a new instance of MetricsClient.
func NewMetricsClient(tp domain.TokenProvider, instanceID string) *MetricsClient {
	baseURL := os.Getenv("METRICS_SERVER_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8099/api/v1/metrics-server" // Fallback default
	}

	return &MetricsClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		apiKey:        os.Getenv("METRICS_SERVER_KEY"),
		tokenProvider: tp,
		instanceID:    instanceID,
		tokens:        make(map[string]cachedToken),
	}
}

// SendS3Metrics sends a metrics snapshot to the metrics server.
func (c *MetricsClient) SendS3Metrics(ctx context.Context, req dto.S3IngestRequest) error {
	url := fmt.Sprintf("%s/s3/ingest", c.baseURL)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}


	fmt.Println("jsonData", string(jsonData))
	fmt.Println("9999999999999999999999999999999999999url", url)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create metrics request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		httpReq.Header.Set("X-API-Key", c.apiKey)
	}

	// Dynamic token authentication with cache
	if c.tokenProvider != nil {
		token := c.getValidToken(req.OwnerID)
		if token == "" {
			var err error
			token, err = c.tokenProvider.RequestInstanceToken(ctx, req.OwnerID, c.instanceID)

			if err == nil && token != "" {
				c.setCachedToken(req.OwnerID, token)

 



			} else {
				log.Printf("[MetricsClient] Warning: failed to get auth token: %v", err)
			}
		}

		if token != "" {
			httpReq.Header.Set("Authorization", "Bearer "+token)
		}
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send metrics to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("metrics server returned error: %s", resp.Status)
	}

	log.Printf("[MetricsClient] [SUCCESS] Metrics sent for bucket: %s (Owner: %s)", req.BucketID, req.OwnerID)
	return nil
}

func (c *MetricsClient) getValidToken(ownerID string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.tokens[ownerID]
	if !exists || time.Now().After(cached.expiry) {
		return ""
	}
	return cached.token
}

func (c *MetricsClient) setCachedToken(ownerID, token string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tokens[ownerID] = cachedToken{
		token:  token,
		expiry: time.Now().Add(5 * time.Minute), // Cache for 5 minutes
	}
}
