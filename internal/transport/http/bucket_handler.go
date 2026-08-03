package http

import (
	"fmt"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/utils"
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
	userId := c.GetString("userId")
	requestID := c.GetString("requestId")
	var input dto.CreateBucketInput
	input.OwnerId = userId
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CreateBucket] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	actorI, ok := c.Get("actor")
	if ok {
		if actor, ok := actorI.(domain.Actor); ok {
			input.OwnerId = actor.ID
		} else if actorStr, ok := actorI.(string); ok {
			if strings.HasPrefix(actorStr, "user:") {
				input.OwnerId = strings.TrimPrefix(actorStr, "user:")
			}
		}
	}

	output, err := h.bucketService.CreateBucket(c.Request.Context(), input)
	if err != nil {
		errorString := fmt.Sprintf("bucket %s already exists", input.Name)
		// Handle "bucket already exists" error using type assertion
		if strings.Contains(err.Error(), errorString) {
			log.Printf("[Handler:CreateBucket] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusConflict, fmt.Errorf("bucket already exists"))
			return
		}

		// All other errors
		log.Printf("[Handler:CreateBucket] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to create bucket"))
		return
	}

	newBucket := dto.CreateBucketOutput{
		BucketID:  output.BucketID,
		Name:      output.Name,
		CreatedAt: output.CreatedAt,
	}

	utils.RespondSucces(c, http.StatusCreated, "bucket created successfully", newBucket)
}

// ListBuckets handles listing all available buckets
// GET /buckets
func (h *BucketHandler) ListBuckets(c *gin.Context) {
	userId := c.GetString("userId")
	requestID := c.GetString("requestId")

	buckets, err := h.bucketService.ListBuckets(c.Request.Context(), userId)
	if err != nil {
		log.Printf("[Handler:ListBuckets] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list buckets"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "buckets listed successfully", gin.H{
		"count":   len(buckets),
		"buckets": buckets,
	})
}

// GET /:bucketId
func (h *BucketHandler) GetBucketInfo(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucket(c.Request.Context(), bucketID)
	if err != nil {
		log.Printf("[Handler:GetBucketInfo] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket info retrieved successfully", output)
}

// GET /:bucketId
func (h *BucketHandler) ListBucketFiles(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.ListBucketFiles(c.Request.Context(), bucketID)
	if err != nil {
		log.Printf("[Handler:ListBucketFiles] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket files listed successfully", output)
}

// UpdateBucket handles updating bucket settings
// PATCH /:bucketIdj
func (h *BucketHandler) UpdateBucket(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.UpdateBucketInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateBucket] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.bucketService.UpdateBucket(c.Request.Context(), bucketID, input)

	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:UpdateBucket] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:UpdateBucket] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update bucket"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket updated successfully", output)
}

// // DeleteBucket handles deleting a bucket
// // DELETE /:bucketId
func (h *BucketHandler) DeleteBucket(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	if err := h.bucketService.DeleteBucket(c.Request.Context(), bucketID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:DeleteBucket] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:DeleteBucket] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to delete bucket"))
		return
	}

	utils.RespondSucces(c, http.StatusNoContent, "bucket deleted successfully", nil)
}

// POST /buckets/:bucketId/empty
func (h *BucketHandler) EmptyBucket(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	if err := h.bucketService.EmptyBucket(c.Request.Context(), bucketID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:EmptyBucket] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:EmptyBucket] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to empty bucket"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket emptied successfully", gin.H{"status": "emptied"})
}

// // GetBucketStats handles getting bucket statistics
// // GET /:bucketId/stats
func (h *BucketHandler) GetBucketStats(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	stats, err := h.bucketService.GetBucketStats(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketStats] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketStats] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket stats"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket stats retrieved successfully", stats)
}

// // GetBucketPolicy handles getting bucket policy
// // GET /:bucketId/policy
func (h *BucketHandler) GetBucketPolicy(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")
	policy, err := h.bucketService.GetBucketPolicy(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketPolicy] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketPolicy] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket policy"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket policy retrieved successfully", policy)
}

// // UpdateBucketPolicy handles updating bucket policy
// // PUT /:bucketId/policy
func (h *BucketHandler) UpdateBucketPolicy(c *gin.Context) {
	actorI, ok := c.Get("actor") // set by auth middleware
	requestID := c.GetString("requestId")
	if !ok {
		log.Printf("[Handler:UpdateBucketPolicy] Bad request, requestID %s error unauthenticated", requestID)
		utils.RespondError(c, http.StatusUnauthorized, fmt.Errorf("authentication required"))
		return
	}
	var actor string
	if a, ok := actorI.(domain.Actor); ok {
		actor = a.ID
	} else if aStr, ok := actorI.(string); ok {
		actor = aStr
	} else {
		log.Printf("[Handler:UpdateBucketPolicy] Bad request, requestID %s error invalid actor type", requestID)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("invalid authentication state"))
		return
	}

	bucketID := c.Param("bucketId")
	var input dto.UpdatePolicyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateBucketPolicy] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.UpdateBucketPolicy(c.Request.Context(), bucketID, input, actor); err != nil {
		if strings.Contains(err.Error(), "forbidden") {
			log.Printf("[Handler:UpdateBucketPolicy] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusForbidden, fmt.Errorf("access denied"))
			return
		}
		log.Printf("[Handler:UpdateBucketPolicy] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("failed to update bucket policy"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "bucket policy updated successfully", gin.H{"status": "updated"})
}

// // SetBucketVersioning handles enabling/disabling versioning
// // PUT /:bucketId/versioning
func (h *BucketHandler) SetBucketVersioning(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.VersioningInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:SetBucketVersioning] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketVersioning(c.Request.Context(), bucketID, input.Enabled); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:SetBucketVersioning] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:SetBucketVersioning] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to set bucket versioning"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket versioning updated successfully", gin.H{"versioning_enabled": input.Enabled})
}

