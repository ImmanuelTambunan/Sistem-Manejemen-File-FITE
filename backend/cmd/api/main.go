package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fite-arsip-api/internal/app"
	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/handler"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/router"
	"fite-arsip-api/internal/service"
	"fite-arsip-api/internal/storage"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	if err := run(); err != nil {
		slog.Error("aplikasi berhenti", "error", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}

	httpx.Init()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := app.OpenDB(cfg)
	if err != nil {
		return err
	}
	sqlDB, _ := db.DB()

	var store storage.Storage
	if cfg.GCSBucket != "" {
		gcsStore, err := storage.NewGCS(ctx, cfg.GCSBucket, cfg.GCSCredentials)
		if err != nil {
			slog.Warn("gagal inisialisasi GCS, beralih ke memory storage", "error", err)
			store = storage.NewMemoryStorage()
		} else {
			store = gcsStore
		}
	} else {
		slog.Info("GCS_BUCKET belum diisi, menggunakan memory storage")
		store = storage.NewMemoryStorage()
	}

	authSvc := service.NewAuthService(db, cfg)
	docSvc := service.NewDocumentService(db, store)
	verSvc := service.NewVerificationService(db)

	h := router.Handlers{
		Auth:         handler.NewAuthHandler(authSvc, cfg, middleware.NewFailureLimiter(5, time.Minute)),
		Document:     handler.NewDocumentHandler(docSvc, cfg),
		Verification: handler.NewVerificationHandler(verSvc),
		Category:     handler.NewCategoryHandler(db),
		Notification: handler.NewNotificationHandler(db),
		User:         handler.NewUserHandler(db),
		Dashboard:    handler.NewDashboardHandler(db, cfg),
	}

	ready := func(c context.Context) error {
		if err := sqlDB.PingContext(c); err != nil {
			return err
		}
		return store.Ping(c)
	}

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router.New(cfg, authSvc, h, ready),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       2 * time.Minute, // unggahan 10 MB pada koneksi lambat
		WriteTimeout:      2 * time.Minute, // unduhan berkas
		IdleTimeout:       2 * time.Minute,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	slog.Info("API berjalan", "addr", cfg.HTTPAddr)

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdown)
	}
	return nil
}
