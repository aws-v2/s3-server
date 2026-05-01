package http

import (
	"fmt"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"strconv"

	"github.com/gin-gonic/gin"
)

type PresignHandler struct {
	presignService *application.PresignService
	uploadService  *application.UploadService
}

func NewPresignHandler(presignService *application.PresignService, uploadService *application.UploadService) *PresignHandler {
	return &PresignHandler{
		presignService: presignService,
		uploadService:  uploadService,
	}
}

// HandleObjectUpload handles the actual object upload for presigned URLs
// PUT /api/v1/buckets/:bucket/objects/*key
func (h *PresignHandler) HandleObjectUpload(c *gin.Context) {
	bucketID := c.Param("bucketId")
	key := c.Param("key")

	// Get signing parameters from query
	urlID := c.Query("urlId")

	// Check if we have an authenticated actor (optional logging)
	actor, exists := c.Get("actor")
	if exists {
		log.Printf("[S3-PRESIGN] Actor found in context: %v. Proceeding with upload.", actor)
	}

	var validation *dto.ValidatePresignedURLOutput
	if urlID != "" {
		validateInput := dto.ValidatePresignedURLInput{
			URLID: urlID,
		}

		var err error
		validation, err = h.presignService.ValidatePresignedURL(c.Request.Context(), validateInput)
		if err != nil {
			log.Printf("[S3-PRESIGN] URL validation error (ignoring): %v", err)
		} else if !validation.Valid {
			log.Printf("[S3-PRESIGN] URL invalid (ignoring): %s", validation.Reason)
		}
	}

	// Determine bucket and key
	targetBucketID := bucketID
	targetKey := key

	if validation != nil && validation.Valid {
		targetBucketID = validation.BucketID
		targetKey = key // Use the key from the URL path as per S3 convention
		
		// Verify bucket matches validation (optional warning)
		if validation.BucketID != bucketID {
			log.Printf("[S3-PRESIGN] Warning: request bucketId %s does not match validation bucketId %s", bucketID, validation.BucketID)
		}
	}

	// Perform upload
	err := h.uploadService.UploadObjectReader(
		c.Request.Context(),
		targetBucketID,
		targetKey,
		c.Request.Body,
		c.Request.ContentLength,
		c.Request.Header.Get("Content-Type"),
		nil, // Metadata could be extracted from headers if needed
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "upload successful"})
}

// GenerateUploadURL handles generating presigned URL for upload
// POST /presign/:bucketId/upload
func (h *PresignHandler) GenerateUploadURL(c *gin.Context) {
	bucketId := c.Param("bucketId")
	
	var input dto.GenerateUploadURLInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId



	 


	output, err := h.presignService.GenerateUploadURL(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}




// GenerateDownloadURL handles generating presigned URL for download
// POST /presign/:bucketId/files/:fileId/download
func (h *PresignHandler) GenerateDownloadURL(c *gin.Context) {
	bucketId := c.Param("bucketId")
	fileId := c.Param("fileId")
	
	var input dto.GenerateDownloadURLInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId
	input.FileID = fileId

	output, err := h.presignService.GenerateDownloadURL(c.Request.Context(), input)
	if err != nil {
		fmt.Println( input.FileID)

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}












// RevokePresignedURL handles revoking a presigned URL
// DELETE /presign/urls/:urlId
func (h *PresignHandler) RevokePresignedURL(c *gin.Context) {
	urlId := c.Param("urlId")

	err := h.presignService.RevokePresignedURL(c.Request.Context(), urlId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "presigned URL revoked successfully"})
}

// ListPresignedURLs handles listing active presigned URLs
// GET /presign/urls
func (h *PresignHandler) ListPresignedURLs(c *gin.Context) {
	bucketId := c.Query("bucketId")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit parameter"})
		return
	}

	input := dto.ListPresignedURLsInput{
		BucketID: bucketId,
		Limit:    limit,
	}

	output, err := h.presignService.ListPresignedURLs(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}



// ValidatePresignedURL handles validating a presigned URL
// POST /presign/validate
func (h *PresignHandler) ValidatePresignedURL(c *gin.Context) {
	var input dto.ValidatePresignedURLInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.presignService.ValidatePresignedURL(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}


// GenerateMultipartUploadURLs handles generating presigned URLs for multipart upload
// POST /presign/:bucketId/multipart
func (h *PresignHandler) GenerateMultipartUploadURLs(c *gin.Context) {
	bucketId := c.Param("bucketId")
	
	var input dto.GenerateMultipartUploadURLsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId

	output, err := h.presignService.GenerateMultipartUploadURLs(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}



// CompleteMultipartUpload completes a multipart upload
func (h *PresignHandler) CompleteMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")

	var input dto.CompleteMultipartUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input.BucketID = bucketId
	input.UploadID = uploadId

	output, err := h.presignService.CompleteMultipartUpload(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// AbortMultipartUpload aborts a multipart upload
func (h *PresignHandler) AbortMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")

	input := dto.AbortMultipartUploadInput{
		BucketID: bucketId,
		UploadID: uploadId,
	}

	err := h.presignService.AbortMultipartUpload(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListMultipartUploadParts lists parts for a multipart upload
func (h *PresignHandler) ListMultipartUploadParts(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")

	input := dto.ListMultipartPartsInput{
		BucketID: bucketId,
		UploadID: uploadId,
	}

	output, err := h.presignService.ListMultipartUploadParts(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}