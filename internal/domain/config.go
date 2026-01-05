package domain

// Config holds all application configuration
type Config struct {
	MinIO MinIOConfig
	Postgres PostgresConfig
	Server ServerConfig
	Environment string
}

type MinIOConfig struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	UseSSL    bool
}

type PostgresConfig struct {
	ConnectionString string
	MaxConnections   int
	MaxIdleConns     int
}

type ServerConfig struct {
	Port string
	Host string
}
