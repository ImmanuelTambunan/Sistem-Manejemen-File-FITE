package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DocumentService struct {
	db    *gorm.DB
	store storage.Storage
	now   func() time.Time
}

func NewDocumentService(db *gorm.DB, store storage.Storage) *DocumentService {
	return &DocumentService{db: db, store: store, now: time.Now}
}

func (s *DocumentService) SetNowFunc(f func() time.Time) {
	s.now = f
}

// DocumentFilter: parameter query GET /documents (tag binding dipakai langsung oleh handler).
type DocumentFilter struct {
	Q            string `form:"q" binding:"omitempty,max=100"`
	Prodi        string `form:"prodi" binding:"omitempty,oneof=IF SI TE"`
	UploaderRole string `form:"uploader_role" binding:"omitempty,oneof=super_admin dekan baak kaprodi dosen mahasiswa"`
	CategoryID   *uint  `form:"category_id" binding:"omitempty,gt=0"`
	NIM          string `form:"nim" binding:"omitempty,max=30"`
	NIP          string `form:"nip" binding:"omitempty,max=30"`
	Extension    string `form:"extension" binding:"omitempty,oneof=pdf docx xlsx pptx zip png jpg"`
	Visibility   string `form:"visibility" binding:"omitempty,oneof=private public_approved"`
	Sort         string `form:"sort" binding:"omitempty,oneof=created_at -created_at title -title size_kb -size_kb"`
	Page         *int   `form:"page" binding:"omitempty,min=1"`
	PerPage      *int   `form:"per_page" binding:"omitempty,min=1,max=100"`
}

func (f DocumentFilter) PageNum() int {
	if f.Page != nil {
		return *f.Page
	}
	return 1
}

func (f DocumentFilter) PerPageNum() int {
	if f.PerPage != nil {
		return *f.PerPage
	}
	return 15
}

var sortClauses = map[string]string{
	"":            "documents.created_at DESC, documents.id DESC",
	"created_at":  "documents.created_at ASC, documents.id ASC",
	"-created_at": "documents.created_at DESC, documents.id DESC",
	"title":       "documents.title ASC, documents.id ASC",
	"-title":      "documents.title DESC, documents.id DESC",
	"size_kb":     "documents.size_kb ASC, documents.id ASC",
	"-size_kb":    "documents.size_kb DESC, documents.id DESC",
}

func likePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + r.Replace(s) + "%"
}

func applyFilter(q *gorm.DB, f DocumentFilter, public bool) *gorm.DB {
	if f.Q != "" {
		p := likePattern(f.Q)
		q = q.Where("(documents.title LIKE ? OR documents.original_name LIKE ?)", p, p)
	}
	if f.Prodi != "" {
		q = q.Where("documents.prodi_id = (SELECT id FROM prodi WHERE code = ?)", f.Prodi)
	}
	if f.CategoryID != nil {
		q = q.Where("documents.category_id = ?", *f.CategoryID)
	}
	if f.Extension != "" {
		q = q.Where("documents.extension = ?", f.Extension)
	}
	if public {
		return q
	}
	if f.UploaderRole != "" {
		q = q.Where("documents.uploader_id IN (SELECT u.id FROM users u JOIN roles r ON r.id = u.role_id WHERE r.`key` = ?)", f.UploaderRole)
	}
	if f.NIM != "" {
		q = q.Where("documents.owner_id IN (SELECT id FROM users WHERE nim = ?)", f.NIM)
	}
	if f.NIP != "" {
		q = q.Where("documents.owner_id IN (SELECT id FROM users WHERE nip = ?)", f.NIP)
	}
	switch f.Visibility {
	case "private":
		q = q.Where("documents.visibility = ?", "private")
	case "public_approved":
		q = q.Where("documents.visibility = ? AND documents.verification_status = ?", "public", "approved")
	}
	return q
}

func withRelations(db *gorm.DB) *gorm.DB {
	unscoped := func(tx *gorm.DB) *gorm.DB { return tx.Unscoped() }
	return db.Preload("Category").Preload("Prodi").
		Preload("Uploader", unscoped).Preload("Uploader.Role").
		Preload("Owner", unscoped).Preload("Owner.Role")
}

func (s *DocumentService) List(ctx context.Context, actor *domain.User, f DocumentFilter, publicOnly bool) ([]domain.Document, int64, error) {
	q := s.db.WithContext(ctx).Model(&domain.Document{})
	if publicOnly {
		q = q.Where("documents.visibility = ? AND documents.verification_status = ?", "public", "approved")
	} else {
		q = q.Scopes(VisibleTo(actor))
	}
	base := applyFilter(q, f, publicOnly).Session(&gorm.Session{})
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	docs := make([]domain.Document, 0, f.PerPageNum())
	err := withRelations(base).Order(sortClauses[f.Sort]).
		Offset((f.PageNum() - 1) * f.PerPageNum()).Limit(f.PerPageNum()).Find(&docs).Error
	return docs, total, err
}

