package http

import (
	"net/http"
	"s3/internal/application"
	"s3/internal/utils"

	"github.com/gin-gonic/gin"
)

type HandlerForHealth struct {
	healthService *application.HealthService
}

// NewHealthHandler returns a new health handler
func NewHealthHandler(healthService *application.HealthService) *HandlerForHealth {
	return &HandlerForHealth{
		healthService: healthService}
}

// GET /health/ping
func (h *HandlerForHealth) Ping(c *gin.Context) {
	utils.RespondSucces(c, http.StatusOK, "service is healthy", gin.H{
		"status": "ok",
	})
}

// GET /health/status
func (h *HandlerForHealth) GetDetailedStatus(c *gin.Context) {
	stst := h.healthService.Status()
	utils.RespondSucces(c, http.StatusOK, "system status retrieved successfully", gin.H{
		"database":  stst.Database,
		"storage":   stst.Storage,
		"checkedAt": stst.CheckedAt,
	})
}

// GET /health/metrics
func (h *HandlerForHealth) GetMetrics(c *gin.Context) {
	metrics := h.healthService.GetMetrics()
	utils.RespondSucces(c, http.StatusOK, "system metrics retrieved successfully", metrics)
}
