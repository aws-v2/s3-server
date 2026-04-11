package config

import (
	"fmt"
	"os"
	"s3/internal/utils"
	"strconv"
	"time"
)

type Config struct {
	APP_PROFILE string
	ServerPort  string
	Eureka      EurekaConfig
	Database    DatabaseConfig
	NATS        NATSConfig
	S3          S3Config
}

type EurekaConfig struct {
	ServerURL         string
	AppName           string
	HostName          string
	IPAddr            string
	Port              int
	VipAddress        string
	InstanceID        string
	HeartbeatInterval time.Duration
}

type DatabaseConfig struct {
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

type NATSConfig struct {
	URL      string
	User     string
	Password string
}

type S3Config struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

func Load() (*Config, error) {
	utilsCfg, err := utils.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load base config: %w", err)
	}

	appProfile := getEnv("APP_PROFILE", "dev")
	serverPort := getEnv("SERVER_PORT", "8082")

	return &Config{
		APP_PROFILE: appProfile,
		ServerPort:  serverPort,
		Eureka: EurekaConfig{
			ServerURL:         getEnv("EUREKA_SERVER_URL", ""),
			AppName:           getEnv("EUREKA_APP_NAME", "S3-SERVICE"),
			HostName:          getEnv("EUREKA_HOSTNAME", "localhost"),
			IPAddr:            getEnv("EUREKA_IP_ADDR", "127.0.0.1"),
			Port:              getEnvInt("SERVER_PORT", 8082),
			VipAddress:        getEnv("EUREKA_VIP_ADDRESS", "s3-service"),
			InstanceID:        getEnv("EUREKA_INSTANCE_ID", "s3-service:8082"),
			HeartbeatInterval: getEnvDuration("EUREKA_HEARTBEAT_INTERVAL", 30*time.Second),
		},
		Database: DatabaseConfig{
			Host:            utilsCfg.DB.Host,
			Port:            utilsCfg.DB.Port,
			User:            utilsCfg.DB.User,
			Password:        utilsCfg.DB.Password,
			Database:        utilsCfg.DB.Database,
			SSLMode:         utilsCfg.DB.SSLMode,
			MaxOpenConns:    utilsCfg.DB.MaxOpenConns,
			MaxIdleConns:    utilsCfg.DB.MaxIdleConns,
			ConnMaxLifetime: utilsCfg.DB.ConnMaxLifetime,
			ConnMaxIdleTime: utilsCfg.DB.ConnMaxIdleTime,
		},
		NATS: NATSConfig{
			URL:      utilsCfg.NATS.URL,
			User:     utilsCfg.NATS.User,
			Password: utilsCfg.NATS.Password,
		},
		S3: S3Config{
			Endpoint:  utilsCfg.S3.Endpoint,
			AccessKey: utilsCfg.S3.AccessKey,
			SecretKey: utilsCfg.S3.SecretKey,
			UseSSL:    utilsCfg.S3.UseSSL,
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultVal int) int {
	if val, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(val); err == nil {
			return intVal
		}
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
	}
	return defaultVal
}
