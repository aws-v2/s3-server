package http

import (
	"s3/internal/domain"
	"s3/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Handlers struct holds all handler dependencies
type Handlers struct {
	File         *HandlerForFiles
	Bucket       *BucketHandler
	Health       *HandlerForHealth
	Presign      *PresignHandler
	Batch        *BatchHandler
	Prefix       *PrefixHandler
	Search       *SearchHandler
	Webhook      *WebhookHandler
	Multipart    *MultipartHandler
	Analytics    *AnalyticsHandler
	AccessPoint  *AccessPointHandler
	Security     *SecurityHandler
	JWTValidator domain.JWTValidator
	Docs         *DocsHandler
}

// RegisterRoutes registers all application routes
func RegisterRoutes(router *gin.Engine, handlers *Handlers) {
	// API group (Gateway strips /api/v1)
	// router.Use(middleware.CORSMiddleware()) // Disabled: CORS handled by API Gateway
	v1 := router.Group("/api/v1/s3")

	v1.Use(middleware.AuthContextMiddleware())

	// Chain authentication middlewares: try API key first, then JWT
	// v1.Use(middleware.APIKeyAuthMiddleware(handlers.Validator))------------>>IAM
	// v1.Use(middleware.BearerAuthMiddleware(handlers.JWTValidator))------------>>IAM

	// Track all V1 requests
	v1.Use(handlers.Analytics.TrackRequestMiddleware())

	// Register domain-specific routes
	registerObjectRoutes(v1, handlers.File)
	registerBucketRoutes(v1, handlers)
	registerHealthRoutes(v1, handlers.Health)
	registerWebhookRoutes(v1, handlers.Webhook)
	registerMultipartRoutes(v1, handlers.Multipart)
	registerAnalyticsRoutes(v1, handlers.Analytics)
	registerPresignRoutes(v1, handlers.Presign)
	registerBatchRoutes(v1, handlers.Batch)
	registerSearchRoutes(v1, handlers.Search)
	registerPrefixRoutes(v1, handlers.Prefix)
	registerSecurityRoutes(v1, handlers.Security)

	registerDocsRoutes(v1, handlers)

}

// registerHealthRoutes registers all health check routes
func registerHealthRoutes(v1 *gin.RouterGroup, handler *HandlerForHealth) {

	health := v1.Group("/health")
	{
		health.GET("/ping", handler.Ping)
		health.GET("/status", handler.GetDetailedStatus)
		health.GET("/metrics", handler.GetMetrics)
	}
}
func registerDocsRoutes(v1 *gin.RouterGroup, handlers *Handlers) {
	docs := v1.Group("/docs")
	{
		docs.GET("", handlers.Docs.GetPublicManifest)
		docs.GET("/:slug", handlers.Docs.GetPublicDoc)
	}
	internal := v1.Group("/internal/docs")

	// 🔐 protect internal docs
	// internal.Use(middleware.BearerAuthMiddleware(handlers.JWTValidator))

	{
		internal.GET("", handlers.Docs.GetInternalManifest)
		internal.GET("/:slug", handlers.Docs.GetInternalDoc)
	}
}

// registerFileRoutes registers all file-related routes
func registerObjectRoutes(v1 *gin.RouterGroup, handler *HandlerForFiles) {
	object := v1.Group("/files")
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)

	// object.Use(middleware.APIKeyAuthMiddleware(validator))------------>>IAM

	{
		// Upload file to bucket
		object.POST("/upload/:bucketId",
			middleware.AllowedFileTypesMiddleware(),
			// middleware.MaxFileSizeMiddleware(10<<20),
			middleware.MaxFileSizeMiddleware(1*GB),

			handler.UploadFile)

		// Create direct folder
		object.POST("/folders/:bucketId", handler.CreateFolder)

		// List files in bucket
		object.GET("/:bucketId", handler.ListFiles)

		// Get file info/metadata
		object.GET("/:bucketId/files/:fileId", handler.GetFileInfo)

		// // Download file
		object.GET("/:bucketId/files/:fileId/download", handler.DownloadFile)

		// Delete file
		object.DELETE("/:bucketId/files/:fileId", handler.DeleteFile)

		// Update file metadata
		object.PATCH("/:bucketId/files/:fileId", handler.UpdateFileMetadata)

		// Copy file
		object.POST("/:bucketId/files/:fileId/copy", handler.CopyFile)

		// Move file
		object.POST("/:bucketId/files/:fileId/move", handler.MoveFile)
	}
}

