package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	AppEnv     string
	HTTPPort   string
	LogLevel   string
	DBDSN      string
	DBMaxConns int32

	MinioEndpoint       string
	MinioPublicEndpoint string
	MinioAccessKey      string
	MinioSecretKey      string
	MinioBucket         string
	MinioRegion         string
	MinioUseSSL         bool
	UploadURLTTL        time.Duration
}

func Load() (*Config, error) {
	maxConns, err := strconv.Atoi(getenv("DB_MAX_CONNS", "10"))
	if err != nil {
		return nil, fmt.Errorf("DB_MAX_CONNS is not a number: %w", err)
	}
	if maxConns < 1 {
		return nil, fmt.Errorf("DB_MAX_CONNS must be greater than 0")
	}

	useSSL, err := strconv.ParseBool(getenv("MINIO_USE_SSL", "false"))
	if err != nil {
		return nil, fmt.Errorf("MINIO_USE_SSL is not a boolean: %w", err)
	}

	ttl, err := time.ParseDuration(getenv("UPLOAD_URL_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("UPLOAD_URL_TTL is not a valid duration (e.g. 15m): %w", err)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("UPLOAD_URL_TTL must be greater than 0")
	}

	cfg := &Config{
		AppEnv:     getenv("APP_ENV", "development"),
		HTTPPort:   getenv("HTTP_PORT", "8080"),
		LogLevel:   getenv("LOG_LEVEL", "info"),
		DBDSN:      getenv("DB_DSN", "postgres://hamy:hamy@localhost:5432/hamy?sslmode=disable"),
		DBMaxConns: int32(maxConns),

		MinioEndpoint:       getenv("MINIO_ENDPOINT", "localhost:9000"),
		MinioPublicEndpoint: getenv("MINIO_PUBLIC_ENDPOINT", ""),
		MinioAccessKey:      getenv("MINIO_ACCESS_KEY", ""),
		MinioSecretKey:      getenv("MINIO_SECRET_KEY", ""),
		MinioBucket:         getenv("MINIO_BUCKET", "hamy-photos"),
		MinioRegion:         getenv("MINIO_REGION", "us-east-1"),
		MinioUseSSL:         useSSL,
		UploadURLTTL:        ttl,
	}

	if cfg.MinioPublicEndpoint == "" {
		cfg.MinioPublicEndpoint = cfg.MinioEndpoint
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	switch {
	case c.DBDSN == "":
		return fmt.Errorf("DB_DSN is required")
	case c.MinioEndpoint == "":
		return fmt.Errorf("MINIO_ENDPOINT is required")
	case c.MinioAccessKey == "":
		return fmt.Errorf("MINIO_ACCESS_KEY is required")
	case c.MinioSecretKey == "":
		return fmt.Errorf("MINIO_SECRET_KEY is required")
	case c.MinioBucket == "":
		return fmt.Errorf("MINIO_BUCKET is required")
	}
	return nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
