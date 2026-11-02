package service

import (
	"context"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type VerificationService struct {
	db  *gorm.DB
	now func() time.Time
}

func NewVerificationService(db *gorm.DB) *VerificationService {
	return &VerificationService{db: db, now: time.Now}
}

func (s *VerificationService) SetNowFunc(f func() time.Time) {
	s.now = f
}

type VerificationFilter struct {
	Status     string
	Prodi      string
	DocumentID *uint
	Page       int
	PerPage    int
}

func verificationRelations(db *gorm.DB) *gorm.DB {
	unscoped := func(tx *gorm.DB) *gorm.DB { return tx.Unscoped() }
	return db.Preload("Document", unscoped).Preload("Document.Prodi").Preload("Requester").Preload("Verifier")
}

func (s *VerificationService) List(ctx context.Context, f VerificationFilter) ([]domain.Verification, int64, error) {
	q := s.db.WithContext(ctx).Model(&domain.Verification{})
	if f.Status != "" {
		q = q.Where("document_verifications.status = ?", f.Status)
	}
	if f.DocumentID != nil {
		q = q.Where("document_verifications.document_id = ?", *f.DocumentID)
	}
	if f.Prodi != "" {
		q = q.Where("document_verifications.document_id IN (SELECT d.id FROM documents d JOIN prodi p ON p.id = d.prodi_id WHERE p.code = ?)", f.Prodi)
	}
	base := q.Session(&gorm.Session{})
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	order := "document_verifications.requested_at DESC, document_verifications.id DESC"
	if f.Status == "pending" {
		order = "document_verifications.requested_at ASC, document_verifications.id ASC" // paling lama menunggu dulu
	}
	if f.PerPage <= 0 {
		f.PerPage = 15
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	items := make([]domain.Verification, 0, f.PerPage)
	err := verificationRelations(base).Order(order).Offset((f.Page - 1) * f.PerPage).Limit(f.PerPage).Find(&items).Error
	return items, total, err
}

func (s *VerificationService) Get(ctx context.Context, id uint) (*domain.Verification, error) {
	var v domain.Verification
	if err := verificationRelations(s.db.WithContext(ctx)).First(&v, id).Error; err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *VerificationService) Decide(ctx context.Context, actor *domain.User, id uint, status, reason string) (*domain.Verification, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var probe domain.Verification
		if err := tx.Select("id", "document_id").First(&probe, id).Error; err != nil {
			return err
		}
		var doc domain.Document
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&doc, probe.DocumentID).Error; err != nil {
			return err
		}
		var v domain.Verification
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, id).Error; err != nil {
			return err
		}
		if v.Status != "pending" {
			return apperr.Conflict("Berkas ini sudah diverifikasi.")
		}
		var reasonVal any
		if status == "rejected" {
			reasonVal = reason
		}
		if err := tx.Model(&v).Updates(map[string]any{
			"status":      status,
			"reason":      reasonVal,
			"verified_by": actor.ID,
			"verified_at": s.now().UTC(),
		}).Error; err != nil {
			return err
		}

		docUpdate := map[string]any{"visibility": "private", "verification_status": "rejected"}
		title, body := "Berkas Anda ditolak", `"`+doc.Title+`" ditolak. Alasan: `+reason
		if status == "approved" {
			docUpdate = map[string]any{"visibility": "public", "verification_status": "approved"}
			title, body = "Berkas Anda disetujui", `"`+doc.Title+`" kini tampil di explorer publik.`
		}
		if err := tx.Model(&doc).Updates(docUpdate).Error; err != nil {
			return err
		}

		meta := domain.JSONMap{}
		if status == "rejected" {
			meta["reason"] = reason
		}
		if err := writeLog(tx, "verify_"+status, &doc.ID, &actor.ID, meta); err != nil {
			return err
		}
		return notifyUser(tx, doc.UploaderID, "verification_"+status, title, body,
			domain.JSONMap{"document_id": doc.ID, "verification_id": v.ID})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}
