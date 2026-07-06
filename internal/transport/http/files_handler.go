package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type HandlerForFiles struct {
	uploadService *application.UploadService
	deleteService *application.DeleteService
	secretKey     string
}

// Constructor for all file-related handlers
func NewFileHandler(
	uploadService *application.UploadService,
	deleteService *application.DeleteService,
	secretKey string,

) *HandlerForFiles {
	return &HandlerForFiles{
		uploadService: uploadService,
		deleteService: deleteService,
		secretKey:     secretKey,
	}
}

// UploadFile handles file upload
// POST /buckets/:bucketId/files
// UploadFile handles file upload
// POST /buckets/:bucketId/files
func (h *HandlerForFiles) UploadFile(c *gin.Context) {
	start := time.Now()

	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")
	userID := c.GetString("userId")

	log.Printf(
		"[UploadFile] started request_id=%s user_id=%s bucket_id=%s",
		requestID,
		userID,
		bucketID,
	)

	// Parse Multipart Form
	form, err := c.MultipartForm()
	if err != nil {
		log.Printf(
			"[UploadFile] multipart parse failed request_id=%s error=%v",
			requestID,
			err,
		)

		c.JSON(http.StatusBadRequest, gin.H{
			"error": "failed to parse multipart form: " + err.Error(),
		})
		return
	}

	// 1. Process Files
	files := form.File["files"]

	log.Printf(
		"[UploadFile] processing files request_id=%s file_count=%d",
		requestID,
		len(files),
	)

	var fileContents []application.FileContent

	for _, fileHeader := range files {
		log.Printf(
			"[UploadFile] reading file request_id=%s filename=%s size=%d type=%s",
			requestID,
			fileHeader.Filename,
			fileHeader.Size,
			fileHeader.Header.Get("Content-Type"),
		)

		file, err := fileHeader.Open()
		if err != nil {
			log.Printf(
				"[UploadFile] failed opening file request_id=%s filename=%s error=%v",
				requestID,
				fileHeader.Filename,
				err,
			)
			continue
		}

		data, err := io.ReadAll(file)
		file.Close()

		if err != nil {
			log.Printf(
				"[UploadFile] failed reading file request_id=%s filename=%s error=%v",
				requestID,
				fileHeader.Filename,
				err,
			)
			continue
		}

		fileContents = append(fileContents, application.FileContent{
			Name: fileHeader.Filename,
			Size: fileHeader.Size,
			Type: fileHeader.Header.Get("Content-Type"),
			Data: data,
		})
	}

	log.Printf(
		"[UploadFile] files loaded request_id=%s valid_files=%d",
		requestID,
		len(fileContents),
	)

	// Parse JSON fields from form
	var destSettings application.DestinationSettings
	if ds := c.PostForm("destinationSettings"); ds != "" {
		if err := json.Unmarshal([]byte(ds), &destSettings); err != nil {
			log.Printf(
				"[UploadFile] failed parsing destinationSettings request_id=%s error=%v",
				requestID,
				err,
			)
		}
	}

	var properties application.UploadProperties
	if p := c.PostForm("properties"); p != "" {
		if err := json.Unmarshal([]byte(p), &properties); err != nil {
			log.Printf(
				"[UploadFile] failed parsing properties request_id=%s error=%v",
				requestID,
				err,
			)
		}
	}

	var tags []application.Tag
	if t := c.PostForm("tags"); t != "" {
		if err := json.Unmarshal([]byte(t), &tags); err != nil {
			log.Printf(
				"[UploadFile] failed parsing tags request_id=%s error=%v",
				requestID,
				err,
			)
		}
	}

	var metadata []application.MetadataItem
	if m := c.PostForm("metadata"); m != "" {
		if err := json.Unmarshal([]byte(m), &metadata); err != nil {
			log.Printf(
				"[UploadFile] failed parsing metadata request_id=%s error=%v",
				requestID,
				err,
			)
		}
	}

	prefix := c.PostForm("prefix")

	input := application.UploadFileInput{
		BucketID:            bucketID,
		Prefix:              prefix,
		Files:               fileContents,
		DestinationSettings: destSettings,
		Properties:          properties,
		Tags:                tags,
		Metadata:            metadata,
	}

	log.Printf(
		"[UploadFile] calling upload service request_id=%s bucket_id=%s prefix=%s",
		requestID,
		bucketID,
		prefix,
	)

	output, err := h.uploadService.UploadFile(c.Request.Context(), input)
	if err != nil {
		log.Printf(
			"[UploadFile] upload failed request_id=%s error=%v duration=%s",
			requestID,
			err,
			time.Since(start),
		)

		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	log.Printf(
		"[UploadFile] completed request_id=%s uploaded_files=%d duration=%s",
		requestID,
		len(output.FileIDs),
		time.Since(start),
	)

	c.JSON(http.StatusCreated, output)
}

func (h *HandlerForFiles) UploadFilePresign(c *gin.Context) {
	start := time.Now()
	requestID := c.GetString("requestId")

	// 1. Extract token from query
	token := c.Query("t")
	if token == "" {
		log.Printf("[UploadFilePresign] missing token request_id=%s", requestID)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
		return
	}

	// 2. Split token into payload and signature
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		log.Printf("[UploadFilePresign] invalid token format request_id=%s", requestID)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token format"})
		return
	}

	payloadEncoded := parts[0]
	signature := parts[1]

	// 3. Verify signature
	expectedSignature := h.signString(payloadEncoded)
	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		log.Printf("[UploadFilePresign] invalid signature request_id=%s", requestID)
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return
	}

	// 4. Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		log.Printf("[UploadFilePresign] failed to decode payload request_id=%s error=%v", requestID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to decode token"})
		return
	}

	var payload struct {
		URLID     string `json:"u"`
		BucketID  string `json:"b"`
		Key       string `json:"k"`
		AssetID   string `json:"a"`
		UserID    string `json:"uid"`
		SHA256    string `json:"sha"`
		Method    string `json:"m"`
		ExpiresAt int64  `json:"e"`
	}

	
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Printf("[UploadFilePresign] failed to unmarshal payload request_id=%s error=%v", requestID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid token payload"})
		return
	}

	// 5. Check expiry
	
	if time.Now().Unix() > payload.ExpiresAt {
		log.Printf("[UploadFilePresign] token expired request_id=%s", requestID)
		c.JSON(http.StatusForbidden, gin.H{"error": "token has expired"})
		return
	}

	// 6. Check method
	if c.Request.Method != payload.Method {
		log.Printf("[UploadFilePresign] method mismatch request_id=%s expected=%s got=%s", requestID, payload.Method, c.Request.Method)
		c.JSON(http.StatusMethodNotAllowed, gin.H{"error": "method not allowed"})
		return
	}

	log.Printf(
		"[UploadFilePresign] valid token request_id=%s user_id=%s bucket_name=%s key=%s",
		requestID, payload.UserID, payload.BucketID, payload.Key,
	)

	// 7. Read raw body as file data
	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[UploadFilePresign] failed to read body request_id=%s error=%v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read upload data"})
		return
	}

	// 8. Call UploadService
	input := application.UploadFileInput{
		BucketID: payload.BucketID,
		Files: []application.FileContent{
			{
				Name: payload.Key,
				Size: int64(len(data)),
				Type: "application/octet-stream", // or sniff from headers
				Data: data,
			},
		},
		Metadata: []application.MetadataItem{
			{Key: "asset_id", Value: payload.AssetID},
			{Key: "original_sha256", Value: payload.SHA256},
		},
	}

	// Set userId in context for the service
	ctx := context.WithValue(c.Request.Context(), "userId", payload.UserID)

	output, err := h.uploadService.UploadFile(ctx, input)
	if err != nil {
		log.Printf("[UploadFilePresign] upload service failed request_id=%s error=%v", requestID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf(
		"[UploadFilePresign] upload completed request_id=%s file_id=%s duration=%s",
		requestID, output.FileIDs[0], time.Since(start),
	)

	c.JSON(http.StatusCreated, output)
}
// CreateFolder handles explicit folder creation (0-byte object)
// POST /buckets/:bucketId/folders
func (h *HandlerForFiles) CreateFolder(c *gin.Context) {
	bucketID := c.Param("bucketId")

	var input struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	if err := h.uploadService.CreateFolder(c.Request.Context(), bucketID, input.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "folder created successfully"})
}

