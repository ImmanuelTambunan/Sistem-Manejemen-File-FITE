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

	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/router"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	// Membaca backend/.env bila ada (pengembangan lokal). Di server, variabel datang dari lingkungan.
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("gagal memuat konfigurasi", "error", err)
		os.Exit(1)
	}

	db, err := gorm.Open(mysql.Open(cfg.DBDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		slog.Error("gagal membuka koneksi basis data", "error", err)
		os.Exit(1)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router.New(cfg, db),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		slog.Info("server berjalan", "addr", cfg.HTTPAddr, "env", cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server berhenti", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown tidak mulus", "error", err)
	}
	slog.Info("server dihentikan")
}
