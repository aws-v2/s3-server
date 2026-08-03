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

type AccessPointHandler struct {
	service *application.AccessPointService
}

func NewAccessPointHandler(service *application.AccessPointService) *AccessPointHandler {
	return &AccessPointHandler{service: service}
}

func (h *AccessPointHandler) CreateAccessPoint(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")
	var input domain.CreateAccessPointInput

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CreateAccessPoint] Payload unmarshal, bad request for requestID %s, with error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("Bad request"))
		return
	}

	ap, err := h.service.CreateAccessPoint(c.Request.Context(), bucketID, input.Name, input.NetworkOrigin, input.VpcID)
	if err != nil {
		log.Printf("[Handler:CreateAccessPoint] Service call, requestID %s  error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to create access point"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "public docs successfully", ap)

}

func (h *AccessPointHandler) ListAccessPoints(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.service.ListAccessPoints(c.Request.Context(), bucketID)
	if err != nil {
		log.Printf("[Handler:ListAccessPoints] Service call, requestID %s  error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list access points"))
		return
	}
	utils.RespondSucces(c, http.StatusCreated, "listed accesspoints successfully", output)

}

func (h *AccessPointHandler) UpdateAccessPointOrigin(c *gin.Context) {
	bucketID := c.Param("bucketId")
	name := c.Param("accessPointName")
	requestID := c.GetString("requestId")

	var input domain.UpdateAccessPointOrigin

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CreateAccessPoint] Payload unmarshal, bad request for requestID %s, with error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("Bad request"))
		return
	}
	ap, err := h.service.UpdateAccessPointOrigin(c.Request.Context(), bucketID, name, input.NetworkOrigin, input.VpcID)
	if err != nil {
		log.Printf("[Handler:UpdateAccessPointOrigin] Service call, requestID %s  error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update access points"))
		return
	}
	utils.RespondSucces(c, http.StatusCreated, "updated accesspoints successfully", ap)

}
