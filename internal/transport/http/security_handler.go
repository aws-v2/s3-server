package http

import (
	"net/http"
	"s3/internal/application"
	"s3/internal/domain"

	"github.com/gin-gonic/gin"
)

type SecurityHandler struct {
	securityService *application.SecurityService
}

func NewSecurityHandler(securityService *application.SecurityService) *SecurityHandler {
	return &SecurityHandler{
		securityService: securityService,
	}
}

func (h *SecurityHandler) GetSecuritySummary(c *gin.Context) {
	actor, ok := c.Request.Context().Value("actor").(domain.Actor)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	summary, err := h.securityService.AnalyzePosture(c.Request.Context(), actor.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, summary)
}