func (s *DocumentService) Get(ctx context.Context, id uint) (*domain.Document, error) {
	var d domain.Document
	if err := withRelations(s.db.WithContext(ctx)).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *DocumentService) GetUnscoped(ctx context.Context, id uint) (*domain.Document, error) {
	var d domain.Document
	if err := withRelations(s.db.WithContext(ctx).Unscoped()).First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *DocumentService) CanView(ctx context.Context, actor *domain.User, d *domain.Document) (bool, error) {
	if d.Visibility == "public" && d.VerificationStatus != nil && *d.VerificationStatus == "approved" {
		return true, nil
	}
	var n int64
	err := s.db.WithContext(ctx).Model(&domain.Document{}).Scopes(VisibleTo(actor)).
		Where("documents.id = ?", d.ID).Count(&n).Error
	return n > 0, err
}

type CreateInput struct {
	Actor         *domain.User
	Title         string
	Description   *string
	CategoryID    uint
	Public        bool
	ProdiID       *uint
	SubjectUserID *uint
	ArchivedAt    *time.Time
	Upload        *UploadInfo
}

func (s *DocumentService) Create(ctx context.Context, in CreateInput) (*domain.Document, error) {
	actor := in.Actor
	var cat domain.Category
	if err := s.db.WithContext(ctx).First(&cat, in.CategoryID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.FieldError("category_id", "Kategori tidak ditemukan.")
		}
		return nil, err
	}
	if !cat.AllowedRoles.Contains(actor.Role.Key) {
		return nil, apperr.Forbidden("Kategori ini tidak diizinkan untuk peran Anda.")
	}

	privileged := actor.Role.Key == domain.RoleSuperAdmin || actor.Role.Key == domain.RoleBaak
	if !privileged {
		fields := map[string][]string{}
		if in.SubjectUserID != nil {
			fields["subject_user_id"] = []string{"Field ini tidak diizinkan untuk peran Anda."}
		}
		if in.ArchivedAt != nil {
			fields["archived_at"] = []string{"Field ini tidak diizinkan untuk peran Anda."}
		}
		if len(fields) > 0 {
			return nil, apperr.Validation(fields)
		}
	}

	owner := actor
	if in.SubjectUserID != nil && *in.SubjectUserID != actor.ID {
		var subj domain.User
		err := s.db.WithContext(ctx).Preload("Role").Preload("Prodi").First(&subj, *in.SubjectUserID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperr.FieldError("subject_user_id", "Pengguna subjek tidak ditemukan.")
		}
		if err != nil {
			return nil, err
		}
		switch subj.Role.Key {
		case domain.RoleMahasiswa, domain.RoleDosen, domain.RoleKaprodi:
		default:
			return nil, apperr.FieldError("subject_user_id", "Subjek berkas harus mahasiswa, dosen, atau kaprodi.")
		}
		owner = &subj
	}

	var prodiID *uint
	switch {
	case owner.ProdiID != nil:
		if in.ProdiID != nil && *in.ProdiID != *owner.ProdiID {
			return nil, apperr.FieldError("prodi_id", "Prodi tidak sesuai dengan prodi pemilik berkas.")
		}
		prodiID = owner.ProdiID
	case privileged:
		if in.ProdiID == nil {
			return nil, apperr.FieldError("prodi_id", "Program studi wajib dipilih.")
		}
		prodiID = in.ProdiID
	}

	prodiCode := "fakultas"
	if prodiID != nil {
		var p domain.Prodi
		if err := s.db.WithContext(ctx).First(&p, *prodiID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperr.FieldError("prodi_id", "Program studi tidak ditemukan.")
			}
			return nil, err
		}
		if !p.IsActive {
			return nil, apperr.FieldError("prodi_id", "Program studi tidak aktif.")
		}
		prodiCode = p.Code
	}

	now := s.now().UTC()
	archived := now
	if in.ArchivedAt != nil {
		archived = in.ArchivedAt.UTC()
	}

	path := fmt.Sprintf("documents/%s/%04d/%02d/%s.%s", prodiCode, now.Year(), int(now.Month()), uuid.NewString(), in.Upload.Ext)
	if _, err := in.Upload.File.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	if err := s.store.Put(ctx, path, in.Upload.File, in.Upload.Mime); err != nil {
		return nil, apperr.Internal(fmt.Errorf("gagal menyimpan ke GCS: %w", err))
	}

	visibility := "private"
	var vstatus *string
	if in.Public {
		visibility = "public"
		p := "pending"
		vstatus = &p
	}

	doc := domain.Document{
		Title:              in.Title,
		Description:        in.Description,
		OriginalName:       in.Upload.OriginalName,
		Extension:          in.Upload.Ext,
		MimeType:           in.Upload.Mime,
		SizeKB:             uint(max64(1, (in.Upload.SizeBytes+1023)/1024)),
		StoragePath:        path,
		CategoryID:         cat.ID,
		ProdiID:            prodiID,
		UploaderID:         actor.ID,
		OwnerID:            owner.ID,
		OwnerRole:          owner.Role.Key,
		Visibility:         visibility,
		VerificationStatus: vstatus,
		ArchivedAt:         &archived,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit(clause.Associations).Create(&doc).Error; err != nil {
			return err
		}
		if in.Public {
			if err := openVerification(tx, &doc, actor.Name); err != nil {
				return err
			}
		}
		return writeLog(tx, "upload", &doc.ID, &actor.ID, domain.JSONMap{"size_kb": doc.SizeKB, "visibility": doc.Visibility})
	})
	if err != nil {
		cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if derr := s.store.Delete(cctx, path); derr != nil {
			slog.Error("kompensasi hapus objek GCS gagal", "path", path, "error", derr)
		}
		return nil, err
	}
	return s.Get(ctx, doc.ID)
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type UpdateMetaInput struct {
	Title       *string
	Description *string
	CategoryID  *uint
}

