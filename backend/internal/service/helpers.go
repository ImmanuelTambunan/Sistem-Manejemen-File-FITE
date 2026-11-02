package service

import (
	"strconv"
	"strings"
	"time"

	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/storage"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ storage.Storage // pastikan paket terpakai oleh seluruh service

func writeLog(tx *gorm.DB, action string, docID, userID *uint, meta domain.JSONMap) error {
	return tx.Create(&domain.StorageLog{DocumentID: docID, UserID: userID, Action: action, Meta: meta}).Error
}

func notifyRoles(tx *gorm.DB, roleKeys []string, typ, title, msg string, data domain.JSONMap) error {
	var ids []uint
	err := tx.Model(&domain.User{}).
		Joins("JOIN roles ON roles.id = users.role_id").
		Where("users.is_active = ? AND roles.`key` IN ?", true, roleKeys).
		Pluck("users.id", &ids).Error
	if err != nil || len(ids) == 0 {
		return err
	}
	rows := make([]domain.Notification, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, domain.Notification{ID: uuid.NewString(), UserID: id, Type: typ, Title: title, Message: msg, Data: data})
	}
	return tx.CreateInBatches(&rows, 100).Error
}

func notifyUser(tx *gorm.DB, userID uint, typ, title, msg string, data domain.JSONMap) error {
	return tx.Create(&domain.Notification{ID: uuid.NewString(), UserID: userID, Type: typ, Title: title, Message: msg, Data: data}).Error
}

// cancelVerification menghapus baris verifikasi dan notifikasi antrean terkait (tanpa rujukan menggantung).
func cancelVerification(tx *gorm.DB, docID uint) error {
	if err := tx.Where("document_id = ?", docID).Delete(&domain.Verification{}).Error; err != nil {
		return err
	}
	return tx.Where("type = ? AND JSON_UNQUOTE(JSON_EXTRACT(data, '$.document_id')) = ?",
		"verification_pending", strconv.FormatUint(uint64(docID), 10)).Delete(&domain.Notification{}).Error
}

// openVerification membuka (atau membuka ulang) antrean dan memberi tahu BAAK dan Super Admin.
func openVerification(tx *gorm.DB, doc *domain.Document, uploaderName string) error {
	now := time.Now().UTC()
	v := domain.Verification{DocumentID: doc.ID, Status: "pending", RequestedBy: doc.UploaderID, RequestedAt: now}
	err := tx.Omit(clause.Associations).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "document_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"status": "pending", "reason": nil, "requested_by": doc.UploaderID,
			"verified_by": nil, "requested_at": now, "verified_at": nil, "updated_at": now,
		}),
	}).Create(&v).Error
	if err != nil {
		return err
	}
	if err := tx.Where("document_id = ?", doc.ID).First(&v).Error; err != nil {
		return err
	}
	return notifyRoles(tx, []string{domain.RoleBaak, domain.RoleSuperAdmin},
		"verification_pending", "Berkas publik baru menunggu verifikasi",
		doc.Title+" oleh "+uploaderName,
		domain.JSONMap{"document_id": doc.ID, "verification_id": v.ID})
}

// VisibleTo: cakupan baca per peran (sumber kebenaran tunggal untuk daftar dan otorisasi).
func VisibleTo(u *domain.User) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		switch u.Role.Key {
		case domain.RoleSuperAdmin, domain.RoleDekan, domain.RoleBaak:
			return db
		}
		conds := []string{"documents.uploader_id = ?", "documents.owner_id = ?"}
		args := []any{u.ID, u.ID}
		switch u.Role.Key {
		case domain.RoleKaprodi:
			conds = append(conds, "documents.owner_role = ?")
			args = append(args, domain.RoleDosen) // berkas dosen se-fakultas
			if u.ProdiID != nil {
				conds = append(conds, "(documents.owner_role = ? AND documents.prodi_id = ?)")
				args = append(args, domain.RoleMahasiswa, *u.ProdiID)
			}
		case domain.RoleDosen:
			if u.ProdiID != nil {
				conds = append(conds, "(documents.owner_role = ? AND documents.prodi_id = ?)")
				args = append(args, domain.RoleMahasiswa, *u.ProdiID)
			}
		}
		return db.Where("("+strings.Join(conds, " OR ")+")", args...)
	}
}

func CanUpdateDocument(u *domain.User, d *domain.Document) bool {
	return u.Role.Key == domain.RoleSuperAdmin || d.UploaderID == u.ID ||
		(u.Role.Key == domain.RoleBaak && d.ProdiID != nil)
}

func CanChangeVisibility(u *domain.User, d *domain.Document) bool {
	return u.Role.Key == domain.RoleSuperAdmin || d.UploaderID == u.ID
}

// CanDeleteDocument: SA semua; pengunggah miliknya; BAAK berkas kategori operasional.
func CanDeleteDocument(u *domain.User, d *domain.Document) bool {
	if u.Role.Key == domain.RoleSuperAdmin || d.UploaderID == u.ID {
		return true
	}
	return u.Role.Key == domain.RoleBaak && d.Category.IsOperational
}

