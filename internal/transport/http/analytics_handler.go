package http

import (
	"fmt"
	"log"
	"net/http"

	"s3/internal/application"
	"s3/internal/domain"
	"s3/internal/infrastructure/dto"
	"s3/internal/utils"

	"github.com/gin-gonic/gin"
)

type AnalyticsHandler struct {
	analyticsService *application.AnalyticsService
}

func NewAnalyticsHandler(analyticsService *application.AnalyticsService) *AnalyticsHandler {
	return &AnalyticsHandler{analyticsService: analyticsService}
}

func (h *AnalyticsHandler) TrackRequestMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		// Track after request is processed
		status := fmt.Sprintf("%d", c.Writer.Status())
		h.analyticsService.TrackRequest(c.Request.Method, c.FullPath(), status)
	}
}

func (h *AnalyticsHandler) GetStorageUsage(c *gin.Context) {
	requestID := c.GetString("requestId")
	output, err := h.analyticsService.GetStorageUsage(c.Request.Context())
	if err != nil {
		log.Printf("[Handler:GetStorageUsage] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve storage usage"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "storage usage retrieved successfully", output)
}

func (h *AnalyticsHandler) GetTrafficStats(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.GetTrafficStatsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:GetTrafficStats] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.analyticsService.GetTrafficStats(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GetTrafficStats] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve traffic statistics"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "traffic stats retrieved successfully", output)
}

func (h *AnalyticsHandler) GetFileTypeDistribution(c *gin.Context) {
	requestID := c.GetString("requestId")
	output, err := h.analyticsService.GetFileTypeDistribution(c.Request.Context())
	if err != nil {
		log.Printf("[Handler:GetFileTypeDistribution] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve file type distribution"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "file type distribution retrieved successfully", output)
}

func (h *AnalyticsHandler) GetBucketUsageOverTime(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.GetBucketUsageOverTimeInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:GetBucketUsageOverTime] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.analyticsService.GetBucketUsageOverTime(c.Request.Context(), bucketId, input)
	if err != nil {
		log.Printf("[Handler:GetBucketUsageOverTime] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve bucket usage"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "bucket usage retrieved successfully", output)
}

func (h *AnalyticsHandler) GetPopularFiles(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.GetPopularFilesInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:GetPopularFiles] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	output, err := h.analyticsService.GetPopularFiles(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GetPopularFiles] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve popular files"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "popular files retrieved successfully", output)
}

func (h *AnalyticsHandler) GetUserActivity(c *gin.Context) {
	userId := c.Param("userId")
	requestID := c.GetString("requestId")

	output, err := h.analyticsService.GetUserActivity(c.Request.Context(), userId)
	if err != nil {
		log.Printf("[Handler:GetUserActivity] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve user activity"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "user activity retrieved successfully", output)
}

func (h *AnalyticsHandler) ExportAnalytics(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.ExportAnalyticsInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:ExportAnalytics] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	data, contentType, err := h.analyticsService.ExportAnalytics(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ExportAnalytics] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to export analytics"))
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "attachment; filename=analytics."+input.Format)
	c.Data(http.StatusOK, contentType, data)
}

func (h *AnalyticsHandler) GetAPIUsage(c *gin.Context) {
	requestID := c.GetString("requestId")
	output, err := h.analyticsService.GetAPIUsage(c.Request.Context())
	if err != nil {
		log.Printf("[Handler:GetAPIUsage] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve API usage"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "API usage retrieved successfully", output)
}

func (h *AnalyticsHandler) GetStorageLensData(c *gin.Context) {
	requestID := c.GetString("requestId")
	// For now, we'll extract user ID from the context (auth middleware)
	// actor, _ := c.Request.Context().Value("actor").(domain.Actor)
	// userID := actor.ID

	// Mocking userID as 'sys' if not present for local testing, or use extracted ID
	userID := "sys"
	if actor, ok := c.Request.Context().Value("actor").(domain.Actor); ok {
		userID = actor.ID
	}

	output, err := h.analyticsService.GetStorageLensReport(c.Request.Context(), userID)
	if err != nil {
		log.Printf("[Handler:GetStorageLensData] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve storage lens data"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "storage lens data retrieved successfully", output)
}
