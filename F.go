package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	_"os"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"s3/internal/application"
	"s3/internal/infrastructure/dto"
	"s3/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type HandlerForFiles struct {
	uploadService *application.UploadService
	deleteService *application.DeleteService
	secretKey     string
}

// Constructor for all file-related handlers
func NewFileHandler(
	uploadService *application.UploadService,
	deleteService *application.DeleteService,
	secretKey string,

) *HandlerForFiles {
	return &HandlerForFiles{
		uploadService: uploadService,
		deleteService: deleteService,
		secretKey:     secretKey,
	}
}

// UploadFile handles file upload
// POST /buckets/:bucketId/files
// UploadFile handles file upload
// POST /buckets/:bucketId/files
func (h *HandlerForFiles) UploadFile(c *gin.Context) {
	start := time.Now()

	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")
	userID := c.GetString("userId")

	log.Printf(
		"[UploadFile] started request_id=%s user_id=%s bucket_id=%s",
		requestID,
		userID,
		bucketID,
	)

	// Parse Multipart Form
	form, err := c.MultipartForm()
	if err != nil {
		log.Printf(
			"[Handler:UploadFile] Bad request, requestID %s error %s",
			requestID,
			err.Error(),
		)

		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("failed to parse multipart form"))
		return
	}

	// 1. Process Files
	files := form.File["files"]

	log.Printf(
		"[UploadFile] processing files request_id=%s file_count=%d",
		requestID,
		len(files),
	)

	var fileContents []application.FileContent

	for _, fileHeader := range files {
		log.Printf(
			"[UploadFile] reading file request_id=%s filename=%s size=%d type=%s",
			requestID,
			fileHeader.Filename,
			fileHeader.Size,
			fileHeader.Header.Get("Content-Type"),
		)

		file, err := fileHeader.Open()
		if err != nil {
			log.Printf(
				"[UploadFile] failed opening file request_id=%s filename=%s error=%v",
				requestID,
				fileHeader.Filename,
				err,
			)
			continue
		}

		data, err := io.ReadAll(file)
		file.Close()

		if err != nil {
			log.Printf(
				"[UploadFile] failed reading file request_id=%s filename=%s error=%v",
				requestID,
				fileHeader.Filename,
				err,
			)
			continue
		}

		fileContents = append(fileContents, application.FileContent{
			Name: fileHeader.Filename,
			Size: fileHeader.Size,
			Type: fileHeader.Header.Get("Content-Type"),
			Data: data,
		})
	}

	log.Printf(
		"[UploadFile] files loaded request_id=%s valid_files=%d",
		requestID,
		len(fileContents),
	)

	// Parse JSON fields from form
	var destSettings application.DestinationSettings
	if ds := c.PostForm("destinationSettings"); ds != "" {
		if err := json.Unmarshal([]byte(ds), &destSettings); err != nil {
			log.Printf(
				"[UploadFile] failed parsing destinationSettings request_id=%s error=%v",
				requestID,
				err,
			)
		}
	}

	var properties application.UploadProperties
	if p := c.PostForm("properties"); p != "" {
		if err := json.Unmarshal([]byte(p), &properties); err != nil {
			log.Printf(
				"[UploadFile] failed parsing properties request_id=%s error=%v",
				requestID,
				err,
			)
		}
	}

	var tags []application.Tag
	if t := c.PostForm("tags"); t != "" {
		if err := json.Unmarshal([]byte(t), &tags); err != nil {
			log.Printf(
				"[UploadFile] failed parsing tags request_id=%s error=%v",
				requestID,
				err,
			)
		}
	}

	var metadata []application.MetadataItem
	if m := c.PostForm("metadata"); m != "" {
		if err := json.Unmarshal([]byte(m), &metadata); err != nil {
			log.Printf(
				"[UploadFile] failed parsing metadata request_id=%s error=%v",
				requestID,
				err,
			)
		}
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

	log.Printf(
		"[UploadFile] calling upload service request_id=%s bucket_id=%s prefix=%s",
		requestID,
		bucketID,
		prefix,
	)

	output, err := h.uploadService.UploadFile(c.Request.Context(), input)
	if err != nil {
		log.Printf(
			"[Handler:UploadFile] Service call, requestID %s error=%v duration=%s",
			requestID,
			err,
			time.Since(start),
		)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to upload files"))
		return
	}

	log.Printf(
		"[Handler:UploadFile] completed request_id=%s uploaded_files=%d duration=%s",
		requestID,
		len(output.FileIDs),
		time.Since(start),
	)

	utils.RespondSucces(c, http.StatusCreated, "files uploaded successfully", output)
}

