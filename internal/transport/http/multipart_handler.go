package http

import (
	"fmt"
	"io"
	"log"
	"net/http"

	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"s3/internal/utils"
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
	requestID := c.GetString("requestId")

	var input dto.InitiateMultipartUploadInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:InitiateMultipartUpload] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId

	output, err := h.multipartService.InitiateMultipartUpload(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:InitiateMultipartUpload] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to initiate multipart upload"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "multipart upload initiated", output)
}

func (h *MultipartHandler) UploadPart(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")
	partNumber, _ := strconv.Atoi(c.Param("partNumber"))
	requestID := c.GetString("requestId")

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[Handler:UploadPart] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("failed to read request body"))
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
		log.Printf("[Handler:UploadPart] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to upload part"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "part uploaded successfully", output)
}

func (h *MultipartHandler) CompleteMultipartUpload(c *gin.Context) {
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

	output, err := h.multipartService.CompleteMultipartUpload(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:CompleteMultipartUpload] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to complete multipart upload"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "multipart upload completed successfully", output)
}

func (h *MultipartHandler) AbortMultipartUpload(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")
	requestID := c.GetString("requestId")

	err := h.multipartService.AbortMultipartUpload(c.Request.Context(), bucketId, uploadId)
	if err != nil {
		log.Printf("[Handler:AbortMultipartUpload] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to abort multipart upload"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "multipart upload aborted", gin.H{"uploadId": uploadId, "status": "aborted"})
}

func (h *MultipartHandler) ListParts(c *gin.Context) {
	bucketId := c.Param("bucketId")
	uploadId := c.Param("uploadId")
	requestID := c.GetString("requestId")

	output, err := h.multipartService.ListParts(c.Request.Context(), bucketId, uploadId)
	if err != nil {
		log.Printf("[Handler:ListParts] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list parts"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "parts listed successfully", output)
}

func (h *MultipartHandler) ListMultipartUploads(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.multipartService.ListMultipartUploads(c.Request.Context(), bucketId)
	if err != nil {
		log.Printf("[Handler:ListMultipartUploads] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list multipart uploads"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "multipart uploads listed successfully", output)
}
