package http

import (
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

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read file"})
		return
	}

	output, err := h.uploadService.UploadFile(c.Request.Context(), application.UploadFileInput{
		BucketID: bucketID,
		Key:      header.Filename,
		Data:     data,
		MimeType: header.Header.Get("Content-Type"),
		Metadata: map[string]string{
			"original_name": header.Filename,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
 
	c.JSON(http.StatusCreated, gin.H{
		"file_id":    output.FileID,
		"key":        output.Key,
		"size":       output.Size,
		"created_at": output.CreatedAt,
 
	})
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
		"count":     len(files),
		"files":     files,
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