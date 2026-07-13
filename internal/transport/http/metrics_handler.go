package http

import (
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/domain"

	"github.com/gin-gonic/gin"
)

type MetricsHandler struct {
	metricsService *application.MetricsService
}

func NewMetricsHandler(metricsService *application.MetricsService) *MetricsHandler {
	return &MetricsHandler{
		metricsService: metricsService,
	}

}

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *MetricsHandler) HandleHeartbeat(c *gin.Context) {
	var req domain.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Log the specific unmarshaling error to the console
		log.Printf("[host-handler] Heartbeat bind error: %v", err)

		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body: " + err.Error(),
		})

		return
	}

	resp, err := h.metricsService.HandleHeartbeat(req, c)
	if err != nil {
		log.Printf("[host-handler] Heartbeat service error: %v", err)

		c.JSON(http.StatusBadRequest, APIResponse{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})

		return
	}
	c.JSON(http.StatusOK, APIResponse{
		Code:    http.StatusOK,
		Message: "Heartbeat recorded",
		Data:    resp,
	})
}