// registerBucketRoutes registers all bucket management routes
func registerBucketRoutes(v1 *gin.RouterGroup, handlers *Handlers) {
	buckets := v1.Group("/buckets")
	handler := handlers.Bucket

	{
		// Create new bucket
		buckets.POST("/create-bucket", handler.CreateBucket)
		// List all buckets
		buckets.GET("", handler.ListBuckets)
		// // Get bucket info
		buckets.GET("/:bucketId", handler.GetBucketInfo)
		// // Update bucket settings
		buckets.PATCH("/:bucketId", handler.UpdateBucket)
		// // Delete bucket
		buckets.DELETE("/:bucketId", handler.DeleteBucket)
		// // Get bucket statistics
		buckets.GET("/:bucketId/stats", handler.GetBucketStats)
		// // Get bucket policy
		buckets.GET("/:bucketId/policy", handler.GetBucketPolicy)
		// // Update bucket policy
		buckets.PUT("/:bucketId/policy", handler.UpdateBucketPolicy)
		// // Enable/disable bucket versioning
		buckets.PUT("/:bucketId/versioning", handler.SetBucketVersioning)
		buckets.GET("/:bucketId/versioning", handler.GetBucketVersioning)
		// CORS (No Auth for local testing)
		buckets.GET("/:bucketId/cors", handler.GetBucketCORS)
		buckets.PUT("/:bucketId/cors", handler.UpdateBucketCORS)

		// Block Public Access
		buckets.PUT("/:bucketId/block-public-access", handler.SetBucketBlockPublicAccess)

		// // Set bucket lifecycle rules
		buckets.PUT("/:bucketId/lifecycle", handler.SetBucketLifecycle)
		buckets.GET("/:bucketId/lifecycle", handler.GetBucketLifecycle)

		// Encryption
		buckets.PUT("/:bucketId/encryption", handler.SetBucketEncryption)
		buckets.GET("/:bucketId/encryption", handler.GetBucketEncryption)
		// Replication
		buckets.GET("/:bucketId/replication", handler.GetBucketReplication)
		buckets.POST("/:bucketId/replication", handler.UpdateBucketReplication)
		// Tags
		buckets.GET("/:bucketId/tags", handler.GetBucketTags)
		buckets.PUT("/:bucketId/tags", handler.UpdateBucketTags)
		// Notifications
		buckets.GET("/:bucketId/notifications", handler.GetBucketNotifications)
		buckets.POST("/:bucketId/notifications", handler.UpdateBucketNotifications)
		// Logging
		buckets.PUT("/:bucketId/logging", handler.UpdateBucketLogging)
		buckets.GET("/:bucketId/logging", handler.GetBucketLogging)
		// Object Lock
		buckets.PUT("/:bucketId/object-lock", handler.SetBucketObjectLock)
		buckets.GET("/:bucketId/object-lock", handler.GetBucketObjectLock)

		// Access Points
		buckets.POST("/:bucketId/access-points", handlers.AccessPoint.CreateAccessPoint)
		buckets.GET("/:bucketId/access-points", handlers.AccessPoint.ListAccessPoints)
		buckets.PUT("/:bucketId/access-points/:accessPointName", handlers.AccessPoint.UpdateAccessPointOrigin)

		// Empty bucket
		buckets.POST("/:bucketId/empty", handler.EmptyBucket)
	}
}

func registerSecurityRoutes(v1 *gin.RouterGroup, handler *SecurityHandler) {
	security := v1.Group("/security")
	{
		security.GET("/summary", handler.GetSecuritySummary)
	}
}

// TODO: IMPLEMENT MILTIPART FOR PRESIGNED URLS
func registerPresignRoutes(v1 *gin.RouterGroup, handler *PresignHandler) {
	presign := v1.Group("/presign")
	{
		// Generic object handler for presigned URLs
		v1.PUT("/buckets/:bucketId/objects/*key", handler.HandleObjectUpload)

		// Generate presigned URL for upload
		presign.POST("/:bucketId/upload", handler.GenerateUploadURL)

		// 		// Generate presigned URL for download
		presign.POST("/:bucketId/files/:fileId/download", handler.GenerateDownloadURL)

		// 		// Revoke presigned URL
		presign.DELETE("/urls/:urlId", handler.RevokePresignedURL)

		// 		// List active presigned URLs
		presign.GET("/urls", handler.ListPresignedURLs)

		// 		// Validate presigned URL
		presign.POST("/validate", handler.ValidatePresignedURL)
		// 		// Generate presigned URL for multipart upload
		// Generate multipart upload URLs
		presign.POST("/:bucketId/multipart", handler.GenerateMultipartUploadURLs)

		// Complete multipart upload
		presign.POST("/:bucketId/multipart/:uploadId/complete", handler.CompleteMultipartUpload)

		// Abort multipart upload
		presign.DELETE("/:bucketId/multipart/:uploadId", handler.AbortMultipartUpload)

		// List multipart parts
		presign.GET("/:bucketId/multipart/:uploadId/parts", handler.ListMultipartUploadParts)
	}
}

// ===================================********************************=================================================================

func registerBatchRoutes(v1 *gin.RouterGroup, handler *BatchHandler) {
	batch := v1.Group("/batch")
	{
		// Batch upload files
		batch.POST("/upload", handler.BatchUpload)

		// Batch delete files
		batch.DELETE("/delete", handler.BatchDelete)

		// Batch copy files
		batch.POST("/copy", handler.BatchCopy)

		// Batch move files
		batch.POST("/move", handler.BatchMove)

		// Batch update metadata
		batch.PATCH("/metadata", handler.BatchUpdateMetadata)

		// Get batch operation status
		batch.GET("/operations/:operationId", handler.GetBatchOperationStatus)

		// List batch operations
		batch.GET("/operations", handler.ListBatchOperations)

		// Cancel batch operation
		batch.DELETE("/operations/:operationId", handler.CancelBatchOperation)
	}
}

