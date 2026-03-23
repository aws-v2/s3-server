package main

import (
	"database/sql"
	"fmt"
	"log"
	"s3/internal/application"

	"s3/internal/infrastructure/config"
	"s3/internal/infrastructure/database"
	"s3/internal/infrastructure/event"
	"s3/internal/infrastructure/metrics"
	"s3/internal/infrastructure/repository"
	"s3/internal/infrastructure/storage"
	"s3/internal/infrastructure/system"
	"s3/internal/middleware"
	"s3/internal/transport/http"
	"s3/internal/transport/nats"
	"time"

	"github.com/gin-gonic/gin"

	"bytes"
	"encoding/json"
	"io"
	httpd "net/http"
)

// registerWithEureka registers the service instance with Eureka server
func registerWithEureka(config config.EurekaConfig) error {
	instance := map[string]interface{}{
		"instance": map[string]interface{}{
			"instanceId": config.InstanceID,
			"hostName":   config.HostName,
			"app":        config.AppName,
			"ipAddr":     config.IPAddr,
			"vipAddress": config.VipAddress,
			"status":     "UP",
			"port": map[string]interface{}{
				"$":        config.Port,
				"@enabled": "true",
			},
			"dataCenterInfo": map[string]interface{}{
				"@class": "com.netflix.appinfo.InstanceInfo$DefaultDataCenterInfo",
				"name":   "MyOwn",
			},
			"healthCheckUrl": fmt.Sprintf("http://%s:%d/health", config.HostName, config.Port),
			"statusPageUrl":  fmt.Sprintf("http://%s:%d/health", config.HostName, config.Port),
			"homePageUrl":    fmt.Sprintf("http://%s:%d/", config.HostName, config.Port),
		},
	}

	jsonData, err := json.Marshal(instance)
	if err != nil {
		return fmt.Errorf("failed to marshal Eureka registration data: %w", err)
	}

	url := fmt.Sprintf("%s/apps/%s", config.ServerURL, config.AppName)
	req, err := httpd.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &httpd.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register with Eureka: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != httpd.StatusNoContent && resp.StatusCode != httpd.StatusOK {
		return fmt.Errorf("eureka registration failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Successfully registered with Eureka server at %s", url)
	return nil
}

// sendHeartbeat sends periodic heartbeats to Eureka server
func sendHeartbeat(config config.EurekaConfig) {
	ticker := time.NewTicker(config.HeartbeatInterval)
	defer ticker.Stop()

	url := fmt.Sprintf("%s/apps/%s/%s", config.ServerURL, config.AppName, config.InstanceID)
	client := &httpd.Client{Timeout: 5 * time.Second}

	for range ticker.C {
		req, err := httpd.NewRequest("PUT", url, nil)
		if err != nil {
			log.Printf("❌ Failed to create heartbeat request: %v", err)
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("❌ Failed to send heartbeat to Eureka: %v", err)
			continue
		}

		if resp.StatusCode != httpd.StatusOK && resp.StatusCode != httpd.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			log.Printf("⚠️  Heartbeat failed with status %d: %s", resp.StatusCode, string(body))
		} else {
			log.Printf("💓 Heartbeat sent successfully to Eureka")
		}

		resp.Body.Close()
	}
}

