package http

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "os"
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
	prefixName := c.Query("prefix")

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
	var folder_id string
	folder_id = c.PostForm("prefix_id"); 
	if folder_id != "" {
		 
			log.Printf(
				"[UploadFile] failed parsing metadata request_id=%s error=%v",
				requestID,
				err,
			)
		
	}


	fmt.Printf("89898977 %v\n", folder_id)



	input := application.UploadFileInput{
		BucketID:            bucketID,
		Prefix:              prefixName,
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
		prefixName,
	)

	output, err := h.uploadService.UploadFile(c.Request.Context(), input, prefixName,userID)
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

	expectedSignature := h.signString(payloadEncoded)

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
		log.Printf("[Handler:UploadFilePresign] Service call, requestID!! %s error %s", requestID, err.Error())
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

	output, err := h.uploadService.UploadFile(ctx, input, "root",payload.UserID)
	if err != nil {
		log.Printf("[Handler:UploadFilePresign] Service call, requestID$$ %s error=%v", requestID, err)
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
	// TODO: inserting the *gin.Context as aparameter is glue
	// code it should be removed
	files, err := h.uploadService.ListFiles(c.Request.Context(), c, bucketID)
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

 


// DownloadFile3 handles direct authenticated file downloads.
//
// GET /:bucketId/files/:fileId/download-direct
//
// Unlike DownloadFile2, this endpoint does not use a presigned
// token. Authentication is handled by the existing middleware,
// which provides the userId through the Gin context.
func (h *HandlerForFiles) DownloadFile3(c *gin.Context) {
    bucketID := c.Param("bucketId")
    fileID := c.Param("fileId")
    userID := c.GetString("userId")
    requestID := c.GetString("requestId")

    if userID == "" {
        log.Printf(
            "[Handler:DownloadFile3] Missing userId requestID=%s",
            requestID,
        )

        utils.RespondError(
            c,
            http.StatusUnauthorized,
            fmt.Errorf("user authentication required"),
        )
        return
    }

    log.Printf(
        "[Handler:DownloadFile3] Starting direct download requestID=%s bucketID=%s fileID=%s userID=%s",
        requestID,
        bucketID,
        fileID,
        userID,
    )

    object, metadata, err := h.uploadService.DownloadFile2(
        c.Request.Context(),
        bucketID,
        fileID,
        userID,
    )

    if err != nil {
        log.Printf(
            "[Handler:DownloadFile3] Service error requestID=%s bucketID=%s fileID=%s error=%v",
            requestID,
            bucketID,
            fileID,
            err,
        )

        utils.RespondError(
            c,
            http.StatusNotFound,
            fmt.Errorf("file not found"),
        )
        return
    }

    defer object.Close()

    c.Header(
        "Content-Disposition",
        fmt.Sprintf(
            `attachment; filename="%s"`,
            metadata.Key,
        ),
    )

    c.Header(
        "Content-Type",
        metadata.MimeType,
    )

    c.Header(
        "Content-Length",
        fmt.Sprintf("%d", metadata.Size),
    )

    log.Printf(
        "[Handler:DownloadFile3] Streaming file requestID=%s fileID=%s size=%d",
        requestID,
        metadata.FileID,
        metadata.Size,
    )

    c.Status(http.StatusOK)

    _, err = io.Copy(c.Writer, object)

    if err != nil {
        log.Printf(
            "[Handler:DownloadFile3] Streaming failed requestID=%s fileID=%s error=%v",
            requestID,
            metadata.FileID,
            err,
        )
        return
    }

    log.Printf(
        "[Handler:DownloadFile3] Streaming completed requestID=%s fileID=%s",
        requestID,
        metadata.FileID,
    )
}









// DownloadFile2 handles streaming file downloads.
// GET /:bucketId/files/:fileId/download2?t=xxx
func (h *HandlerForFiles) DownloadFile2(c *gin.Context) {
    bucketID := c.Param("bucketId")
    fileID := c.Param("fileId")
    requestID := c.GetString("requestId")

    token := c.Query("t")

    if token == "" {
        log.Printf(
            "[Handler:DownloadFile2] Missing token requestID=%s",
            requestID,
        )

        utils.RespondError(
            c,
            http.StatusUnauthorized,
            fmt.Errorf("authentication token required"),
        )
        return
    }

    payload, err := h.ValidateSignature(token, http.MethodGet)
    if err != nil {
        log.Printf(
            "[Handler:DownloadFile2] Invalid token requestID=%s error=%s",
            requestID,
            err.Error(),
        )

        utils.RespondError(
            c,
            http.StatusForbidden,
            fmt.Errorf("invalid or expired token"),
        )
        return
    }

    userID := payload.UId

    log.Printf(
        "[Handler:DownloadFile2] Starting streaming download requestID=%s bucketID=%s fileID=%s userID=%s",
        requestID,
        bucketID,
        fileID,
        userID,
    )

    object, metadata, err := h.uploadService.DownloadFile2(
        c.Request.Context(),
        bucketID,
        fileID,
        userID,
    )

    if err != nil {
        log.Printf(
            "[Handler:DownloadFile2] Service error requestID=%s error=%s",
            requestID,
            err.Error(),
        )

        utils.RespondError(
            c,
            http.StatusNotFound,
            fmt.Errorf("file not found"),
        )
        return
    }

    defer object.Close()

    c.Header(
        "Content-Disposition",
        fmt.Sprintf(
            `attachment; filename="%s"`,
            metadata.Key,
        ),
    )

    c.Header(
        "Content-Type",
        metadata.MimeType,
    )

    c.Header(
        "Content-Length",
        fmt.Sprintf("%d", metadata.Size),
    )

    log.Printf(
        "[Handler:DownloadFile2] Streaming file requestID=%s fileID=%s size=%d",
        requestID,
        metadata.FileID,
        metadata.Size,
    )

    c.Status(http.StatusOK)

    _, err = io.Copy(c.Writer, object)

    if err != nil {
        log.Printf(
            "[Handler:DownloadFile2] Streaming failed requestID=%s fileID=%s error=%s",
            requestID,
            metadata.FileID,
            err.Error(),
        )
        return
    }

    log.Printf(
        "[Handler:DownloadFile2] Streaming completed requestID=%s fileID=%s",
        requestID,
        metadata.FileID,
    )
}






























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
	// fileID := "b6384990-63c2-408b-9a4c-6709e93c7427"
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


	fmt.Printf("Payload used downloading the file %+v",payload)

// Payload used downloading the file &{
// A:91e7e999-4e4b-41f2-ae46-339e6443e5e6 
// B:371bfcf0-f0b4-4983-b0c4-22cca886d6fd 
// E:1789064025 
// K:371bfcf0-f0b4-4983-b0c4-22cca886d6fd/functions/91e7e999-4e4b-41f2-ae46-339e6443e5e6 
// M:GET 
// S:fd914edda5ce414784cb0dfb0d4036e223e52227efe901e11734063819aadea8 
// U:b0fc3323-b2ba-4020-88ae-feaf9a24b0b3 
// UId:00000000-0000-0000-0000-000000000000}

	userID=payload.UId





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

	fmt.Printf("\nsize according to metadata: %v, the actual file size\n", metadata.Size)


	c.Header(
		"Content-Disposition",
		fmt.Sprintf("attachment; filename=\"%s\"", metadata.Key),
	)

	c.Header("Content-Type", metadata.MimeType)
	// c.Header("Content-Length", fmt.Sprintf("%d", metadata.Size))

	// c.Data(http.StatusOK, metadata.MimeType, fileData)

	
	type FilesData struct{
		FileData []byte `json:"file_data"`
	}



	utils.RespondSucces(c, http.StatusOK, "folder exported successfully", FilesData{FileData: fileData})

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
	// TODO: Remove this comments, 
	// if time.Now().Unix() > payload.E {
	// 	return nil, errors.New("token expired")
	// }

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
