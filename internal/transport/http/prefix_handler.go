package http

import (
	"net/http"

	"s3/internal/application"
	"s3/internal/infrastructure/dto"

	"github.com/gin-gonic/gin"
)

type PrefixHandler struct {
	prefixService *application.PrefixService
}

func NewPrefixHandler(prefixService *application.PrefixService) *PrefixHandler {
	return &PrefixHandler{
		prefixService: prefixService,
	}
}

// ListByPrefix lists files by prefix
// GET /prefix/:bucketId/list
func (h *PrefixHandler) ListByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.ListByPrefixInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.ListByPrefix(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// DeleteByPrefix deletes files by prefix
// DELETE /prefix/:bucketId/delete
func (h *PrefixHandler) DeleteByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.DeleteByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.DeleteByPrefix(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// CopyByPrefix copies files by prefix
// POST /prefix/:bucketId/copy
func (h *PrefixHandler) CopyByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.CopyByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.CopyByPrefix(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// GetSizeByPrefix gets total size of files by prefix
// GET /prefix/:bucketId/size
func (h *PrefixHandler) GetSizeByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.GetSizeByPrefixInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.GetSizeByPrefix(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// CountByPrefix counts files by prefix
// GET /prefix/:bucketId/count
func (h *PrefixHandler) CountByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.CountByPrefixInput
	if err := c.ShouldBindQuery(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid query parameters"})
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.CountByPrefix(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// ArchiveByPrefix archives files by prefix
// POST /prefix/:bucketId/archive
func (h *PrefixHandler) ArchiveByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.ArchiveByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.ArchiveByPrefix(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// SetMetadataByPrefix sets metadata for files by prefix
// PATCH /prefix/:bucketId/metadata
func (h *PrefixHandler) SetMetadataByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")

	var input dto.SetMetadataByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.SetMetadataByPrefix(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}