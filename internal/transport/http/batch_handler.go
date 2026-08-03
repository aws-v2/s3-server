package http

import (
	"fmt"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"s3/internal/utils"

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
	requestID := c.GetString("requestId")
	var input dto.BatchUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:BatchUpload] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.batchService.BatchUpload(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:BatchUpload] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("batch upload failed"))
		return
	}

	utils.RespondSucces(c, http.StatusAccepted, "batch upload initiated", output)
}

// BatchDelete handles batch file deletion
// DELETE /batch/delete
func (h *BatchHandler) BatchDelete(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.BatchDeleteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:BatchDelete] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.batchService.BatchDelete(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:BatchDelete] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("batch delete failed"))
		return
	}

	utils.RespondSucces(c, http.StatusAccepted, "batch delete initiated", output)
}

// BatchCopy handles batch file copy
// POST /batch/copy
func (h *BatchHandler) BatchCopy(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.BatchCopyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:BatchCopy] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.batchService.BatchCopy(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:BatchCopy] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("batch copy failed"))
		return
	}

	utils.RespondSucces(c, http.StatusAccepted, "batch copy initiated", output)
}

// BatchMove handles batch file move
// POST /batch/move
func (h *BatchHandler) BatchMove(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.BatchMoveInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:BatchMove] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.batchService.BatchMove(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:BatchMove] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("batch move failed"))
		return
	}

	utils.RespondSucces(c, http.StatusAccepted, "batch move initiated", output)
}

// BatchUpdateMetadata handles batch metadata update
// PATCH /batch/metadata
func (h *BatchHandler) BatchUpdateMetadata(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.BatchUpdateMetadataInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:BatchUpdateMetadata] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.batchService.BatchUpdateMetadata(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:BatchUpdateMetadata] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("batch metadata update failed"))
		return
	}

	utils.RespondSucces(c, http.StatusAccepted, "batch metadata update initiated", output)
}

// GetBatchOperationStatus gets status of a batch operation
// GET /batch/operations/:operationId
func (h *BatchHandler) GetBatchOperationStatus(c *gin.Context) {
	operationId := c.Param("operationId")
	requestID := c.GetString("requestId")

	output, err := h.batchService.GetBatchOperationStatus(c.Request.Context(), operationId)
	if err != nil {
		log.Printf("[Handler:GetBatchOperationStatus] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve operation status"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "operation status retrieved successfully", output)
}

// ListBatchOperations lists batch operations
// GET /batch/operations
func (h *BatchHandler) ListBatchOperations(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.ListBatchOperationsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:ListBatchOperations] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.batchService.ListBatchOperations(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ListBatchOperations] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list operations"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "operations listed successfully", output)
}

// CancelBatchOperation cancels a batch operation
// DELETE /batch/operations/:operationId
func (h *BatchHandler) CancelBatchOperation(c *gin.Context) {
	operationId := c.Param("operationId")
	requestID := c.GetString("requestId")

	err := h.batchService.CancelBatchOperation(c.Request.Context(), operationId)
	if err != nil {
		log.Printf("[Handler:CancelBatchOperation] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to cancel operation"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "operation cancelled successfully", gin.H{"operationId": operationId, "status": "cancelled"})
}
