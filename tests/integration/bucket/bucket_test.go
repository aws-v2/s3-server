package bucket_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	baseURL = "http://localhost:8082/api/v1/s3"
	jwt     = "eyJhbGciOiJIUzUxMiJ9.eyJ1c2VySWQiOiI5NzQ2OWVhNS04YWI0LTQ4YjAtYjlkNi0yN2NkMmZkZGFlNmYiLCJzdWIiOiJlbXFhcmFuaTFAZ21haWwuY29tIiwiaWF0IjoxNzczMjkyMTkwLCJleHAiOjE3NzMzNzg1OTB9.tjL7GjorcL29fEGIJBU9IubsEY3RRTHtsIyPM0MAPCtkZe_4eLC7ddUQfwX13OLFecBobVTmFIQpcT1Vui2IrA"
)

func TestBucketLifecycle(t *testing.T) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	bucketName := fmt.Sprintf("it-bucket-%d", time.Now().Unix())
	var createdBucketID string

	// 1. Create Bucket
	t.Run("CreateBucket", func(t *testing.T) {
		payload := map[string]interface{}{
			"name":   bucketName,
			"region": "us-east-1",
		}
		jsonPayload, _ := json.Marshal(payload)

		req, _ := http.NewRequest("POST", baseURL+"/buckets/create-bucket", bytes.NewBuffer(jsonPayload))
		req.Header.Set("Authorization", "Bearer "+jwt)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		assert.Contains(t, []int{http.StatusOK, http.StatusCreated}, resp.StatusCode, string(body))

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		createdBucketID = result["bucket_id"].(string)
		assert.NotEmpty(t, createdBucketID)
	})

	// 2. List Buckets
	t.Run("ListBuckets", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/buckets", nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		assert.Contains(t, string(body), bucketName)
	})

	// 3. Get Bucket Info
	t.Run("GetBucket", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/buckets/"+createdBucketID, nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(body))
		assert.Contains(t, string(body), bucketName)
	})

	// 4. Delete Bucket
	t.Run("DeleteBucket", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/buckets/"+createdBucketID, nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}
