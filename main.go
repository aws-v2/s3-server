package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"s3/internal/application"

	// "s3/internal/infrastructure/database"
	"s3/internal/infrastructure/repository"
	"s3/internal/infrastructure/database"

	"s3/internal/infrastructure/storage"
	"s3/internal/infrastructure/event"
	"s3/internal/middleware"

	"s3/internal/infrastructure/system"
	"s3/internal/transport/http"
	"s3/internal/utils"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {

	cfg, err := utils.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Println("Initializing MinIO adapter...")
	minioAdapter, err := storage.NewMinIOAdapter(
		cfg.S3.Endpoint,
		cfg.S3.AccessKey,
		cfg.S3.SecretKey,
		cfg.S3.UseSSL,
	)
	if err != nil {
		log.Fatalf("Failed to create MinIO adapter: %v", err)
	}

	serverPort := getEnv("SERVER_PORT", "8080")

	dbConfig := database.Config{
		Host:            cfg.DB.Host,
		Port:            cfg.DB.Port,
		User:            cfg.DB.User,
		Password:        cfg.DB.Password,
		Database:        cfg.DB.Database,
		SSLMode:         cfg.DB.SSLMode,
		MaxOpenConns:    cfg.DB.MaxOpenConns,
		MaxIdleConns:    cfg.DB.MaxIdleConns,
		ConnMaxLifetime: cfg.DB.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DB.ConnMaxIdleTime,
	}

	log.Println("Connecting to PostgreSQL...")
	db, err := database.NewPostgresDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	go monitorDBStats(db)
	postgresRepo := repository.NewPostgresRepository(db)

	// Initialize NATS connection for IAM integration
	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	log.Printf("Connecting to NATS at %s...", natsURL)
	natsAdapter, err := event.NewNATSAdapter(natsURL)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsAdapter.Close()

	// Create IAM validator
	iamValidator := middleware.NewIAMValidator(natsAdapter.GetConnection())

	// // Run migrations
	// log.Println("Running database migrations...")
	// if err := database.RunMigrations(db, dbConfig.Database); err != nil {
	// 	log.Fatalf("1Failed to run migrations: %v", err)
	// }
	// log.Println("Migrations completed successfully")

	// // Check current migration version (optional)
	// version, dirty, err := database.GetMigrationVersion(db, dbConfig.Database)
	// if err != nil {
	// 	log.Printf("Warning: Failed to get migration version: %v", err)
	// } else {
	// 	log.Printf("Current migration version: %d (dirty: %t)", version, dirty)
	// }

	sys := &system.System{} // note pointer, so methods can be called

	// 2. Initialize Application Layer (Services)
	log.Println("Initializing services...")
	uploadService := application.NewUploadService(minioAdapter, postgresRepo)
	bucketService := application.NewBucketService(postgresRepo, minioAdapter)
	deleteService := application.NewDeleteService(minioAdapter, postgresRepo)
	healthService := application.NewHealthService(postgresRepo, minioAdapter, sys)
	presignedService := application.NewPresignService(postgresRepo, minioAdapter, "sys")
	batchService := application.NewBatchService(postgresRepo, minioAdapter)
	prefixService := application.NewPrefixService(postgresRepo, minioAdapter)
	SearchService := application.NewSearchService(postgresRepo)
	webhookService := application.NewWebhookService(postgresRepo)
	analyticsService := application.NewAnalyticsService(postgresRepo)
	multipartService := application.NewMultipartService(postgresRepo,minioAdapter)

	// 3. Initialize Transport Layer (HTTP)
	log.Println("Initializing HTTP handlers...")
	handlers := &http.Handlers{
		File:      http.NewFileHandler(uploadService, deleteService),
		Bucket:    http.NewBucketHandler(bucketService),
		Health:    http.NewHealthHandler(healthService),
		Presign:   http.NewPresignHandler(presignedService),   // TODO: implement later
		Batch:     http.NewBatchHandler(batchService),         // TODO: implement later
		Prefix:    http.NewPrefixHandler(prefixService),       // TODO: implement later
		Search:    http.NewSearchHandler(SearchService),       // TODO: implement later
		Webhook:   http.NewWebhookHandler(webhookService),     // TODO: implement later
		Analytics: http.NewAnalyticsHandler(analyticsService), // TODO: implement later
		Multipart: http.NewMultipartHandler(multipartService), // TODO: implement later
		Validator: iamValidator,                               // IAM validator for authentication

	}

	// 4. Setup Router
	router := gin.Default()
	http.RegisterRoutes(router, handlers)

	// 5. Start Server
	log.Printf("🚀 Server starting on port %s...", serverPort)
	log.Printf("📝 API endpoints:")
	log.Printf("  - POST   /api/v1/buckets/:bucketId/files")
	log.Printf("  - GET    /api/v1/buckets/:bucketId/files")
	log.Printf("  - DELETE /api/v1/buckets/:bucketId/files/:fileId?key=<filename>")
	log.Printf("  - GET    /health")

	if err := router.Run(":" + serverPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func getEnvInt(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
		fmt.Printf("⚠️  Invalid integer for %s: %s — using default %d\n", key, val, defaultVal)
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(val); err == nil {
			return time.Duration(intVal) * time.Second
		}
		if durVal, err := time.ParseDuration(val); err == nil {
			return durVal
		}
		fmt.Printf("⚠️  Invalid duration for %s: %s — using default\n", key, val)
	}
	return defaultVal
}

func monitorDBStats(db *sql.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := db.Stats()
		log.Printf("DB Pool Stats - Open: %d, InUse: %d, Idle: %d, WaitCount: %d",
			stats.OpenConnections, stats.InUse, stats.Idle, stats.WaitCount)
	}
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
