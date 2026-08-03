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

type WebhookHandler struct {
	webhookService *application.WebhookService
}

func NewWebhookHandler(webhookService *application.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: webhookService}
}

func (h *WebhookHandler) CreateWebhook(c *gin.Context) {
	requestID := c.GetString("requestId")
	var input dto.CreateWebhookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CreateWebhook] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.webhookService.CreateWebhook(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:CreateWebhook] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to create webhook"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "webhook created successfully", output)
}

func (h *WebhookHandler) ListWebhooks(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.webhookService.ListWebhooks(c.Request.Context(), bucketId)
	if err != nil {
		log.Printf("[Handler:ListWebhooks] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list webhooks"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "webhooks listed successfully", output)
}

func (h *WebhookHandler) GetWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")
	requestID := c.GetString("requestId")

	output, err := h.webhookService.GetWebhook(c.Request.Context(), webhookId)
	if err != nil {
		log.Printf("[Handler:GetWebhook] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve webhook"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "webhook retrieved successfully", output)
}

func (h *WebhookHandler) UpdateWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")
	requestID := c.GetString("requestId")

	var input dto.UpdateWebhookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateWebhook] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	err := h.webhookService.UpdateWebhook(c.Request.Context(), webhookId, input)
	if err != nil {
		log.Printf("[Handler:UpdateWebhook] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update webhook"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "webhook updated successfully", gin.H{"webhookId": webhookId, "status": "updated"})
}

func (h *WebhookHandler) DeleteWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")
	requestID := c.GetString("requestId")

	err := h.webhookService.DeleteWebhook(c.Request.Context(), webhookId)
	if err != nil {
		log.Printf("[Handler:DeleteWebhook] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to delete webhook"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "webhook deleted successfully", gin.H{"webhookId": webhookId, "status": "deleted"})
}

func (h *WebhookHandler) TestWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")
	requestID := c.GetString("requestId")

	output, err := h.webhookService.TestWebhook(c.Request.Context(), webhookId)
	if err != nil {
		log.Printf("[Handler:TestWebhook] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to test webhook"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "webhook test completed", output)
}

func (h *WebhookHandler) GetWebhookDeliveries(c *gin.Context) {
	webhookId := c.Param("webhookId")
	requestID := c.GetString("requestId")

	output, err := h.webhookService.GetWebhookDeliveries(c.Request.Context(), webhookId)
	if err != nil {
		log.Printf("[Handler:GetWebhookDeliveries] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to retrieve webhook deliveries"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "webhook deliveries retrieved successfully", output)
}