func (h *BucketHandler) GetBucketVersioning(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketVersioning(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketVersioning] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketVersioning] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket versioning"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket versioning retrieved successfully", output)
}

// // SetBucketLifecycle handles setting lifecycle rules
// SetBucketLifecycle handles setting lifecycle rules for a bucket.
// PUT /:bucketId/lifecycle
func (h *BucketHandler) SetBucketLifecycle(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.SetLifecycleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:SetBucketLifecycle] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketLifecycle(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:SetBucketLifecycle] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:SetBucketLifecycle] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to set bucket lifecycle"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "lifecycle rules updated successfully", gin.H{"status": "updated"})
}

// GetBucketLifecycle handles fetching lifecycle rules for a bucket.
// GET /:bucketId/lifecycle
func (h *BucketHandler) GetBucketLifecycle(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	rules, err := h.bucketService.GetBucketLifecycle(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketLifecycle] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketLifecycle] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket lifecycle"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "lifecycle rules retrieved successfully", gin.H{"rules": rules})
}

func (h *BucketHandler) SetBucketBlockPublicAccess(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.SetBlockPublicAccessInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:SetBucketBlockPublicAccess] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketBlockPublicAccess(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:SetBucketBlockPublicAccess] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:SetBucketBlockPublicAccess] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to set block public access"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "block public access updated successfully", gin.H{"status": "updated"})
}

func (h *BucketHandler) GetBucketCORS(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketCORS(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketCORS] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketCORS] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket CORS"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket CORS retrieved successfully", output)
}

func (h *BucketHandler) UpdateBucketCORS(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.CORSConfiguration
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateBucketCORS] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketCORS(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:UpdateBucketCORS] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:UpdateBucketCORS] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update bucket CORS"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket CORS updated successfully", gin.H{"status": "updated"})
}

func (h *BucketHandler) SetBucketEncryption(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.BucketEncryption
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:SetBucketEncryption] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketEncryption(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:SetBucketEncryption] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:SetBucketEncryption] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to set bucket encryption"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket encryption updated successfully", gin.H{"status": "updated"})
}

func (h *BucketHandler) GetBucketEncryption(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketEncryption(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketEncryption] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketEncryption] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket encryption"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket encryption retrieved successfully", output)
}

func (h *BucketHandler) GetBucketObjectLock(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketObjectLock(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketObjectLock] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketObjectLock] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket object lock"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket object lock retrieved successfully", output)
}

func (h *BucketHandler) GetBucketLogging(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketLogging(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketLogging] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketLogging] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket logging"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket logging retrieved successfully", output)
}

func (h *BucketHandler) GetBucketReplication(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketReplication(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketReplication] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketReplication] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket replication"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket replication retrieved successfully", output)
}

func (h *BucketHandler) GetBucketTags(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketTags(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketTags] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketTags] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket tags"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket tags retrieved successfully", output)
}

func (h *BucketHandler) GetBucketNotifications(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.bucketService.GetBucketNotifications(c.Request.Context(), bucketID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:GetBucketNotifications] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:GetBucketNotifications] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get bucket notifications"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "bucket notifications retrieved successfully", output)
}

func (h *BucketHandler) SetBucketObjectLock(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.ObjectLockInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:SetBucketObjectLock] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketObjectLock(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:SetBucketObjectLock] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:SetBucketObjectLock] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to set object lock"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "object lock configuration updated successfully", gin.H{"status": "updated"})
}

func (h *BucketHandler) UpdateBucketReplication(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.UpdateReplicationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateBucketReplication] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketReplication(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:UpdateBucketReplication] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:UpdateBucketReplication] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update replication"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "replication configuration updated successfully", gin.H{"status": "updated"})
}

func (h *BucketHandler) UpdateBucketLogging(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.UpdateLoggingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateBucketLogging] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketLogging(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:UpdateBucketLogging] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:UpdateBucketLogging] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update logging"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "logging configuration updated successfully", gin.H{"status": "updated"})
}

func (h *BucketHandler) UpdateBucketNotifications(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.UpdateNotificationsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateBucketNotifications] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketNotifications(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:UpdateBucketNotifications] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:UpdateBucketNotifications] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update notifications"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "notifications configuration updated successfully", gin.H{"status": "updated"})
}

func (h *BucketHandler) UpdateBucketTags(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.UpdateTagsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateBucketTags] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.bucketService.SetBucketTags(c.Request.Context(), bucketID, input); err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Printf("[Handler:UpdateBucketTags] Service call, requestID %s error %s", requestID, err.Error())
			utils.RespondError(c, http.StatusNotFound, fmt.Errorf("bucket not found"))
			return
		}
		log.Printf("[Handler:UpdateBucketTags] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update tags"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "tags configuration updated successfully", gin.H{"status": "updated"})
}
