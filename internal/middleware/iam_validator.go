package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
)

// ValidationRequest matches the IAM service's ValidationRequestDTO
type ValidationRequest struct {
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
}

// ValidationResponse matches the IAM service's ValidationResponse
type ValidationResponse struct {
	Valid    bool     `json:"valid"`
	UserID   string   `json:"userId"`
	Policies []string `json:"policies"`
}

// IAMValidator validates credentials using the IAM service via NATS
type IAMValidator struct {
	natsConn *nats.Conn
	timeout  time.Duration
}

// NewIAMValidator creates a new IAM validator
func NewIAMValidator(natsConn *nats.Conn) *IAMValidator {
	return &IAMValidator{
		natsConn: natsConn,
		timeout:  5 * time.Second, // 5 second timeout for IAM requests
	}
}

// ValidateAPIKey validates an API key by communicating with the IAM service
func (v *IAMValidator) ValidateAPIKey(key string) (string, error) {
	// In AWS S3, the Authorization header typically contains both access key and secret
	// For this implementation, we'll parse the key to extract accessKeyId and secretAccessKey
	// Expected format: "accessKeyId:secretAccessKey"
	accessKeyID, secretAccessKey, err := parseAPIKey(key)
	if err != nil {
		return "", fmt.Errorf("invalid api key format: %w", err)
	}

	// Create validation request
	request := ValidationRequest{
		AccessKeyID:     accessKeyID,
		SecretAccessKey: secretAccessKey,
	}

	// Marshal request to JSON
	requestData, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal validation request: %w", err)
	}

	// Send request to IAM service and wait for response
	msg, err := v.natsConn.Request("iam.auth.validate", requestData, v.timeout)
	if err != nil {
		fmt.Printf("DEBUG: NATS request to iam.auth.validate failed: %v\n", err)
		return "", fmt.Errorf("failed to communicate with IAM service: %w", err)
	}

	// Parse response
	var response ValidationResponse
	if err := json.Unmarshal(msg.Data, &response); err != nil {
		fmt.Printf("DEBUG: Failed to parse IAM NATS response: %v, data: %s\n", err, string(msg.Data))
		return "", fmt.Errorf("failed to parse IAM response: %w", err)
	}

	// Check if credentials are valid
	if !response.Valid {
		fmt.Printf("DEBUG: IAM validation failed for key %s (Valid=false)\n", accessKeyID)
		return "", fmt.Errorf("invalid credentials")
	}

	fmt.Printf("DEBUG: IAM validation successful for userId: %s\n", response.UserID)
	// Return the user ID
	return response.UserID, nil
}

// parseAPIKey parses the API key to extract accessKeyId and secretAccessKey
// Expected format: "accessKeyId:secretAccessKey"
func parseAPIKey(key string) (string, string, error) {
	// Simple parsing - you may want to use a more sophisticated format
	// For example, AWS uses a specific format for access keys
	for i := 0; i < len(key); i++ {
		if key[i] == ':' {
			return key[:i], key[i+1:], nil
		}
	}
	return "", "", fmt.Errorf("invalid key format, expected 'accessKeyId:secretAccessKey'")
}
