package http

import (
	"fmt"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"strings"

	"github.com/gin-gonic/gin"
)

// BucketHandler handles bucket-related HTTP endpoints.
type BucketHandler struct {
	bucketService *application.BucketService
}

type BucketAlreadyExists struct {
	Name string
}

func (e *BucketAlreadyExists) Error() string {
	return fmt.Sprintf("bucket %s already exists", e.Name)
}

// NewBucketHandler creates a new instance of BucketHandler.
func NewBucketHandler(bucketService *application.BucketService) *BucketHandler {
	return &BucketHandler{
		bucketService: bucketService,
	}
}

// POST /buckets
func (h *BucketHandler) CreateBucket(c *gin.Context) {
	var input dto.CreateBucketInput
	if err := c.ShouldBindJSON(&input); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	actorI, ok := c.Get("actor")
	if ok {
		actor := actorI.(string)
		if strings.HasPrefix(actor, "user:") {
			input.OwnerId = strings.TrimPrefix(actor, "user:")
		}
	}

	output, err := h.bucketService.CreateBucket(c.Request.Context(), input)
	if err != nil {
		errorString := fmt.Sprintf("bucket %s already exists", input.Name)
		// Handle "bucket already exists" error using type assertion
		if strings.Contains(err.Error(), errorString) {

			c.JSON(http.StatusConflict, gin.H{"": err.Error()})
			return
		}

		// All other errors
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	newBucket := dto.CreateBucketOutput{
		BucketID:  output.BucketID,
		Name:      output.Name,
		CreatedAt: output.CreatedAt,
	}

	c.JSON(http.StatusCreated, newBucket)
}

// ListBuckets handles listing all available buckets
// GET /buckets
func (h *BucketHandler) ListBuckets(c *gin.Context) {
	buckets, err := h.bucketService.ListBuckets(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(buckets),
		"buckets": buckets,
	})
}

// ListBuckets handles listing all available buckets
// GET /:bucketId
func (h *BucketHandler) GetBucketInfo(c *gin.Context) {
	bucketID := c.Param("bucketId")

	output, err := h.bucketService.GetBucket(c.Request.Context(), bucketID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// UpdateBucket handles updating bucket settings
// PATCH /:bucketId
func (h *BucketHandler) UpdateBucket(c *gin.Context) {
	bucketID := c.Param("bucketId")

	var input dto.UpdateBucketInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.bucketService.UpdateBucket(c.Request.Context(), bucketID, input)

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// // DeleteBucket handles deleting a bucket
// // DELETE /:bucketId
func (h *BucketHandler) DeleteBucket(c *gin.Context) {
	bucketID := c.Param("bucketId")

	if err := h.bucketService.DeleteBucket(c.Request.Context(), bucketID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// // GetBucketStats handles getting bucket statistics
// // GET /:bucketId/stats
func (h *BucketHandler) GetBucketStats(c *gin.Context) {
	bucketID := c.Param("bucketId")

	stats, err := h.bucketService.GetBucketStats(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// // GetBucketPolicy handles getting bucket policy
// // GET /:bucketId/policy
func (h *BucketHandler) GetBucketPolicy(c *gin.Context) {
	bucketID := c.Param("bucketId")
	policy, err := h.bucketService.GetBucketPolicy(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// // UpdateBucketPolicy handles updating bucket policy
// // PUT /:bucketId/policy
func (h *BucketHandler) UpdateBucketPolicy(c *gin.Context) {
	actorI, ok := c.Get("actor") // set by auth middleware
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthenticated"})
		return
	}
	actor := actorI.(string)

	bucketID := c.Param("bucketId")
	var input dto.UpdatePolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {

		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload: " + err.Error()})
		return
	}

	if err := h.bucketService.UpdateBucketPolicy(c.Request.Context(), bucketID, input, actor); err != nil {

		if strings.Contains(err.Error(), "forbidden") {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "policy updated"})
}

// // SetBucketVersioning handles enabling/disabling versioning
// // PUT /:bucketId/versioning
func (h *BucketHandler) SetBucketVersioning(c *gin.Context) {
	bucketID := c.Param("bucketId")

	var input dto.VersioningInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	if err := h.bucketService.SetBucketVersioning(c.Request.Context(), bucketID, input.Enabled); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"versioning_enabled": input.Enabled})
}

func (h *BucketHandler) GetBucketVersioning(c *gin.Context) {
	bucketID := c.Param("bucketId")

	output, err := h.bucketService.GetBucketVersioning(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// // SetBucketLifecycle handles setting lifecycle rules
// SetBucketLifecycle handles setting lifecycle rules for a bucket.
// PUT /:bucketId/lifecycle
func (h *BucketHandler) SetBucketLifecycle(c *gin.Context) {
	bucketID := c.Param("bucketId")

	var input dto.SetLifecycleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	if err := h.bucketService.SetBucketLifecycle(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "lifecycle rules updated"})
}

// GetBucketLifecycle handles fetching lifecycle rules for a bucket.
// GET /:bucketId/lifecycle
func (h *BucketHandler) GetBucketLifecycle(c *gin.Context) {
	bucketID := c.Param("bucketId")

	rules, err := h.bucketService.GetBucketLifecycle(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"rules": rules})
}
