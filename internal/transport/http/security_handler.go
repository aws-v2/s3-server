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

type SecurityHandler struct {
	securityService *application.SecurityService
}

func NewSecurityHandler(securityService *application.SecurityService) *SecurityHandler {
	return &SecurityHandler{
		securityService: securityService,
	}
}

func (h *SecurityHandler) GetSecuritySummary(c *gin.Context) {
	requestID := c.GetString("requestId")
	actor, ok := c.Request.Context().Value("actor").(domain.Actor)
	if !ok {
		log.Printf("[Handler:GetSecuritySummary] Unauthorized, requestID %s", requestID)
		utils.RespondError(c, http.StatusUnauthorized, fmt.Errorf("unauthorized access"))
		return
	}

	summary, err := h.securityService.AnalyzePosture(c.Request.Context(), actor.ID)
	if err != nil {
		log.Printf("[Handler:GetSecuritySummary] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to analyze security posture"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "security summary retrieved successfully", summary)
}