func (h *HandlerForFiles) UploadFilePresign(c *gin.Context) {
	start := time.Now()
	requestID := c.GetString("requestId")

	peek, _ := io.ReadAll(c.Request.Body)
	log.Printf("BODY LENGTH AT HANDLER ENTRY: %d", len(peek))
	c.Request.Body = io.NopCloser(bytes.NewBuffer(peek)) // restore so rest of handler still works
	fmt.Printf("======>1")

	// 1. Extract token from query
	token := c.Query("t")
	if token == "" {
		log.Printf("[Handler:UploadFilePresign] Bad request, requestID %s error missing token", requestID)
		utils.RespondError(c, http.StatusUnauthorized, fmt.Errorf("authentication token required"))
		return
	}

	// 2. Split token into payload and signature
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		log.Printf("[Handler:UploadFilePresign] Bad request, requestID %s error invalid token format", requestID)
		utils.RespondError(c, http.StatusUnauthorized, fmt.Errorf("invalid token format"))
		return
	}
	// 2. Split token into payload and signature
	// parts := strings.Split(token, ".")
		payloadEncoded := parts[0]
	signature := parts[1]

	expectedSignature := h.signString( payloadEncoded)

	if !hmac.Equal([]byte(expectedSignature), []byte(signature)) {
		log.Printf("[Handler:UploadFilePresign] Bad request, requestID %s error invalid signature", requestID)
		utils.RespondError(c, http.StatusForbidden, fmt.Errorf("invalid or expired token"))
		return
	}

	// 4. Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		log.Printf("[Handler:UploadFilePresign] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid token format"))
		return
	}

	var payload struct {
		URLID     string `json:"u"`
		BucketID  string `json:"b"`
		Key       string `json:"k"`
		AssetID   string `json:"a"`
		UserID    string `json:"uid"`
		SHA256    string `json:"sha"`
		Method    string `json:"m"`
		ExpiresAt int64  `json:"e"`
	}

	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		log.Printf("[Handler:UploadFilePresign] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid token format"))
		return
	}

	// 5. Check expiry

	if time.Now().Unix() > payload.ExpiresAt {
		log.Printf("[Handler:UploadFilePresign] Bad request, requestID %s error token expired", requestID)
		utils.RespondError(c, http.StatusForbidden, fmt.Errorf("token has expired"))
		return
	}

	// 6. Check method
	if c.Request.Method != payload.Method {
		log.Printf("[Handler:UploadFilePresign] Bad request, requestID %s error method mismatch", requestID)
		utils.RespondError(c, http.StatusMethodNotAllowed, fmt.Errorf("invalid request method"))
		return
	}

	log.Printf(
		"[UploadFilePresign] valid token request_id=%s user_id=%s bucket_name=%s key=%s",
		requestID, payload.UserID, payload.BucketID, payload.Key,
	)
	fmt.Printf("======>31")

	// 7. Read raw body as file data
	data, err := io.ReadAll(c.Request.Body)

	if err != nil {
		log.Printf("[Handler:UploadFilePresign] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to read upload data"))
		return
	}

	// 8. Call UploadService
	input := application.UploadFileInput{
		Sha256: payload.SHA256,

		BucketID: payload.BucketID,
		Files: []application.FileContent{
			{
				Name: payload.Key,
				Size: int64(len(data)),
				Type: "application/octet-stream", // or sniff from headers
				Data: data,
			},
		},
		Metadata: []application.MetadataItem{
			{Key: "asset_id", Value: payload.AssetID},
			{Key: "original_sha256", Value: payload.SHA256},
		},
	}

	// Set userId in context for the service
	ctx := context.WithValue(c.Request.Context(), "userId", payload.UserID)

	output, err := h.uploadService.UploadFile(ctx, input)
	if err != nil {
		log.Printf("[Handler:UploadFilePresign] Service call, requestID %s error=%v", requestID, err)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to upload file"))
		return
	}

	log.Printf(
		"[Handler:UploadFilePresign] upload completed request_id=%s file_id=%s duration=%s",
		requestID, output.FileIDs[0], time.Since(start),
	)

	utils.RespondSucces(c, http.StatusCreated, "file uploaded successfully", output)
}

