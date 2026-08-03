package http

import (
	"fmt"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/domain"
	"s3/internal/utils"

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
	requestID := c.GetString("requestId")
	var req domain.HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler:HandleHeartbeat] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid request body"))
		return
	}

	resp, err := h.metricsService.HandleHeartbeat(req, c)
	if err != nil {
		log.Printf("[Handler:HandleHeartbeat] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to process heartbeat"))
		return
	}
	utils.RespondSucces(c, http.StatusOK, "Heartbeat recorded", resp)
}
