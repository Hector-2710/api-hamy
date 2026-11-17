package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"api-hamy/internal/config"
	"api-hamy/internal/repository/postgres"
	albumservice "api-hamy/internal/service/album"
	photoservice "api-hamy/internal/service/photo"
	userservice "api-hamy/internal/service/user"
	storageminio "api-hamy/internal/storage/minio"
	apihttp "api-hamy/internal/transport/http"
	"api-hamy/internal/transport/http/handler"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "err", err)
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
		cfg.MinioUseSSL,
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
	albumSvc := albumservice.New(albumRepo, userRepo, logger)
	photoSvc := photoservice.New(photoRepo, albumRepo, blobs, cfg.MinioBucket, cfg.UploadURLTTL, logger)

	userHandler := handler.NewUserHandler(userSvc)
	albumHandler := handler.NewAlbumHandler(albumSvc)
	photoHandler := handler.NewPhotoHandler(photoSvc)

	router := apihttp.NewRouter(userHandler, albumHandler, photoHandler)

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
