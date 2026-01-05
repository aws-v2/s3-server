package http

import (
	"net/http"

	"s3/internal/application"
	"s3/internal/infrastructure/dto"

	"github.com/gin-gonic/gin"
)

type WebhookHandler struct {
	webhookService *application.WebhookService
}

func NewWebhookHandler(webhookService *application.WebhookService) *WebhookHandler {
	return &WebhookHandler{webhookService: webhookService}
}

func (h *WebhookHandler) CreateWebhook(c *gin.Context) {
	var input dto.CreateWebhookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.webhookService.CreateWebhook(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, output)
}

func (h *WebhookHandler) ListWebhooks(c *gin.Context) {
	bucketId := c.Param("bucketId")

	output, err := h.webhookService.ListWebhooks(c.Request.Context(), bucketId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *WebhookHandler) GetWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")

	output, err := h.webhookService.GetWebhook(c.Request.Context(), webhookId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *WebhookHandler) UpdateWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")

	var input dto.UpdateWebhookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	err := h.webhookService.UpdateWebhook(c.Request.Context(), webhookId, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "webhook updated successfully"})
}

func (h *WebhookHandler) DeleteWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")

	err := h.webhookService.DeleteWebhook(c.Request.Context(), webhookId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "webhook deleted successfully"})
}

func (h *WebhookHandler) TestWebhook(c *gin.Context) {
	webhookId := c.Param("webhookId")

	output, err := h.webhookService.TestWebhook(c.Request.Context(), webhookId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *WebhookHandler) GetWebhookDeliveries(c *gin.Context) {
	webhookId := c.Param("webhookId")

	output, err := h.webhookService.GetWebhookDeliveries(c.Request.Context(), webhookId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}