package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/service"
	"fite-arsip-api/internal/storage"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestRetentionService_Cycles(t *testing.T) {
	now := time.Date(2028, 3, 1, 10, 0, 0, 0, time.UTC)
	db, err := gorm.Open(sqlite.Open("file:memdb_retention?mode=memory&cache=shared"), &gorm.Config{
		NowFunc: func() time.Time { return now },
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	_ = db.AutoMigrate(
		&domain.Role{},
		&domain.Prodi{},
		&domain.User{},
		&domain.AuthSession{},
		&domain.Category{},
		&domain.Document{},
		&domain.Verification{},
		&domain.Notification{},
		&domain.StorageLog{},
	)

	memStore := storage.NewMemoryStorage()
	cfg := config.Config{
		RetentionYears: 8,
		GraceDays:      90,
	}

	retSvc := service.NewRetentionService(db, memStore, cfg)
	ctx := context.Background()

	retSvc.Now = func() time.Time { return now }

	// Seed roles, prodi, and user
	mhs := seedUser(t, db, domain.RoleMahasiswa, "ret.mhs@del.ac.id", "Secret123", true, false)
	cat := domain.Category{Name: "Umum", AllowedRoles: domain.StringList{domain.RoleMahasiswa, domain.RoleSuperAdmin}}
	db.Create(&cat)

	// TC-RET-01: 7 years 11 months ago
	t1 := now.AddDate(-7, -11, 0)
	doc1 := domain.Document{
		Title: "Doc 7y11m", OriginalName: "1.pdf", Extension: "pdf", MimeType: "application/pdf",
		SizeKB: 100, StoragePath: "path1.pdf", CategoryID: cat.ID, UploaderID: mhs.ID, OwnerID: mhs.ID,
		OwnerRole: domain.RoleMahasiswa, Visibility: "private", ArchivedAt: &t1,
	}
	db.Create(&doc1)
	_ = memStore.Put(ctx, "path1.pdf", strings.NewReader("1"), "application/pdf")

	// TC-RET-02: Exactly 8 years ago
	t2 := now.AddDate(-8, 0, 0)
	doc2 := domain.Document{
		Title: "Doc exactly 8y", OriginalName: "2.pdf", Extension: "pdf", MimeType: "application/pdf",
		SizeKB: 100, StoragePath: "path2.pdf", CategoryID: cat.ID, UploaderID: mhs.ID, OwnerID: mhs.ID,
		OwnerRole: domain.RoleMahasiswa, Visibility: "private", ArchivedAt: &t2,
	}
	db.Create(&doc2)
	_ = memStore.Put(ctx, "path2.pdf", strings.NewReader("2"), "application/pdf")

	// TC-RET-03: 8 years 1 day ago -> should be soft deleted
	t3 := now.AddDate(-8, 0, -1)
	doc3 := domain.Document{
		Title: "Doc 8y1d", OriginalName: "3.pdf", Extension: "pdf", MimeType: "application/pdf",
		SizeKB: 100, StoragePath: "path3.pdf", CategoryID: cat.ID, UploaderID: mhs.ID, OwnerID: mhs.ID,
		OwnerRole: domain.RoleMahasiswa, Visibility: "private", ArchivedAt: &t3,
	}
	db.Create(&doc3)
	_ = memStore.Put(ctx, "path3.pdf", strings.NewReader("3"), "application/pdf")

	// Soft deleted 89 days ago -> within grace period, not hard deleted
	t4 := now.AddDate(0, 0, -89)
	doc4 := domain.Document{
		Title: "Doc soft 89d", OriginalName: "4.pdf", Extension: "pdf", MimeType: "application/pdf",
		SizeKB: 100, StoragePath: "path4.pdf", CategoryID: cat.ID, UploaderID: mhs.ID, OwnerID: mhs.ID,
		OwnerRole: domain.RoleMahasiswa, Visibility: "private", DeletedAt: gorm.DeletedAt{Time: t4, Valid: true},
	}
	db.Create(&doc4)
	_ = memStore.Put(ctx, "path4.pdf", strings.NewReader("4"), "application/pdf")

	// Soft deleted 91 days ago (expired retention) -> should be hard deleted
	t5 := now.AddDate(0, 0, -91)
	expStatus := "expired"
	doc5 := domain.Document{
		Title: "Doc soft 91d expired", OriginalName: "5.pdf", Extension: "pdf", MimeType: "application/pdf",
		SizeKB: 100, StoragePath: "path5.pdf", CategoryID: cat.ID, UploaderID: mhs.ID, OwnerID: mhs.ID,
		OwnerRole: domain.RoleMahasiswa, Visibility: "private", RetentionStatus: &expStatus,
		DeletedAt: gorm.DeletedAt{Time: t5, Valid: true},
	}
	db.Create(&doc5)
	_ = memStore.Put(ctx, "path5.pdf", strings.NewReader("5"), "application/pdf")

	// Soft deleted 91 days ago by user (not expired) -> should be hard deleted with deleted_hard_delete
	t6 := now.AddDate(0, 0, -91)
	doc6 := domain.Document{
		Title: "Doc soft 91d user", OriginalName: "6.pdf", Extension: "pdf", MimeType: "application/pdf",
		SizeKB: 100, StoragePath: "path6.pdf", CategoryID: cat.ID, UploaderID: mhs.ID, OwnerID: mhs.ID,
		OwnerRole: domain.RoleMahasiswa, Visibility: "private",
		DeletedAt: gorm.DeletedAt{Time: t6, Valid: true},
	}
	db.Create(&doc6)
	_ = memStore.Put(ctx, "path6.pdf", strings.NewReader("6"), "application/pdf")

	// Anomaly: Doc with NULL archived_at
	doc7 := domain.Document{
		Title: "Doc null archived_at", OriginalName: "7.pdf", Extension: "pdf", MimeType: "application/pdf",
		SizeKB: 100, StoragePath: "path7.pdf", CategoryID: cat.ID, UploaderID: mhs.ID, OwnerID: mhs.ID,
		OwnerRole: domain.RoleMahasiswa, Visibility: "private", ArchivedAt: nil,
	}
	db.Create(&doc7)

	// Run retention
	rep, err := retSvc.Run(ctx)
	if err != nil {
		t.Fatalf("retention run failed: %v", err)
	}

	if rep.SoftDeleted != 1 {
		t.Errorf("expected 1 soft deleted doc, got %d", rep.SoftDeleted)
	}
	if rep.HardDeleted != 2 {
		t.Errorf("expected 2 hard deleted docs, got %d", rep.HardDeleted)
	}
	if rep.Anomalies < 1 {
		t.Errorf("expected at least 1 anomaly, got %d", rep.Anomalies)
	}

	// Verify doc3 is now soft deleted
	var checkDoc3 domain.Document
	err = db.First(&checkDoc3, doc3.ID).Error
	if err == nil {
		t.Errorf("expected doc3 to be soft deleted from normal queries")
	}
	db.Unscoped().First(&checkDoc3, doc3.ID)
	if checkDoc3.RetentionStatus == nil || *checkDoc3.RetentionStatus != "expired" {
		t.Errorf("expected retention status expired for doc3")
	}

	// Verify doc5 and doc6 are completely gone (hard deleted)
	var checkDoc5, checkDoc6 domain.Document
	if db.Unscoped().First(&checkDoc5, doc5.ID).Error == nil {
		t.Errorf("expected doc5 to be permanently deleted")
	}
	if db.Unscoped().First(&checkDoc6, doc6.ID).Error == nil {
		t.Errorf("expected doc6 to be permanently deleted")
	}

	// Verify GCS storage deleted for doc5 and doc6
	if _, _, err := memStore.Open(ctx, "path5.pdf"); err == nil {
		t.Errorf("expected path5.pdf deleted from storage")
	}
	if _, _, err := memStore.Open(ctx, "path6.pdf"); err == nil {
		t.Errorf("expected path6.pdf deleted from storage")
	}

	// Verify doc1 and doc2 and doc4 storage objects still exist
	if _, _, err := memStore.Open(ctx, "path1.pdf"); err != nil {
		t.Errorf("expected path1.pdf to remain in storage")
	}
	if _, _, err := memStore.Open(ctx, "path4.pdf"); err != nil {
		t.Errorf("expected path4.pdf to remain in storage")
	}

	// TC-RET-07: Idempotency check - running again changes nothing
	rep2, err := retSvc.Run(ctx)
	if err != nil {
		t.Fatalf("second retention run failed: %v", err)
	}
	if rep2.SoftDeleted != 0 || rep2.HardDeleted != 0 {
		t.Errorf("expected 0 soft and 0 hard on second run, got soft=%d hard=%d", rep2.SoftDeleted, rep2.HardDeleted)
	}
}


func TestRetentionService_SessionPurge(t *testing.T) {
	now := time.Date(2028, 3, 1, 10, 0, 0, 0, time.UTC)
	db := setupTestDB(t)

	memStore := storage.NewMemoryStorage()
	cfg := config.Config{RetentionYears: 8, GraceDays: 90}
	retSvc := service.NewRetentionService(db, memStore, cfg)
	retSvc.Now = func() time.Time { return now }
	ctx := context.Background()

	// 1. Session older than 30 days (expires_at = 31 days ago) -> should be purged
	tOld := now.AddDate(0, 0, -31)
	sOld := domain.AuthSession{
		ID:                "sess-old-31d",
		UserID:            1,
		RefreshTokenHash:  "hash-old",
		ExpiresAt:         tOld,
		AbsoluteExpiresAt: tOld,
	}
	db.Create(&sOld)

	// 2. Active session (expires_at = 5 days from now) -> should be preserved
	tActive := now.AddDate(0, 0, 5)
	sActive := domain.AuthSession{
		ID:                "sess-active",
		UserID:            1,
		RefreshTokenHash:  "hash-active",
		ExpiresAt:         tActive,
		AbsoluteExpiresAt: now.AddDate(0, 0, 20),
	}
	db.Create(&sActive)

	// 3. Recently revoked session (revoked_at = 5 days ago) -> preserved for audit
	tRecentRevoke := now.AddDate(0, 0, -5)
	sRevoked := domain.AuthSession{
		ID:                "sess-recent-revoke",
		UserID:            1,
		RefreshTokenHash:  "hash-recent-revoke",
		ExpiresAt:         tActive,
		AbsoluteExpiresAt: now.AddDate(0, 0, 20),
		RevokedAt:         &tRecentRevoke,
	}
	db.Create(&sRevoked)

	rep, err := retSvc.Run(ctx)
	if err != nil {
		t.Fatalf("retention run failed: %v", err)
	}

	if rep.Sessions != 1 {
		t.Errorf("expected 1 session purged, got %d", rep.Sessions)
	}

	var countActive, countOld, countRevoked int64
	db.Model(&domain.AuthSession{}).Where("id = ?", "sess-active").Count(&countActive)
	db.Model(&domain.AuthSession{}).Where("id = ?", "sess-old-31d").Count(&countOld)
	db.Model(&domain.AuthSession{}).Where("id = ?", "sess-recent-revoke").Count(&countRevoked)

	if countActive != 1 {
		t.Errorf("expected active session to remain")
	}
	if countOld != 0 {
		t.Errorf("expected old session to be deleted")
	}
	if countRevoked != 1 {
		t.Errorf("expected recently revoked session to remain")
	}
}

type failingDeleteStorage struct {
	storage.Storage
}

func (f *failingDeleteStorage) Delete(ctx context.Context, path string) error {
	return errors.New("simulated network timeout deleting from GCS")
}

func TestRetentionService_GCSFailureRollback(t *testing.T) {
	now := time.Date(2028, 3, 1, 10, 0, 0, 0, time.UTC)
	db := setupTestDB(t)

	baseMem := storage.NewMemoryStorage()
	failingStore := &failingDeleteStorage{Storage: baseMem}
	cfg := config.Config{RetentionYears: 8, GraceDays: 90}
	retSvc := service.NewRetentionService(db, failingStore, cfg)
	retSvc.Now = func() time.Time { return now }
	ctx := context.Background()

	// Soft-deleted 95 days ago -> eligible for hard delete
	tDeleted := now.AddDate(0, 0, -95)
	doc := domain.Document{
		ID:           999,
		Title:        "Doc Fail GCS",
		StoragePath:  "path_fail.pdf",
		DeletedAt:    gorm.DeletedAt{Time: tDeleted, Valid: true},
	}
	db.Create(&doc)

	rep, err := retSvc.Run(ctx)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	// GCS delete failed, so document transaction was rolled back
	if rep.Failed != 1 {
		t.Errorf("expected rep.Failed = 1, got %d", rep.Failed)
	}
	if rep.HardDeleted != 0 {
		t.Errorf("expected rep.HardDeleted = 0 on failure, got %d", rep.HardDeleted)
	}

	// Document should still exist in database for next retry
	var count int64
	db.Unscoped().Model(&domain.Document{}).Where("id = ?", 999).Count(&count)
	if count != 1 {
		t.Errorf("expected document to still exist in DB after GCS delete failure")
	}
}
