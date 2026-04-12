package utils

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// Database
	DB DBConfig

	// S3/MinIO
	S3 S3Config

	// Server
	Server ServerConfig

	// NATS
	NATS NATSConfig
}

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Database        string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
	Host      string
	Port      int
}

type ServerConfig struct {
	Port string
}

type NATSConfig struct {
	URL      string
	User     string
	Password string
	Host     string
	Port     int
}

func Load() (*Config, error) {
	// Load .env file
	_ = godotenv.Load()

	natsURL := getEnv("NATS_URL", "nats://localhost:4222")
	natsHost, natsPort := parseAddr(natsURL, "localhost", 4222)

	minioEndpoint := getEnv("MINIO_ENDPOINT", "localhost:9000")
	minioHost, minioPort := parseAddr(minioEndpoint, "localhost", 9000)

	cfg := &Config{
		DB: DBConfig{
			Host:            getEnv("POSTGRES_HOST", "localhost"),
			Port:            getEnvInt("POSTGRES_PORT", 5432),
			User:            getEnv("POSTGRES_USER", "root"),
			Password:        getEnv("POSTGRES_PASSWORD", "root"),
			Database:        getEnv("POSTGRES_DB", "s3"),
			SSLMode:         getEnv("POSTGRES_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 25),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnMaxLifetime: getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute),
			ConnMaxIdleTime: getEnvDuration("DB_CONN_MAX_IDLE_TIME", 10*time.Minute),
		},
		S3: S3Config{
			Endpoint:  minioEndpoint,
			AccessKey: getEnv("MINIO_ACCESS_KEY", "minioadmin"),
			SecretKey: getEnv("MINIO_SECRET_KEY", "minioadmin123"),
			UseSSL:    getEnvBool("MINIO_USE_SSL", false),
			Host:      getEnv("MINIO_HOST", minioHost),
			Port:      getEnvInt("MINIO_PORT", minioPort),
		},
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8083"),
		},
		NATS: NATSConfig{
			URL:      natsURL,
			User:     getEnv("NATS_USER", "auth-server"),
			Password: getEnv("NATS_PASSWORD", "auth-secret"),
			Host:     getEnv("NATS_HOST", natsHost),
			Port:     getEnvInt("NATS_PORT", natsPort),
		},
	}

	if cfg.DB.Password == "" {
		return nil, fmt.Errorf("POSTGRES_PASSWORD is required")
	}

	return cfg, nil
}

func parseAddr(addr string, defaultHost string, defaultPort int) (string, int) {
	if addr == "" {
		return defaultHost, defaultPort
	}

	// Try parsing as URL first (for nats://...)
	u, err := url.Parse(addr)
	if err == nil && u.Host != "" {
		if host, portStr, err := net.SplitHostPort(u.Host); err == nil {
			port, _ := strconv.Atoi(portStr)
			return host, port
		}
		// If SplitHostPort fails, u.Host might just be the hostname
		return u.Host, defaultPort
	}

	// Try parsing as host:port (for MINIO_ENDPOINT)
	if host, portStr, err := net.SplitHostPort(addr); err == nil {
		port, _ := strconv.Atoi(portStr)
		return host, port
	}

	// If all fails, return addr as host and default port
	return addr, defaultPort
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
