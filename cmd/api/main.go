package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"api-hamy/internal/config"
	"api-hamy/internal/repository/postgres"
	adminservice "api-hamy/internal/service/admin"
	albumservice "api-hamy/internal/service/album"
	authservice "api-hamy/internal/service/auth"
	photoservice "api-hamy/internal/service/photo"
	userservice "api-hamy/internal/service/user"
	storageminio "api-hamy/internal/storage/minio"
	"api-hamy/internal/token"
	apihttp "api-hamy/internal/transport/http"
	"api-hamy/internal/transport/http/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	logger := newLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	if cfg.JWTSecret == config.DefaultJWTSecret {
		logger.Warn("JWT_SECRET is using the insecure development default")
	}

	tokens, err := token.NewManager(cfg.JWTSecret, cfg.JWTIssuer, cfg.JWTTTL)
	if err != nil {
		logger.Error("invalid JWT configuration", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.NewPool(ctx, cfg.DBDSN, cfg.DBMaxConns)
	if err != nil {
		logger.Error("could not connect to postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	blobs, err := storageminio.New(
		cfg.MinioEndpoint,
		cfg.MinioPublicEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioRegion,
		cfg.MinioBucket,
		cfg.MinioUseSSL,
		cfg.MinioPublicUseSSL,
	)
	if err != nil {
		logger.Error("invalid MinIO configuration", "err", err)
		os.Exit(1)
	}
	if err := waitForBucket(ctx, blobs, 30*time.Second, logger); err != nil {
		logger.Error("could not prepare the MinIO bucket", "err", err)
		os.Exit(1)
	}

	userRepo := postgres.NewUserRepository(pool)
	albumRepo := postgres.NewAlbumRepository(pool)
	photoRepo := postgres.NewPhotoRepository(pool)

	userSvc := userservice.New(userRepo, logger)
	authSvc := authservice.New(userSvc, tokens, logger)
	adminSvc := adminservice.New(userSvc, logger)
	albumSvc := albumservice.New(albumRepo, userRepo, logger)
	photoSvc := photoservice.New(photoRepo, albumRepo, blobs, cfg.MinioBucket, cfg.UploadURLTTL, logger)

	healthHandler := handler.NewHealthHandler(pool, blobs)
	authHandler := handler.NewAuthHandler(authSvc)
	albumHandler := handler.NewAlbumHandler(albumSvc)
	photoHandler := handler.NewPhotoHandler(photoSvc)
	adminHandler := handler.NewAdminHandler(adminSvc)

	router := apihttp.NewRouter(
		healthHandler,
		authHandler,
		authSvc,
		albumHandler,
		photoHandler,
		adminHandler,
		cfg.CORSAllowedOrigins,
	)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server listening", "port", cfg.HTTPPort, "env", cfg.AppEnv, "bucket", cfg.MinioBucket)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped with error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down server")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("forced shutdown", "err", err)
	}
}

// newLogger builds the JSON logger with the configured level.
func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl}))
}

// waitForBucket retries EnsureBucket until MinIO responds or the timeout expires.
func waitForBucket(ctx context.Context, store *storageminio.Client, timeout time.Duration, log *slog.Logger) error {
	deadline := time.Now().Add(timeout)
	for {
		err := store.EnsureBucket(ctx)
		if err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}

		log.Warn("MinIO is not ready; retrying", "err", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}
