package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"s3/internal/application"

	"s3/internal/infrastructure/config"
	"s3/database"
	"s3/internal/infrastructure/event"
	"s3/internal/infrastructure/logging"
	"s3/internal/infrastructure/metrics"
	"s3/internal/infrastructure/network"
	"s3/internal/infrastructure/repository"
	"s3/internal/infrastructure/storage"
	"s3/internal/infrastructure/system"
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

	slog.Info("Successfully registered with Eureka server", slog.String("url", url))
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
			slog.Error("Failed to create heartbeat request", slog.Any("error", err))
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			slog.Error("Failed to send heartbeat to Eureka", slog.Any("error", err))
			continue
		}

		if resp.StatusCode != httpd.StatusOK && resp.StatusCode != httpd.StatusNoContent {
			body, _ := io.ReadAll(resp.Body)
			slog.Warn("Heartbeat failed", slog.Int("status", resp.StatusCode), slog.String("body", string(body)))
		} else {
			slog.Debug("Heartbeat sent successfully to Eureka")
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

	slog.Info("Successfully deregistered from Eureka server")
	return nil
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	// Initialize Logger
	logging.InitLogger(cfg.APP_PROFILE)
	slog.Info("Application starting", slog.String("profile", cfg.APP_PROFILE))

	// Register with retries
	for i := 0; i < 3; i++ {
		if err := registerWithEureka(cfg.Eureka); err != nil {
			slog.Warn("Eureka registration attempt failed", slog.Int("attempt", i+1), slog.Any("error", err))
			time.Sleep(5 * time.Second)
		} else {
			break
		}
	}

	// Start heartbeat
	go sendHeartbeat(cfg.Eureka)

	// Pre-requisite reachability checks
	slog.Info("Performing reachability checks...")
	
	if err := network.CheckReachability(cfg.NATS.Host, cfg.NATS.Port, 5, 2*time.Second); err != nil {
		slog.Error("FATAL: NATS unreachable", slog.Any("error", err), slog.String("host", cfg.NATS.Host), slog.Int("port", cfg.NATS.Port))
		os.Exit(1)
	}

	if err := network.CheckReachability(cfg.Database.Host, cfg.Database.Port, 5, 2*time.Second); err != nil {
		slog.Error("FATAL: Database unreachable", slog.Any("error", err), slog.String("host", cfg.Database.Host), slog.Int("port", cfg.Database.Port))
		os.Exit(1)
	}

	if err := network.CheckReachability(cfg.S3.Host, cfg.S3.Port, 5, 2*time.Second); err != nil {
		slog.Error("FATAL: MinIO unreachable", slog.Any("error", err), slog.String("host", cfg.S3.Host), slog.Int("port", cfg.S3.Port))
		os.Exit(1)
	}

	slog.Info("Initializing MinIO adapter...")
	minioAdapter, err := storage.NewMinIOAdapter(
		cfg.S3.Endpoint,
		cfg.S3.AccessKey,
		cfg.S3.SecretKey,
		cfg.S3.UseSSL,
	)
	if err != nil {
		slog.Error("Failed to create MinIO adapter", slog.Any("error", err))
		os.Exit(1)
	}

	// Initialize NATS connection FIRST for IAM integration
	slog.Info("Connecting to NATS...", slog.String("url", cfg.NATS.URL))
	natsAdapter, err := event.NewNATSAdapter(cfg.NATS.URL, cfg.NATS.User, cfg.NATS.Password, cfg.APP_PROFILE)
	if err != nil {
		slog.Error("Failed to connect to NATS", slog.Any("error", err))
		os.Exit(1)
	}
	defer natsAdapter.Close()

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

	fmt.Printf("Connecting to postgres database", dbConfig.Database)
	db, err := database.NewPostgresDB(dbConfig)
	if err != nil {
		slog.Error("Failed to connect to database", slog.Any("error", err))
		os.Exit(1)
	}

	go monitorDBStats(db)
	postgresRepo := repository.NewPostgresRepository(db)

	// Create IAM validator

	// Run migrations
	slog.Info("Running database migrations---...",slog.String("url", dbConfig.Host))
	version, dirty, err := database.GetMigrationVersion(db, dbConfig.Database)
	if err == nil && dirty {
		slog.Warn("Database is dirty, forcing version", slog.Uint64("version", uint64(version)))
		if err := database.ForceVersion(db, dbConfig.Database, int(version)); err != nil {
			slog.Error("Failed to force migration version", slog.Any("error", err))
			os.Exit(1)
		}
	}

	if err := database.RunMigrations(db, dbConfig.Database); err != nil {
		slog.Error("Failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Migrations completed successfully")

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
	slog.Info("Initializing services...")
	presignedService := application.NewPresignService(postgresRepo, postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient, cfg.S3.SecretKey)
	uploadService := application.NewUploadService(minioAdapter, postgresRepo, postgresRepo, metricsClient, natsAdapter, presignedService)
	bucketService := application.NewBucketService(postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient)
	deleteService := application.NewDeleteService(minioAdapter, postgresRepo, postgresRepo, metricsClient)
	healthService := application.NewHealthService(postgresRepo, minioAdapter, sys)
	batchService := application.NewBatchService(postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient)
	prefixService := application.NewPrefixService(postgresRepo, postgresRepo, minioAdapter, metricsClient)
	SearchService := application.NewSearchService(postgresRepo, postgresRepo)
	webhookService := application.NewWebhookService(postgresRepo, postgresRepo)
	analyticsService := application.NewAnalyticsService(postgresRepo, postgresRepo, postgresRepo, postgresRepo)
	multipartService := application.NewMultipartService(postgresRepo, postgresRepo, postgresRepo, minioAdapter, metricsClient)
	accessPointService := application.NewAccessPointService(postgresRepo, postgresRepo, natsAdapter)
	securityService := application.NewSecurityService(postgresRepo)
	docsService := application.NewDocsService("./docs")

	// 3. Initialize Transport Layer (HTTP)
	slog.Info("Initializing HTTP handlers...")
	handlers := &http.Handlers{
		File:        http.NewFileHandler(uploadService, deleteService,cfg.S3.SecretKey),
		Bucket:      http.NewBucketHandler(bucketService),
		Health:      http.NewHealthHandler(healthService),
		Presign:     http.NewPresignHandler(presignedService, uploadService),
		Batch:       http.NewBatchHandler(batchService),         
		Prefix:      http.NewPrefixHandler(prefixService),       
		Search:      http.NewSearchHandler(SearchService),       
		Webhook:     http.NewWebhookHandler(webhookService),     
		Analytics:   http.NewAnalyticsHandler(analyticsService), 
		Multipart:   http.NewMultipartHandler(multipartService), 
		AccessPoint: http.NewAccessPointHandler(accessPointService),
		Security:    http.NewSecurityHandler(securityService),
		JWTValidator: nil,        
		Docs:        http.NewDocsHandler(docsService),  
	}
	
	// Initialize and start NATS controllers
	slog.Info("Initializing NATS controllers...")
	presignController := nats.NewPresignController(natsAdapter.GetConnection(), presignedService, bucketService, postgresRepo, cfg.NATS.NatsPrefix, cfg.S3.DefaultBuckets, cfg.S3.SecretKey, postgresRepo)
	if err := presignController.Start(); err != nil {
		slog.Warn("Failed to start NATS presign controller", slog.Any("error", err))
	}

	// 4. Setup Router
	router := gin.Default()
	http.RegisterRoutes(router, handlers)

	// 5. Start Server
	slog.Info("Server starting", slog.String("port", cfg.ServerPort))
	slog.Info("Routes initialized", 
		slog.String("upload", "/api/v1/buckets/:bucketId/files"),
		slog.String("health", "/health"))

	if err := router.Run(":" + cfg.ServerPort); err != nil {
		slog.Error("Failed to start server", slog.Any("error", err))
		os.Exit(1)
	}

}

func monitorDBStats(db *sql.DB) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := db.Stats()
		slog.Info("DB Pool Stats", 
			slog.Int("open", stats.OpenConnections), 
			slog.Int("in_use", stats.InUse), 
			slog.Int("idle", stats.Idle), 
			slog.Int("wait_count", int(stats.WaitCount)))
	}
}
