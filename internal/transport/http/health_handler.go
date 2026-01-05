package http

import (
	"net/http"
	"s3/internal/application"

	"github.com/gin-gonic/gin"
)
type HandlerForHealth struct {
	healthService *application.HealthService
}


// NewHealthHandler returns a new health handler
func NewHealthHandler(healthService *application.HealthService) *HandlerForHealth {
	return &HandlerForHealth{
		healthService:healthService}
	}



// GET /health/ping
func (h *HandlerForHealth) Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
		"status":  "ok",
	})
}

// GET /health/status
func (h *HandlerForHealth) GetDetailedStatus(c *gin.Context) {

	stst :=h.healthService.Status()
	c.JSON(http.StatusOK, gin.H{
		"Database": stst.Database,
		"Storage":  stst.Storage,
		"CheckedAt": stst.CheckedAt,
	})
}

// GET /health/metrics
func (h * HandlerForHealth)GetMetrics(c *gin.Context) {
	metrics :=h.healthService.GetMetrics()
	c.JSON(http.StatusOK, metrics)
}