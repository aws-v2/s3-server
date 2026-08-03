package http

import (
	"fmt"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"s3/internal/utils"
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
	requestID := c.GetString("requestId")

	// Get signing parameters from query
	urlID := c.Query("urlId")
	// Note: In a real implementation, we would extract all params and re-sign to verify,
	// but here we'll use PresignService.ValidatePresignedURL as a simplified check.
	// The PresignService.ValidatePresignedURL in this codebase checks the DB for urlId.

	if urlID == "" {
		log.Printf("[Handler:HandleObjectUpload] Bad request, requestID %s error missing urlId", requestID)
		utils.RespondError(c, http.StatusUnauthorized, fmt.Errorf("missing or invalid presigned URL"))
		return
	}

	validateInput := dto.ValidatePresignedURLInput{
		URLID: urlID,
	}

	validation, err := h.presignService.ValidatePresignedURL(c.Request.Context(), validateInput)
	if err != nil || !validation.Valid {
		log.Printf("[Handler:HandleObjectUpload] Service call, requestID %s error %s", requestID, err.Error())
		reason := "invalid or expired presigned URL"
		if validation != nil && validation.Reason != "" {
			reason = validation.Reason
		}
		utils.RespondError(c, http.StatusUnauthorized, fmt.Errorf(reason))
		return
	}

	// Verify bucket matches validation
	if validation.BucketID != bucketID {
		log.Printf("[S3-PRESIGN] Warning: request bucketId %s does not match validation bucketId %s", bucketID, validation.BucketID)
	}

	// Perform upload
	err = h.uploadService.UploadObjectReader(
		c.Request.Context(),
		validation.BucketID,
		key,
		c.Request.Body,
		c.Request.ContentLength,
		c.Request.Header.Get("Content-Type"),
		nil, // Metadata could be extracted from headers if needed
	)

	if err != nil {
		log.Printf("[Handler:HandleObjectUpload] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to upload object"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "object uploaded successfully", gin.H{"key": key, "status": "uploaded"})
}

// GenerateUploadURL handles generating presigned URL for upload
// POST /presign/:bucketId/upload
func (h *PresignHandler) GenerateUploadURL(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")
	
	var input dto.GenerateUploadURLInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:GenerateUploadURL] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId

	output, err := h.presignService.GenerateUploadURL(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GenerateUploadURL] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate upload URL"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "upload URL generated successfully", output)
}




// GenerateDownloadURL handles generating presigned URL for download
// POST /presign/:bucketId/files/:fileId/download
func (h *PresignHandler) GenerateDownloadURL(c *gin.Context) {
	bucketId := c.Param("bucketId")
	fileId := c.Param("fileId")
	requestID := c.GetString("requestId")
	
	var input dto.GenerateDownloadURLInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:GenerateDownloadURL] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId
	input.FileID = fileId

	output, err := h.presignService.GenerateDownloadURL(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GenerateDownloadURL] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate download URL"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "download URL generated successfully", output)
}












// RevokePresignedURL handles revoking a presigned URL
// DELETE /presign/urls/:urlId
func (h *PresignHandler) RevokePresignedURL(c *gin.Context) {
	urlId := c.Param("urlId")
	requestID := c.GetString("requestId")

	err := h.presignService.RevokePresignedURL(c.Request.Context(), urlId)
	if err != nil {
		log.Printf("[Handler:RevokePresignedURL] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to revoke presigned URL"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "presigned URL revoked successfully", gin.H{"urlId": urlId, "status": "revoked"})
}

// ListPresignedURLs handles listing active presigned URLs
// GET /presign/urls
func (h *PresignHandler) ListPresignedURLs(c *gin.Context) {
	bucketId := c.Query("bucketId")
	limitStr := c.DefaultQuery("limit", "100")
	requestID := c.GetString("requestId")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		log.Printf("[Handler:ListPresignedURLs] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid limit parameter"))
		return
	}

	input := dto.ListPresignedURLsInput{
		BucketID: bucketId,
		Limit:    limit,
	}

	output, err := h.presignService.ListPresignedURLs(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ListPresignedURLs] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list presigned URLs"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "presigned URLs listed successfully", output)
}



// ValidatePresignedURL handles validating a presigned URL
// POST /presign/validate
func (h *PresignHandler) ValidatePresignedURL(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.ValidatePresignedURLInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:ValidatePresignedURL] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.presignService.ValidatePresignedURL(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ValidatePresignedURL] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to validate presigned URL"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "presigned URL validated successfully", output)
}


// GenerateMultipartUploadURLs handles generating presigned URLs for multipart upload
// POST /presign/:bucketId/multipart
func (h *PresignHandler) GenerateMultipartUploadURLs(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")
	
	var input dto.GenerateMultipartUploadURLsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:GenerateMultipartUploadURLs] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId

	output, err := h.presignService.GenerateMultipartUploadURLs(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GenerateMultipartUploadURLs] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to generate multipart upload URLs"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "multipart upload URLs generated successfully", output)
}



// CompleteMultipartUpload completes a multipart upload
func (h *PresignHandler) CompleteMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")
	requestID := c.GetString("requestId")

	var input dto.CompleteMultipartUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CompleteMultipartUpload] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	input.BucketID = bucketId
	input.UploadID = uploadId

	output, err := h.presignService.CompleteMultipartUpload(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:CompleteMultipartUpload] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to complete multipart upload"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "multipart upload completed successfully", output)
}

// AbortMultipartUpload aborts a multipart upload
func (h *PresignHandler) AbortMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")
	requestID := c.GetString("requestId")

	input := dto.AbortMultipartUploadInput{
		BucketID: bucketId,
		UploadID: uploadId,
	}

	err := h.presignService.AbortMultipartUpload(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:AbortMultipartUpload] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to abort multipart upload"))
		return
	}

	utils.RespondSucces(c, http.StatusNoContent, "multipart upload aborted", gin.H{"uploadId": uploadId, "status": "aborted"})
}

// ListMultipartUploadParts lists parts for a multipart upload
func (h *PresignHandler) ListMultipartUploadParts(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")
	requestID := c.GetString("requestId")

	input := dto.ListMultipartPartsInput{
		BucketID: bucketId,
		UploadID: uploadId,
	}

	output, err := h.presignService.ListMultipartUploadParts(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ListMultipartUploadParts] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list multipart parts"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "multipart parts listed successfully", output)
}