// deregisterFromEureka removes the service instance from Eureka (optional, for graceful shutdown)
func deregisterFromEureka(config config.EurekaConfig) error {
	url := fmt.Sprintf("%s/apps/%s/%s", config.ServerURL, config.AppName, config.InstanceID)
	req, err := httpd.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create deregistration request: %w", err)
	}

	client := &httpd.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to deregister from Eureka: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != httpd.StatusOK && resp.StatusCode != httpd.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deregistration failed with status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("✅ Successfully deregistered from Eureka server")
	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Register with retries
	for i := 0; i < 3; i++ {
		if err := registerWithEureka(cfg.Eureka); err != nil {
			log.Printf("⚠️  Eureka registration attempt %d failed: %v", i+1, err)
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	// Start heartbeat
	go sendHeartbeat(cfg.Eureka)

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

	dbConfig := database.Config{
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		User:            cfg.Database.User,
		Password:        cfg.Database.Password,
		Database:        cfg.Database.Database,
		SSLMode:         cfg.Database.SSLMode,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	}

	log.Println("Connecting to PostgreSQL...")
	db, err := database.NewPostgresDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	go monitorDBStats(db)
	postgresRepo := repository.NewPostgresRepository(db)

	// Initialize NATS connection for IAM integration
	log.Printf("Connecting to NATS at %s...", cfg.NATS.URL)
	natsAdapter, err := event.NewNATSAdapter(cfg.NATS.URL, cfg.NATS.User, cfg.NATS.Password)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}
	defer natsAdapter.Close()

	// Create IAM validator
	iamValidator := middleware.NewIAMValidator(natsAdapter.GetConnection())

	// Run migrations
	log.Println("Running database migrations...")
	version, dirty, err := database.GetMigrationVersion(db, dbConfig.Database)
	if err == nil && dirty {
		log.Printf("⚠️  Database is dirty at version %d. Forcing version to clear dirty flag...", version)
		if err := database.ForceVersion(db, dbConfig.Database, int(version)); err != nil {
			log.Fatalf("Failed to force migration version: %v", err)
		}
	}

	if err := database.RunMigrations(db, dbConfig.Database); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}
	log.Println("Migrations completed successfully")

	// // Check current migration version (optional)
	// version, dirty, err := database.GetMigrationVersion(db, dbConfig.Database)
	// if err != nil {
	// 	log.Printf("Warning: Failed to get migration version: %v", err)
	// } else {
	// 	log.Printf("Current migration version: %d (dirty: %t)", version, dirty)
	// }

	sys := &system.System{} // note pointer, so methods can be called

	// Initialize Metrics Client
	metricsClient := metrics.NewMetricsClient(natsAdapter, cfg.Eureka.InstanceID)

	// 2. Initialize Application Layer (Services)
	log.Println("Initializing services...")
	uploadService := application.NewUploadService(minioAdapter, postgresRepo, postgresRepo, metricsClient, natsAdapter)
	bucketService := application.NewBucketService(postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient)
	deleteService := application.NewDeleteService(minioAdapter, postgresRepo, postgresRepo, metricsClient)
	healthService := application.NewHealthService(postgresRepo, minioAdapter, sys)
	presignedService := application.NewPresignService(postgresRepo, postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient, cfg.S3.SecretKey)
	batchService := application.NewBatchService(postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient)
	prefixService := application.NewPrefixService(postgresRepo, postgresRepo, minioAdapter, metricsClient)
	SearchService := application.NewSearchService(postgresRepo, postgresRepo)
	webhookService := application.NewWebhookService(postgresRepo, postgresRepo)
	analyticsService := application.NewAnalyticsService(postgresRepo, postgresRepo, postgresRepo, postgresRepo)
	multipartService := application.NewMultipartService(postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient)
	accessPointService := application.NewAccessPointService(postgresRepo, postgresRepo, natsAdapter)
	securityService := application.NewSecurityService(postgresRepo)

	// 3. Initialize Transport Layer (HTTP)
	log.Println("Initializing HTTP handlers...")
	handlers := &http.Handlers{
		File:        http.NewFileHandler(uploadService, deleteService),
		Bucket:      http.NewBucketHandler(bucketService),
		Health:      http.NewHealthHandler(healthService),
		Presign:     http.NewPresignHandler(presignedService, uploadService),
		Batch:       http.NewBatchHandler(batchService),         // TODO: implement later
		Prefix:      http.NewPrefixHandler(prefixService),       // TODO: implement later
		Search:      http.NewSearchHandler(SearchService),       // TODO: implement later
		Webhook:     http.NewWebhookHandler(webhookService),     // TODO: implement later
		Analytics:   http.NewAnalyticsHandler(analyticsService), // TODO: implement later
		Multipart:   http.NewMultipartHandler(multipartService), // TODO: implement later
		AccessPoint: http.NewAccessPointHandler(accessPointService),
		Security:    http.NewSecurityHandler(securityService),
		// Auth:         http.NewAuthHandler(authService),           // JWT authentication handler
		Validator:    iamValidator, // IAM/API Key validator
		JWTValidator: nil,          // JWT validator removed
	}
	
	// Initialize and start NATS controllers
	log.Println("Initializing NATS controllers...")
	presignController := nats.NewPresignController(natsAdapter.GetConnection(), presignedService, bucketService)
	if err := presignController.Start(); err != nil {
		log.Printf("Warning: failed to start NATS presign controller: %v", err)
	}

	// 4. Setup Router
	router := gin.Default()
	http.RegisterRoutes(router, handlers)

	// 5. Start Server
	log.Printf("🚀 Server starting on port %s...", cfg.ServerPort)
	log.Printf("  - POST   /api/v1/buckets/:bucketId/files")
	log.Printf("  - GET    /api/v1/buckets/:bucketId/files")
	log.Printf("  - DELETE /api/v1/buckets/:bucketId/files/:fileId?key=<filename>")
	log.Printf("  - GET    /health")

	if err := router.Run(":" + cfg.ServerPort); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

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
