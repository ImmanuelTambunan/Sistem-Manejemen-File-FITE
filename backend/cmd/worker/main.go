package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fite-arsip-api/internal/app"
	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/service"
	"fite-arsip-api/internal/storage"

	"github.com/robfig/cron/v3"
	_ "time/tzdata" // zona waktu tertanam agar Asia/Jakarta tersedia di kontainer minimal
)

func main() {
	once := flag.Bool("once", false, "jalankan satu putaran retensi lalu keluar (untuk pengujian)")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("konfigurasi", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	db, err := app.OpenDB(cfg)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}

	var store storage.Storage
	switch cfg.StorageDriver {
	case "gcs":
		gcsStore, err := storage.NewGCS(ctx, cfg.GCSBucket, cfg.GCSCredentials)
		if err != nil {
			if cfg.IsProd() {
				slog.Error("fatal: inisialisasi Google Cloud Storage gagal di lingkungan production", "error", err)
				os.Exit(1)
			}
			slog.Warn("gagal inisialisasi GCS, beralih ke local storage untuk pengembangan lokal", "error", err)
			localStore, lErr := storage.NewLocalStorage(cfg.StorageLocalPath)
			if lErr != nil {
				slog.Error("fatal: inisialisasi fallback local storage gagal", "error", lErr)
				os.Exit(1)
			}
			store = localStore
		} else {
			store = gcsStore
		}
	case "memory":
		slog.Info("menggunakan in-memory storage")
		store = storage.NewMemoryStorage()
	case "local":
		fallthrough
	default:
		slog.Info("menggunakan local disk storage", "path", cfg.StorageLocalPath)
		localStore, err := storage.NewLocalStorage(cfg.StorageLocalPath)
		if err != nil {
			slog.Error("fatal: inisialisasi local storage gagal", "error", err)
			os.Exit(1)
		}
		store = localStore
	}

	svc := service.NewRetentionService(db, store, cfg)

	run := func() bool {
		rep, err := svc.Run(ctx)
		slog.Info("retensi selesai", "skipped", rep.Skipped, "soft", rep.SoftDeleted, "hard", rep.HardDeleted,
			"failed", rep.Failed, "anomalies", rep.Anomalies, "sessions_purged", rep.Sessions)
		if err != nil {
			slog.Error("retensi galat", "error", err)
		}
		return err == nil && rep.Failed == 0
	}

	if *once {
		if !run() {
			os.Exit(1)
		}
		return
	}

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		slog.Error("zona waktu", "error", err)
		os.Exit(1)
	}

	c := cron.New(cron.WithLocation(loc),
		cron.WithChain(cron.Recover(cron.DefaultLogger), cron.SkipIfStillRunning(cron.DefaultLogger)))

	if _, err := c.AddFunc("0 2 * * *", func() { run() }); err != nil { // 02:00 WIB setiap hari
		slog.Error("jadwal", "error", err)
		os.Exit(1)
	}

	c.Start()
	slog.Info("worker retensi aktif", "jadwal", "02:00 "+cfg.Timezone)
	<-ctx.Done()
	<-c.Stop().Done()
}