// registerPrefixRoutes registers prefix-based operation routes
func registerPrefixRoutes(v1 *gin.RouterGroup, handler *PrefixHandler) {
	prefix := v1.Group("/prefix")
	{
		// List files by prefix
		prefix.GET("/:bucketId/list", handler.ListByPrefix)

		// Delete files by prefix
		prefix.DELETE("/:bucketId/delete", handler.DeleteByPrefix)

		// Copy files by prefix
		prefix.POST("/:bucketId/copy", handler.CopyByPrefix)

		// Get total size of files by prefix
		prefix.GET("/:bucketId/size", handler.GetSizeByPrefix)

		// Count files by prefix
		prefix.GET("/:bucketId/count", handler.CountByPrefix)

		// Archive files by prefix (zip/tar)
		prefix.POST("/:bucketId/archive", handler.ArchiveByPrefix)

		// Set metadata for files by prefix
		prefix.PATCH("/:bucketId/metadata", handler.SetMetadataByPrefix)
	}
}

// registerSearchRoutes registers search and query routes
func registerSearchRoutes(v1 *gin.RouterGroup, handler *SearchHandler) {
	search := v1.Group("/search")
	{
		// Search files by name
		search.GET("/files", handler.SearchFiles)

		// Search by metadata
		search.GET("/metadata", handler.SearchByMetadata)

		// Search by tags
		search.GET("/tags", handler.SearchByTags)

		// Search by content (full-text search)
		search.GET("/content", handler.SearchByContent)

		// Advanced search with filters
		search.POST("/advanced", handler.AdvancedSearch)

		// Search suggestions/autocomplete
		search.GET("/suggestions", handler.GetSearchSuggestions)

		// Get recent searches
		search.GET("/history", handler.GetSearchHistory)

		// Save search query
		search.POST("/save", handler.SaveSearch)
	}
}

func registerAnalyticsRoutes(v1 *gin.RouterGroup, handler *AnalyticsHandler) {
	analytics := v1.Group("/analytics")
	{
		// Get storage usage statistics
		analytics.GET("/storage/usage", handler.GetStorageUsage)

		// Get upload/download statistics
		analytics.GET("/traffic", handler.GetTrafficStats)

		// Get file type distribution
		analytics.GET("/files/types", handler.GetFileTypeDistribution)

		// Get bucket usage over time
		analytics.GET("/buckets/:bucketId/usage", handler.GetBucketUsageOverTime)

		// Get most accessed files
		analytics.GET("/files/popular", handler.GetPopularFiles)

		// Get user activity report
		analytics.GET("/users/:userId/activity", handler.GetUserActivity)

		// Export analytics data
		analytics.GET("/export", handler.ExportAnalytics)

		// Get API usage statistics
		analytics.GET("/api/usage", handler.GetAPIUsage)

		// Storage Lens
		analytics.GET("/storage/lens", handler.GetStorageLensData)
	}
}

func registerMultipartRoutes(v1 *gin.RouterGroup, handler *MultipartHandler) {
	multipart := v1.Group("/multipart")
	{
		// Initiate multipart upload
		multipart.POST("/:bucketId/initiate", handler.InitiateMultipartUpload)

		// Upload a part
		multipart.PUT("/:bucketId/:uploadId/parts/:partNumber", handler.UploadPart)

		// Complete multipart upload
		multipart.POST("/:bucketId/:uploadId/complete", handler.CompleteMultipartUpload)

		// Abort multipart upload
		multipart.DELETE("/:bucketId/:uploadId", handler.AbortMultipartUpload)

		// List parts of multipart upload
		multipart.GET("/:bucketId/:uploadId/parts", handler.ListParts)

		// List in-progress multipart uploads
		multipart.GET("/:bucketId/uploads", handler.ListMultipartUploads)
	}
}

func registerWebhookRoutes(v1 *gin.RouterGroup, handler *WebhookHandler) {
	webhooks := v1.Group("/webhooks")
	{
		// Create webhook
		webhooks.POST("", handler.CreateWebhook)

		// List webhooks for a bucket
		webhooks.GET("/bucket/:bucketId", handler.ListWebhooks)

		// Get webhook details
		webhooks.GET("/:webhookId", handler.GetWebhook)

		// Update webhook
		webhooks.PATCH("/:webhookId", handler.UpdateWebhook)

		// Delete webhook
		webhooks.DELETE("/:webhookId", handler.DeleteWebhook)

		// Test webhook
		webhooks.POST("/:webhookId/test", handler.TestWebhook)

		// Get webhook delivery logs
		webhooks.GET("/:webhookId/deliveries", handler.GetWebhookDeliveries)
	}
}
