package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/storage"

	"gorm.io/gorm"
)

const (
	retentionLock  = "fite_retention_cleanup"
	retentionChunk = 200
)

type RetentionReport struct {
	Skipped     bool  // instans lain sedang berjalan
	SoftDeleted int
	HardDeleted int
	Failed      int
	Anomalies   int   // berkas tanpa archived_at
	Sessions    int64
}

type RetentionService struct {
	db    *gorm.DB
	store storage.Storage
	cfg   config.Config
	Now   func() time.Time // dapat diganti pada pengujian
}

func NewRetentionService(db *gorm.DB, store storage.Storage, cfg config.Config) *RetentionService {
	return &RetentionService{db: db, store: store, cfg: cfg, Now: time.Now}
}

func (s *RetentionService) Run(ctx context.Context) (RetentionReport, error) {
	var rep RetentionReport
	sqlDB, err := s.db.DB()
	if err != nil {
		return rep, err
	}

	// Kunci GET_LOCK terikat pada satu koneksi; koneksi dipegang selama proses berjalan.
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return rep, err
	}
	defer conn.Close()

	// GET_LOCK is MySQL specific; in SQLite or fallback mock, if not supported, we proceed safely
	var got sql.NullInt64
	queryErr := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", retentionLock).Scan(&got)
	if queryErr == nil {
		if !got.Valid || got.Int64 != 1 {
			rep.Skipped = true
			return rep, nil
		}
		defer func() {
			_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK(?)", retentionLock)
		}()
	}

	now := s.Now().UTC()
	cutoff := now.AddDate(-s.cfg.RetentionYears, 0, 0) // lebih dari 8 tahun menurut kalender
	graceCutoff := now.AddDate(0, 0, -s.cfg.GraceDays)  // masa tenggang 90 hari

	if err := s.softStage(ctx, cutoff, &rep); err != nil {
		return rep, err
	}
	if err := s.hardStage(ctx, graceCutoff, &rep); err != nil {
		return rep, err
	}

	var n int64
	if err := s.db.WithContext(ctx).Model(&domain.Document{}).Where("archived_at IS NULL").Count(&n).Error; err == nil {
		rep.Anomalies = int(n)
		if n > 0 {
			slog.Warn("retensi: berkas tanpa archived_at dilewati", "jumlah", n)
		}
	}

	old := now.AddDate(0, 0, -30)
	res := s.db.WithContext(ctx).
		Where("expires_at < ? OR absolute_expires_at < ? OR revoked_at < ?", old, old, old).
		Delete(&domain.AuthSession{})
	rep.Sessions = res.RowsAffected

	return rep, res.Error
}

func (s *RetentionService) softStage(ctx context.Context, cutoff time.Time, rep *RetentionReport) error {
	var last uint
	for {
		var docs []domain.Document
		err := s.db.WithContext(ctx).
			Where("id > ? AND archived_at IS NOT NULL AND archived_at < ?", last, cutoff).
			Order("id").Limit(retentionChunk).Find(&docs).Error
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return nil
		}
		for i := range docs {
			last = docs[i].ID
			if err := s.expireSoft(ctx, &docs[i]); err != nil {
				rep.Failed++
				slog.Error("retensi soft delete gagal", "document_id", docs[i].ID, "error", err)
				continue
			}
			rep.SoftDeleted++
		}
	}
}

func (s *RetentionService) hardStage(ctx context.Context, graceCutoff time.Time, rep *RetentionReport) error {
	var last uint
	for {
		var docs []domain.Document
		err := s.db.WithContext(ctx).Unscoped().
			Where("id > ? AND deleted_at IS NOT NULL AND deleted_at <= ?", last, graceCutoff).
			Order("id").Limit(retentionChunk).Find(&docs).Error
		if err != nil {
			return err
		}
		if len(docs) == 0 {
			return nil
		}
		for i := range docs {
			last = docs[i].ID
			if err := s.expireHard(ctx, &docs[i]); err != nil {
				rep.Failed++ // GCS gagal -> rollback, dicoba lagi pada putaran berikutnya
				slog.Error("retensi hard delete gagal", "document_id", docs[i].ID, "error", err)
				continue
			}
			rep.HardDeleted++
		}
	}
}

func (s *RetentionService) expireSoft(ctx context.Context, d *domain.Document) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := cancelVerification(tx, d.ID); err != nil {
			return err
		}
		if err := tx.Model(&domain.Document{}).Where("id = ?", d.ID).
			Updates(map[string]any{
				"retention_status":    "expired",
				"verification_status": nil,
				"visibility":          "private",
			}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&domain.Document{}, d.ID).Error; err != nil {
			return err
		}
		meta := domain.JSONMap{}
		if d.ArchivedAt != nil {
			meta["archived_at"] = d.ArchivedAt.UTC().Format(time.RFC3339)
		}
		return writeLog(tx, "expired_soft_delete", &d.ID, nil, meta)
	})
}

func (s *RetentionService) expireHard(ctx context.Context, d *domain.Document) error {
	action := "deleted_hard_delete" // dihapus pengguna, bukan karena kedaluwarsa
	if d.RetentionStatus != nil && *d.RetentionStatus == "expired" {
		action = "expired_hard_delete"
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := writeLog(tx, action, &d.ID, nil, domain.JSONMap{"title": d.Title}); err != nil {
			return err
		}
		if err := tx.Unscoped().Delete(&domain.Document{}, d.ID).Error; err != nil {
			return err
		}
		if err := s.store.Delete(ctx, d.StoragePath); err != nil {
			return fmt.Errorf("hapus objek GCS: %w", err)
		}
		return nil
	})
}
