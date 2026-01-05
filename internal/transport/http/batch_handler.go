package http

import (
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"

	"github.com/gin-gonic/gin"
)

type BatchHandler struct {
	batchService *application.BatchService
}

func NewBatchHandler(batchService *application.BatchService) *BatchHandler {
	return &BatchHandler{
		batchService: batchService,
	}
}

// BatchUpload handles batch file upload
// POST /batch/upload
func (h *BatchHandler) BatchUpload(c *gin.Context) {
	var input dto.BatchUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.batchService.BatchUpload(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, output)
}

// BatchDelete handles batch file deletion
// DELETE /batch/delete
func (h *BatchHandler) BatchDelete(c *gin.Context) {
	var input dto.BatchDeleteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.batchService.BatchDelete(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, output)
}

// BatchCopy handles batch file copy
// POST /batch/copy
func (h *BatchHandler) BatchCopy(c *gin.Context) {
	var input dto.BatchCopyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.batchService.BatchCopy(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, output)
}

// BatchMove handles batch file move
// POST /batch/move
func (h *BatchHandler) BatchMove(c *gin.Context) {
	var input dto.BatchMoveInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.batchService.BatchMove(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, output)
}

// BatchUpdateMetadata handles batch metadata update
// PATCH /batch/metadata
func (h *BatchHandler) BatchUpdateMetadata(c *gin.Context) {
	var input dto.BatchUpdateMetadataInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.batchService.BatchUpdateMetadata(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, output)
}

// GetBatchOperationStatus gets status of a batch operation
// GET /batch/operations/:operationId
func (h *BatchHandler) GetBatchOperationStatus(c *gin.Context) {
	operationId := c.Param("operationId")

	output, err := h.batchService.GetBatchOperationStatus(c.Request.Context(), operationId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// ListBatchOperations lists batch operations
// GET /batch/operations
func (h *BatchHandler) ListBatchOperations(c *gin.Context) {
	var input dto.ListBatchOperationsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}

	output, err := h.batchService.ListBatchOperations(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// CancelBatchOperation cancels a batch operation
// DELETE /batch/operations/:operationId
func (h *BatchHandler) CancelBatchOperation(c *gin.Context) {
	operationId := c.Param("operationId")

	err := h.batchService.CancelBatchOperation(c.Request.Context(), operationId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "batch operation cancelled successfully"})
}