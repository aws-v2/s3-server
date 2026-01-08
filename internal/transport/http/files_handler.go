package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"

	"github.com/gin-gonic/gin"
)

type HandlerForFiles struct {
	uploadService *application.UploadService
	deleteService *application.DeleteService
}

// Constructor for all file-related handlers
func NewFileHandler(
	uploadService *application.UploadService,
	deleteService *application.DeleteService,
) *HandlerForFiles {
	return &HandlerForFiles{
		uploadService: uploadService,
		deleteService: deleteService,
	}
}

// UploadFile handles file upload
// POST /buckets/:bucketId/files
func (h *HandlerForFiles) UploadFile(c *gin.Context) {
	bucketID := c.Param("bucketId")

	// Parse Multipart Form
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form: " + err.Error()})
		return
	}

	// 1. Process Files
	files := form.File["files"]
	var fileContents []application.FileContent
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			continue
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			continue
		}

		fileContents = append(fileContents, application.FileContent{
			Name: fileHeader.Filename,
			Size: fileHeader.Size,
			Type: fileHeader.Header.Get("Content-Type"),
			Data: data,
		})
	}

	// 2. Parse JSON fields from form
	var destSettings application.DestinationSettings
	if ds := c.PostForm("destinationSettings"); ds != "" {
		json.Unmarshal([]byte(ds), &destSettings)
	}

	var properties application.UploadProperties
	if p := c.PostForm("properties"); p != "" {
		json.Unmarshal([]byte(p), &properties)
	}

	var tags []application.Tag
	if t := c.PostForm("tags"); t != "" {
		json.Unmarshal([]byte(t), &tags)
	}

	var metadata []application.MetadataItem
	if m := c.PostForm("metadata"); m != "" {
		json.Unmarshal([]byte(m), &metadata)
	}

	prefix := c.PostForm("prefix")

	input := application.UploadFileInput{
		BucketID:            bucketID,
		Prefix:              prefix,
		Files:               fileContents,
		DestinationSettings: destSettings,
		Properties:          properties,
		Tags:                tags,
		Metadata:            metadata,
	}

	output, err := h.uploadService.UploadFile(c.Request.Context(), input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, output)
}

// CreateFolder handles explicit folder creation (0-byte object)
// POST /buckets/:bucketId/folders
func (h *HandlerForFiles) CreateFolder(c *gin.Context) {
	bucketID := c.Param("bucketId")

	var input struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	if err := h.uploadService.CreateFolder(c.Request.Context(), bucketID, input.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "folder created successfully"})
}

// ListFiles handles listing files in a bucket
// GET /buckets/:bucketId/files
func (h *HandlerForFiles) ListFiles(c *gin.Context) {
	bucketID := c.Param("bucketId")

	files, err := h.uploadService.ListFiles(c.Request.Context(), bucketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bucketId": bucketID,
		"count":    len(files),
		"files":    files,
	})
}

// DeleteFile handles file deletion
// DELETE /buckets/:bucketId/files/:fileId
func (h *HandlerForFiles) DeleteFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	err := h.deleteService.DeleteFile(c.Request.Context(), application.DeleteFileInput{
		FileID:   fileID,
		BucketID: bucketID,
	})
	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "file deleted successfully",
	})
}

// GetFileInfo handles getting file metadata
// GET /:bucketId/files/:fileId
func (h *HandlerForFiles) GetFileInfo(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	output, err := h.uploadService.GetFileInfo(c.Request.Context(), bucketID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// DownloadFile handles file download
// GET /:bucketId/files/:fileId/download
func (h *HandlerForFiles) DownloadFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	fileData, metadata, err := h.uploadService.DownloadFile(c.Request.Context(), bucketID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", metadata.Key))
	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", metadata.Size))

	c.Data(http.StatusOK, metadata.MimeType, fileData)
}

// UpdateFileMetadata handles updating file metadata
// PATCH /:bucketId/files/:fileId
func (h *HandlerForFiles) UpdateFileMetadata(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	var input dto.UpdateFileMetadataInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.uploadService.UpdateFileMetadata(c.Request.Context(), bucketID, fileID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}

// CopyFile handles copying a file
// POST /:bucketId/files/:fileId/copy
func (h *HandlerForFiles) CopyFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	var input dto.CopyFileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.uploadService.CopyFile(c.Request.Context(), bucketID, fileID, input)

	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, output)
}

// MoveFile handles moving a file to another bucket
// POST /:bucketId/files/:fileId/move
func (h *HandlerForFiles) MoveFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")

	var input dto.MoveFileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	output, err := h.uploadService.MoveFile(c.Request.Context(), bucketID, fileID, input)
	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, output)
}