// ListFiles handles listing files in a bucket
// GET /buckets/:bucketId/files
func (h *HandlerForFiles) ListFiles(c *gin.Context) {
	bucketID := c.Param("bucketId")

	files, err := h.uploadService.ListFiles(c.Request.Context(), bucketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bucketId": bucketID,
		"count":    len(files),
		"files":    files,
	})
}

// DeleteFile handles file deletion
// DELETE /buckets/:bucketId/files/:fileId
func (h *HandlerForFiles) DeleteFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	err := h.deleteService.DeleteFile(c.Request.Context(), application.DeleteFileInput{
		FileID:   fileID,
		BucketID: bucketID,
	})
	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file deleted successfully",
	})
}

// GetFileInfo handles getting file metadata
// GET /:bucketId/files/:fileId
func (h *HandlerForFiles) GetFileInfo(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	output, err := h.uploadService.GetFileInfo(c.Request.Context(), bucketID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *HandlerForFiles) signString(data string) string {
	mac := hmac.New(sha256.New, []byte(h.secretKey))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// func (h *HandlerForFiles) ValidateSignature(key, method, signature string, expiresAt int64) error {
// 	// Check expiry first — cheap check before crypto
// 	if time.Now().Unix() > expiresAt {
// 		return fmt.Errorf("presigned URL has expired")
// 	}

// 	expected := h.signString(key + method + fmt.Sprintf("%d", expiresAt))

// 	// Constant-time comparison to prevent timing attacks
// 	if !hmac.Equal([]byte(expected), []byte(signature)) {
// 		return fmt.Errorf("invalid signature")
// 	}

// 	return nil
// }

type SignedPayload struct {
	A string `json:"a"` // asset_id
	B string `json:"b"` // bucket_id
	E int64  `json:"e"` // expires_at
	K string `json:"k"` // key
	M string `json:"m"` // method
	S string `json:"s"` // sha256
	U string `json:"u"` // user_id
	UId string `json:"uid"` // user_id
}
// DownloadFile handles file download with presigned URL signature validation.
// GET /:bucketId/files/:fileId/download?signature=xxx&expires=unix

func (h *HandlerForFiles) DownloadFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")
	userID := c.GetString("userId")

	token := c.Query("t")

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "missing token",
		})
		return
	}

	payload, err := h.ValidateSignature(token, http.MethodGet)
	if err != nil {
		c.JSON(http.StatusForbidden, gin.H{
			"error": err.Error(),
		})
		return
	}

	_ = payload

	fileData, metadata, err := h.uploadService.DownloadFile(
		c.Request.Context(),
		bucketID,
		fileID,
		userID,
	)

	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.Header(
		"Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s\"", metadata.Key),
	)

	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", metadata.Size))

	c.Data(http.StatusOK, metadata.MimeType, fileData)
}
 