func (h *HandlerForFiles) ExportBucket(c *gin.Context) {
	requestID := c.GetString("requestId")
	bucketID := c.Param("bucketId")

	output, err := h.uploadService.ExportBucket(c, bucketID)
	if err != nil {
		log.Printf("[Handler:ExportBucket] Service call, requestID %s error=%v", requestID, err)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to export bucket"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "bucket exported successfully", output)
}

func (h *HandlerForFiles) ExportFolder(c *gin.Context) {
	requestID := c.GetString("requestId")
	bucketID := c.Param("bucketId")
	folderPath := c.Param("folderPath")

	if folderPath == "" {
		log.Printf("[Handler:ExportFolder] Bad request, requestID %s error missing folderPath", requestID)
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("folder path is required"))
		return
	}

	output, err := h.uploadService.ExportFolder(c, folderPath, bucketID)
	if err != nil {
		log.Printf("[Handler:ExportFolder] Service call, requestID %s error=%v", requestID, err)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to export folder"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "folder exported successfully", output)
}

type SelectFiles struct {
	Files []string `json:"files"`
}

func (h *HandlerForFiles) ExportSelect(c *gin.Context) {
	requestID := c.GetString("requestId")
	bucketID := c.Param("bucketId")

	var req SelectFiles
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("[Handler:ExportSelect] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.uploadService.ExportSelect(c, req.Files, bucketID)
	if err != nil {
		log.Printf("[Handler:ExportSelect] Service call, requestID %s error=%v", requestID, err)
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to export files"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "files exported successfully", output)
}

// CreateFolder handles explicit folder creation (0-byte object)
// POST /buckets/:bucketId/folders
func (h *HandlerForFiles) CreateFolder(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	var input struct {
		Name string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CreateFolder] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	if err := h.uploadService.CreateFolder(c.Request.Context(), bucketID, input.Name); err != nil {
		log.Printf("[Handler:CreateFolder] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to create folder"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "folder created successfully", gin.H{"status": "created"})
}

type ResponseStruct struct {
	Code    int                       `json:"code"`
	Message string                    `json:"message"`
	Data    dto.BucketStructureOutput `json:"data"`
}

// ListFiles handles listing files in a bucket
// GET /buckets/:bucketId/files
func (h *HandlerForFiles) ListFiles(c *gin.Context) {
	bucketID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	files, err := h.uploadService.ListFiles(c.Request.Context(), bucketID)
	if err != nil {
		log.Printf("[Handler:ListFiles] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to list files"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "files listed successfully", gin.H{
		"bucketId": bucketID,
		"count":    files.RootFiles,
		"files":    files,
	})
}

// DeleteFile handles file deletion
// DELETE /buckets/:bucketId/files/:fileId
func (h *HandlerForFiles) DeleteFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")
	requestID := c.GetString("requestId")

	err := h.deleteService.DeleteFile(c.Request.Context(), application.DeleteFileInput{
		FileID:   fileID,
		BucketID: bucketID,
	})
	if err != nil {
		log.Printf("[Handler:DeleteFile] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to delete file"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "file deleted successfully", gin.H{"fileId": fileID, "status": "deleted"})
}

// GetFileInfo handles getting file metadata
// GET /:bucketId/files/:fileId
func (h *HandlerForFiles) GetFileInfo(c *gin.Context) {
	bucketID := c.Param("bucketId")
	folder := c.Query("folder")
	requestID := c.GetString("requestId")
	fileID := fmt.Sprintf("%s/%s", folder, c.Param("fileId"))

	if folder == "" {
		fileID = strings.Split(strings.Split(fileID, "/")[1], "}")[0]
	}

	output, err := h.uploadService.GetFileInfo(c.Request.Context(), bucketID, fileID)
	if err != nil {
		log.Printf("[Handler:GetFileInfo] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("file not found"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "file info retrieved successfully", output)
}

func (h *HandlerForFiles) GetFileInfoFolder(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")
	folderID := c.Param("bucketId")
	requestID := c.GetString("requestId")

	output, err := h.uploadService.GetFileInfoFolder(c.Request.Context(), bucketID, fileID, folderID)
	if err != nil {
		log.Printf("[Handler:GetFileInfoFolder] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("file not found"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "file info retrieved successfully", output)
}

func (h *HandlerForFiles) signString(data string) string {
	mac := hmac.New(sha256.New, []byte(h.secretKey))
	mac.Write([]byte(data))
	return hex.EncodeToString(mac.Sum(nil))
}

// func (h *HandlerForFiles) ValidateSignature(key, method, signature string, expiresAt int64) error {
// 	// Check expiry first — cheap check before crypto
// 	if time.Now().Unix() > expiresAt {
// 		return fmt.Errorf("presigned URL has expired")
// 	}

// 	expected := h.signString(key + method + fmt.Sprintf("%d", expiresAt))

// 	// Constant-time comparison to prevent timing attacks
// 	if !hmac.Equal([]byte(expected), []byte(signature)) {
// 		return fmt.Errorf("invalid signature")
// 	}

// 	return nil
// }

type SignedPayload struct {
	A   string `json:"a"`   // asset_id
	B   string `json:"b"`   // bucket_id
	E   int64  `json:"e"`   // expires_at
	K   string `json:"k"`   // key
	M   string `json:"m"`   // method
	S   string `json:"s"`   // sha256
	U   string `json:"u"`   // user_id
	UId string `json:"uid"` // user_id
}

// DownloadFile handles file download with presigned URL signature validation.
// GET /:bucketId/files/:fileId/download?signature=xxx&expires=unix
func (h *HandlerForFiles) DownloadFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")
	userID := c.GetString("userId")
	requestID := c.GetString("requestId")

	token := c.Query("t")

	if token == "" {
		log.Printf("[Handler:DownloadFile] Bad request, requestID %s error missing token", requestID)
		utils.RespondError(c, http.StatusUnauthorized, fmt.Errorf("authentication token required"))
		return
	}

	payload, err := h.ValidateSignature(token, http.MethodGet)
	if err != nil {
		log.Printf("[Handler:DownloadFile] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusForbidden, fmt.Errorf("invalid or expired token"))
		return
	}

	_ = payload

	fileData, metadata, err := h.uploadService.DownloadFile(
		c.Request.Context(),
		bucketID,
		fileID,
		userID,
	)

	if err != nil {
		log.Printf("[Handler:DownloadFile] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusNotFound, fmt.Errorf("file not found"))
		return
	}

	c.Header(
		"Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s\"", metadata.Key),
	)

	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", metadata.Size))

	c.Data(http.StatusOK, metadata.MimeType, fileData)

	c.Header(
		"Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s\"", metadata.Key),
	)

	c.Header("Content-Type", metadata.MimeType)
	c.Header("Content-Length", fmt.Sprintf("%d", metadata.Size))

	c.Data(http.StatusOK, metadata.MimeType, fileData)
}

func (h *HandlerForFiles) ValidateSignature(
	token string,
	expectedMethod string,
) (*SignedPayload, error) {

	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, errors.New("invalid token format")
	}

	payloadEncoded := parts[0]
	providedSig := parts[1]

	// Recompute signature
	expectedSig := h.signString(payloadEncoded)

	if !hmac.Equal([]byte(providedSig), []byte(expectedSig)) {
		return nil, errors.New("invalid signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadEncoded)
	if err != nil {
		return nil, errors.New("invalid payload encoding")
	}

	var payload SignedPayload

	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, errors.New("invalid payload")
	}

	// Expiry check
	if time.Now().Unix() > payload.E {
		return nil, errors.New("token expired")
	}

	// Method check
	if payload.M != expectedMethod {
		return nil, errors.New("invalid method")
	}

	return &payload, nil
}

// UpdateFileMetadata handles updating file metadata
// PATCH /:bucketId/files/:fileId
func (h *HandlerForFiles) UpdateFileMetadata(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")
	requestID := c.GetString("requestId")

	var input dto.UpdateFileMetadataInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:UpdateFileMetadata] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.uploadService.UpdateFileMetadata(c.Request.Context(), bucketID, fileID, input)
	if err != nil {
		log.Printf("[Handler:UpdateFileMetadata] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to update file metadata"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "file metadata updated successfully", output)
}

// CopyFile handles copying a file
// POST /:bucketId/files/:fileId/copy
func (h *HandlerForFiles) CopyFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")
	requestID := c.GetString("requestId")

	var input dto.CopyFileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:CopyFile] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.uploadService.CopyFile(c.Request.Context(), bucketID, fileID, input)
	if err != nil {
		log.Printf("[Handler:CopyFile] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to copy file"))
		return
	}

	utils.RespondSucces(c, http.StatusCreated, "file copied successfully", output)
}

// MoveFile handles moving a file to another bucket
// POST /:bucketId/files/:fileId/move
func (h *HandlerForFiles) MoveFile(c *gin.Context) {
	bucketID := c.Param("bucketId")
	fileID := c.Param("fileId")
	requestID := c.GetString("requestId")

	var input dto.MoveFileInput
	if err := c.ShouldBindJSON(&input); err != nil {
		log.Printf("[Handler:MoveFile] Bad request, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusBadRequest, fmt.Errorf("invalid JSON payload"))
		return
	}

	output, err := h.uploadService.MoveFile(c.Request.Context(), bucketID, fileID, input)
	if err != nil {
		log.Printf("[Handler:MoveFile] Service call, requestID %s error %s", requestID, err.Error())
		utils.RespondError(c, http.StatusInternalServerError, fmt.Errorf("failed to move file"))
		return
	}

	utils.RespondSucces(c, http.StatusOK, "file moved successfully", output)
}
