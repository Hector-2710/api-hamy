package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// DefaultJWTSecret is used only outside production when JWT_SECRET is unset.
// main logs a warning whenever this value is active.
const DefaultJWTSecret = "dev-only-insecure-secret-change-me-please"

type Config struct {
	AppEnv     string
	HTTPPort   string
	LogLevel   string
	DBDSN      string
	DBMaxConns int32

	CORSAllowedOrigins []string

	JWTSecret string
	JWTIssuer string
	JWTTTL    time.Duration

	MinioEndpoint       string
	MinioPublicEndpoint string
	MinioAccessKey      string
	MinioSecretKey      string
	MinioBucket         string
	MinioRegion         string
	MinioUseSSL         bool
	MinioPublicUseSSL   bool
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

	publicUseSSL, err := strconv.ParseBool(getenv("MINIO_PUBLIC_USE_SSL", getenv("MINIO_USE_SSL", "false")))
	if err != nil {
		return nil, fmt.Errorf("MINIO_PUBLIC_USE_SSL is not a boolean: %w", err)
	}

	ttl, err := time.ParseDuration(getenv("UPLOAD_URL_TTL", "15m"))
	if err != nil {
		return nil, fmt.Errorf("UPLOAD_URL_TTL is not a valid duration (e.g. 15m): %w", err)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("UPLOAD_URL_TTL must be greater than 0")
	}

	appEnv := getenv("APP_ENV", "development")

	jwtSecret := getenv("JWT_SECRET", "")
	if jwtSecret == "" {
		if appEnv == "production" {
			return nil, fmt.Errorf("JWT_SECRET is required when APP_ENV=production")
		}
		jwtSecret = DefaultJWTSecret
	}
	if len(jwtSecret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 characters")
	}

	jwtTTL, err := time.ParseDuration(getenv("JWT_TTL", "24h"))
	if err != nil {
		return nil, fmt.Errorf("JWT_TTL is not a valid duration (e.g. 24h): %w", err)
	}
	if jwtTTL <= 0 {
		return nil, fmt.Errorf("JWT_TTL must be greater than 0")
	}

	cfg := &Config{
		AppEnv:     appEnv,
		HTTPPort:   getenv("HTTP_PORT", "8080"),
		LogLevel:   getenv("LOG_LEVEL", "info"),
		DBDSN:      getenv("DB_DSN", "postgres://hamy:hamy@localhost:5432/hamy?sslmode=disable"),
		DBMaxConns: int32(maxConns),

		CORSAllowedOrigins: parseList(getenv("CORS_ALLOWED_ORIGINS", "*")),

		JWTSecret: jwtSecret,
		JWTIssuer: getenv("JWT_ISSUER", "api-hamy"),
		JWTTTL:    jwtTTL,

		MinioEndpoint:       getenv("MINIO_ENDPOINT", "localhost:9000"),
		MinioPublicEndpoint: getenv("MINIO_PUBLIC_ENDPOINT", ""),
		MinioAccessKey:      getenv("MINIO_ACCESS_KEY", ""),
		MinioSecretKey:      getenv("MINIO_SECRET_KEY", ""),
		MinioBucket:         getenv("MINIO_BUCKET", "hamy-photos"),
		MinioRegion:         getenv("MINIO_REGION", "us-east-1"),
		MinioUseSSL:         useSSL,
		MinioPublicUseSSL:   publicUseSSL,
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
	case len(c.JWTSecret) < 32:
		return fmt.Errorf("JWT_SECRET must be at least 32 characters")
	case c.AppEnv == "production" && c.JWTSecret == DefaultJWTSecret:
		return fmt.Errorf("JWT_SECRET must be changed when APP_ENV=production")
	case c.JWTIssuer == "":
		return fmt.Errorf("JWT_ISSUER is required")
	case c.JWTTTL <= 0:
		return fmt.Errorf("JWT_TTL must be greater than 0")
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

// parseList splits a comma-separated env var, dropping blank entries.
func parseList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