func (h *HandlerForFiles) ValidateSignature(
	token string,
	expectedMethod string,
) (*SignedPayload, error) {

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}

	payloadEncoded := parts[0]
	providedSig := parts[1]

	// Recompute signature
	expectedSig := h.signString(payloadEncoded)

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return nil, errors.New("invalid signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}

	var payload SignedPayload

	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, errors.New("invalid payload")
	}

	// Expiry check
	if time.Now().Unix() > payload.E {
		return nil, errors.New("token expired")
	}

	// Method check
	if payload.M != expectedMethod {
		return nil, errors.New("invalid method")
	}

	return &payload, nil
}

// UpdateFileMetadata handles updating file metadata
// PATCH /:bucketId/files/:fileId
func (h *HandlerForFiles) UpdateFileMetadata(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	var input dto.UpdateFileMetadataInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.uploadService.UpdateFileMetadata(c.Request.Context(), bucketID, fileID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// CopyFile handles copying a file
// POST /:bucketId/files/:fileId/copy
func (h *HandlerForFiles) CopyFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	var input dto.CopyFileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.uploadService.CopyFile(c.Request.Context(), bucketID, fileID, input)

	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, output)
}

// MoveFile handles moving a file to another bucket
// POST /:bucketId/files/:fileId/move
func (h *HandlerForFiles) MoveFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	var input dto.MoveFileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.uploadService.MoveFile(c.Request.Context(), bucketID, fileID, input)
	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}
