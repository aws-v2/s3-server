package file_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	baseURL = "http://localhost:8083/api/v1/s3"
	jwt     = "eyJhbGciOiJIUzUxMiJ9.eyJ1c2VySWQiOiI5NzQ2OWVhNS04YWI0LTQ4YjAtYjlkNi0yN2NkMmZkZGFlNmYiLCJzdWIiOiJlbXFhcmFuaTFAZ21haWwuY29tIiwiaWF0IjoxNzczMjkyMTkwLCJleHAiOjE3NzMzNzg1OTB9.tjL7GjorcL29fEGIJBU9IubsEY3RRTHtsIyPM0MAPCtkZe_4eLC7ddUQfwX13OLFecBobVTmFIQpcT1Vui2IrA"
)

func TestFileLifecycle(t *testing.T) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	bucketName := fmt.Sprintf("it-file-bucket-%d", time.Now().Unix())
	var bucketID string
	var fileID string

	// 1. Setup: Create Bucket
	t.Run("SetupBucket", func(t *testing.T) {
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
		assert.Equal(t, http.StatusCreated, resp.StatusCode, string(body))

		var result map[string]interface{}
		json.Unmarshal(body, &result)
		bucketID = result["bucket_id"].(string)
	})

	// 2. Upload File
	t.Run("UploadFile", func(t *testing.T) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		
		part, _ := writer.CreateFormFile("files", "hello.txt")
		part.Write([]byte("hello world integration test"))
		
		writer.WriteField("prefix", "test-prefix")
		writer.Close()

		req, _ := http.NewRequest("POST", baseURL+"/files/upload/"+bucketID, body)
		req.Header.Set("Authorization", "Bearer "+jwt)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusCreated, resp.StatusCode, string(respBody))

		var result map[string]interface{}
		json.Unmarshal(respBody, &result)
		fileIDs := result["FileIDs"].([]interface{})
		assert.Len(t, fileIDs, 1)
		fileID = fileIDs[0].(string)
	})

	// 3. List Files
	t.Run("ListFiles", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/files/"+bucketID, nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))
		assert.Contains(t, string(respBody), "hello.txt")
	})

	// 4. Get File Info
	t.Run("GetFileInfo", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/files/"+bucketID+"/files/"+fileID, nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))
	})

	// 5. Download File
	t.Run("DownloadFile", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/files/"+bucketID+"/files/"+fileID+"/download", nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		respBody, _ := io.ReadAll(resp.Body)
		assert.Equal(t, http.StatusOK, resp.StatusCode, string(respBody))
		assert.Equal(t, "hello world integration test", string(respBody))
	})

	// 6. Delete File
	t.Run("DeleteFile", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/files/"+bucketID+"/files/"+fileID, nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// 7. Cleanup: Delete Bucket
	t.Run("CleanupBucket", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", baseURL+"/buckets/"+bucketID, nil)
		req.Header.Set("Authorization", "Bearer "+jwt)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}