func (s *DocumentService) UpdateMeta(ctx context.Context, actor *domain.User, d *domain.Document, in UpdateMetaInput) (*domain.Document, error) {
	updates := map[string]any{}
	if in.Title != nil {
		updates["title"] = *in.Title
	}
	if in.Description != nil {
		if *in.Description == "" {
			updates["description"] = nil
		} else {
			updates["description"] = *in.Description
		}
	}
	if in.CategoryID != nil {
		var cat domain.Category
		if err := s.db.WithContext(ctx).First(&cat, *in.CategoryID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, apperr.FieldError("category_id", "Kategori tidak ditemukan.")
			}
			return nil, err
		}
		if !cat.AllowedRoles.Contains(d.Uploader.Role.Key) {
			return nil, apperr.FieldError("category_id", "Kategori tidak sesuai dengan peran pengunggah berkas.")
		}
		updates["category_id"] = cat.ID
	}
	if len(updates) == 0 {
		return nil, apperr.FieldError("body", "Tidak ada perubahan yang dikirim.")
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.Document{}).Where("id = ?", d.ID).Updates(updates).Error; err != nil {
			return err
		}
		keys := make([]string, 0, len(updates))
		for k := range updates {
			keys = append(keys, k)
		}
		return writeLog(tx, "update", &d.ID, &actor.ID, domain.JSONMap{"fields": keys})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, d.ID)
}

func (s *DocumentService) ChangeVisibility(ctx context.Context, actor *domain.User, id uint, target string) (*domain.Document, error) {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var doc domain.Document
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&doc, id).Error; err != nil {
			return err
		}
		if target == "public" {
			if doc.Visibility == "public" {
				return apperr.Conflict("Berkas sudah berstatus publik.")
			}
			pending := "pending"
			if err := tx.Model(&doc).Updates(map[string]any{"visibility": "public", "verification_status": pending}).Error; err != nil {
				return err
			}
			var up domain.User
			if err := tx.Unscoped().First(&up, doc.UploaderID).Error; err != nil {
				return err
			}
			if err := openVerification(tx, &doc, up.Name); err != nil {
				return err
			}
		} else {
			if doc.Visibility == "private" {
				return nil // idempoten
			}
			if err := cancelVerification(tx, doc.ID); err != nil {
				return err
			}
			if err := tx.Model(&doc).Updates(map[string]any{"visibility": "private", "verification_status": nil}).Error; err != nil {
				return err
			}
		}
		return writeLog(tx, "visibility_change", &doc.ID, &actor.ID, domain.JSONMap{"to": target})
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

func (s *DocumentService) Download(ctx context.Context, actor *domain.User, d *domain.Document) (io.ReadCloser, int64, error) {
	rc, size, err := s.store.Open(ctx, d.StoragePath)
	if errors.Is(err, storage.ErrNotFound) {
		return nil, 0, apperr.NotFound("Objek berkas tidak ditemukan di penyimpanan.")
	}
	if err != nil {
		return nil, 0, err
	}
	if lerr := writeLog(s.db.WithContext(ctx), "download", &d.ID, &actor.ID, nil); lerr != nil {
		slog.Error("log unduhan gagal", "document_id", d.ID, "error", lerr)
	}
	return rc, size, nil
}

func (s *DocumentService) Delete(ctx context.Context, actor *domain.User, d *domain.Document, force bool) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := cancelVerification(tx, d.ID); err != nil {
			return err
		}
		if force {
			if err := writeLog(tx, "hard_delete", &d.ID, &actor.ID, domain.JSONMap{"title": d.Title}); err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&domain.Document{}, d.ID).Error; err != nil {
				return err
			}
			return s.store.Delete(ctx, d.StoragePath)
		}
		if err := tx.Model(&domain.Document{}).Where("id = ?", d.ID).
			Updates(map[string]any{"verification_status": nil, "visibility": "private"}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&domain.Document{}, d.ID).Error; err != nil {
			return err
		}
		return writeLog(tx, "soft_delete", &d.ID, &actor.ID, nil)
	})
}
