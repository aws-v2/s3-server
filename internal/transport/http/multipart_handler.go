package http

import (
	"io"
	"net/http"

	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"strconv"

	"github.com/gin-gonic/gin"
)

type MultipartHandler struct {
	multipartService *application.MultipartService
}

func NewMultipartHandler(multipartService *application.MultipartService) *MultipartHandler {
	return &MultipartHandler{multipartService: multipartService}
}

func (h *MultipartHandler) InitiateMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.InitiateMultipartUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId

	output, err := h.multipartService.InitiateMultipartUpload(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *MultipartHandler) UploadPart(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")
	partNumber, _ := strconv.Atoi(c.Param("partNumber"))

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	input := dto.UploadPartInput{
		BucketID:   bucketId,
		UploadID:   uploadId,
		PartNumber: partNumber,
		Data:       data,
	}

	output, err := h.multipartService.UploadPart(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *MultipartHandler) CompleteMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")

	var input dto.CompleteMultipartUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId
	input.UploadID = uploadId

	output, err := h.multipartService.CompleteMultipartUpload(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *MultipartHandler) AbortMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")

	err := h.multipartService.AbortMultipartUpload(c.Request.Context(), bucketId, uploadId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "multipart upload aborted"})
}

func (h *MultipartHandler) ListParts(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")

	output, err := h.multipartService.ListParts(c.Request.Context(), bucketId, uploadId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *MultipartHandler) ListMultipartUploads(c *gin.Context) {
	bucketId := c.Param("bucketId")

	output, err := h.multipartService.ListMultipartUploads(c.Request.Context(), bucketId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}