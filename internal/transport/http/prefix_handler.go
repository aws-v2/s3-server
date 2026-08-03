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

type PrefixHandler struct {
	prefixService *application.PrefixService
}

func NewPrefixHandler(prefixService *application.PrefixService) *PrefixHandler {
	return &PrefixHandler{
		prefixService: prefixService,
	}
}



func (h *PrefixHandler) CreatePrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")
	userId := c.GetString("userId")

	if bucketId == "" || userId=="" {
		log.Printf("[Handler:ListByPrefix] Bad request, requestID %s error %s", requestID, fmt.Errorf("Missing folder or bucket ids"))
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))

	}

	var input dto.CreatePrefixInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:ListByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}

	input.BucketId = bucketId
	input.FildterId = userId

	 err := h.prefixService.CreatePrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ListByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list files by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "files listed successfully", gin.H{
		"data":"Folder created successfully",
	})

}

// ListByPrefix lists files by prefix
// GET /prefix/:bucketId/list
func (h *PrefixHandler) PrefixContent(c *gin.Context) {
	bucketId := c.Param("bucketId")
	folderID := c.Param("folderId")
	requestID := c.GetString("requestId")

	if bucketId == "" || folderID == "" {
		log.Printf("[Handler:ListByPrefix] Bad request, requestID %s error %s", requestID, fmt.Errorf("Missing folder or bucket ids"))
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))

	}

	output, err := h.prefixService.PrefixContent(c.Request.Context(), bucketId, folderID)
	if err != nil {
		log.Printf("[Handler:ListByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list files by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "files listed successfully", output)

}
func (h *PrefixHandler) ListByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.ListByPrefixInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:ListByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.ListByPrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ListByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list files by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "files listed successfully", output)
}

// DeleteByPrefix deletes files by prefix
// DELETE /prefix/:bucketId/delete
func (h *PrefixHandler) DeleteByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.DeleteByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:DeleteByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.DeleteByPrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:DeleteByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to delete files by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "files deleted successfully", output)
}

// CopyByPrefix copies files by prefix
// POST /prefix/:bucketId/copy
func (h *PrefixHandler) CopyByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.CopyByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CopyByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.CopyByPrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:CopyByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to copy files by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "files copied successfully", output)
}

// GetSizeByPrefix gets total size of files by prefix
// GET /prefix/:bucketId/size
func (h *PrefixHandler) GetSizeByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.GetSizeByPrefixInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:GetSizeByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.GetSizeByPrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:GetSizeByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to get size by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "size retrieved successfully", output)
}

// CountByPrefix counts files by prefix
// GET /prefix/:bucketId/count
func (h *PrefixHandler) CountByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.CountByPrefixInput
	if err := c.ShouldBindQuery(&input); err != nil {
		log.Printf("[Handler:CountByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid query parameters"))
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.CountByPrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:CountByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to count files by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "file count retrieved successfully", output)
}

// ArchiveByPrefix archives files by prefix
// POST /prefix/:bucketId/archive
func (h *PrefixHandler) ArchiveByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	userID := c.GetString("requestId")
	requestID := c.GetString("requestId")
	var input dto.ArchiveByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:ArchiveByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId
	input.UserID = userID

	output, err := h.prefixService.ArchiveByPrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:ArchiveByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to archive files by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "files archived successfully", output)
}

// SetMetadataByPrefix sets metadata for files by prefix
// PATCH /prefix/:bucketId/metadata
func (h *PrefixHandler) SetMetadataByPrefix(c *gin.Context) {
	bucketId := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input dto.SetMetadataByPrefixInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:SetMetadataByPrefix] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}
	input.BucketID = bucketId

	output, err := h.prefixService.SetMetadataByPrefix(c.Request.Context(), input)
	if err != nil {
		log.Printf("[Handler:SetMetadataByPrefix] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to set metadata by prefix"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "metadata set successfully", output)
}
