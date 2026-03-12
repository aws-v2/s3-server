package http

import (
	"net/http"
	"s3/internal/application"
	"s3/internal/domain"

	"github.com/gin-gonic/gin"
)

type AccessPointHandler struct {
	service *application.AccessPointService
}

func NewAccessPointHandler(service *application.AccessPointService) *AccessPointHandler {
	return &AccessPointHandler{service: service}
}

func (h *AccessPointHandler) CreateAccessPoint(c *gin.Context) {
	bucketID := c.Param("bucketId")
	var input domain.CreateAccessPointInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ap, err := h.service.CreateAccessPoint(c.Request.Context(), bucketID, input.Name, input.NetworkOrigin, input.VpcID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, ap)
}

func (h *AccessPointHandler) ListAccessPoints(c *gin.Context) {
	bucketID := c.Param("bucketId")

	output, err := h.service.ListAccessPoints(c.Request.Context(), bucketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

func (h *AccessPointHandler) UpdateAccessPointOrigin(c *gin.Context) {
	bucketID := c.Param("bucketId")
	name := c.Param("accessPointName")

	var input struct {
		NetworkOrigin string `json:"networkOrigin" binding:"required"`
		VpcID         string `json:"vpcId"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ap, err := h.service.UpdateAccessPointOrigin(c.Request.Context(), bucketID, name, input.NetworkOrigin, input.VpcID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, ap)
}
