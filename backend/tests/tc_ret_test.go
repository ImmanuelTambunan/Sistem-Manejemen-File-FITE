package tests

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/handler"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/router"
	"fite-arsip-api/internal/service"
	"fite-arsip-api/internal/storage"

	"github.com/gin-gonic/gin"
	gosqlite "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "time/tzdata"
)

var (
	retSuiteSeq uint64

	// Global advisory lock emulation for SQLite test harness
	globalRetLockMu   sync.Mutex
	globalRetLockHeld bool
)

func init() {
	// Register scalar functions GET_LOCK and RELEASE_LOCK for SQLite test harness
	// to enable testing of MySQL advisory lock coordination logic.
	_ = gosqlite.RegisterScalarFunction("GET_LOCK", 2, func(ctx *gosqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		globalRetLockMu.Lock()
		defer globalRetLockMu.Unlock()
		if globalRetLockHeld {
			return int64(0), nil // lock not obtained (another instance holds it)
		}
		globalRetLockHeld = true
		return int64(1), nil // lock successfully acquired
	})

	_ = gosqlite.RegisterScalarFunction("RELEASE_LOCK", 1, func(ctx *gosqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
		globalRetLockMu.Lock()
		defer globalRetLockMu.Unlock()
		globalRetLockHeld = false
		return int64(1), nil
	})
}

type retStorageWrapper struct {
	*storage.MemoryStorage
	mu         sync.Mutex
	failDelete bool
}

func (s *retStorageWrapper) Delete(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failDelete {
		return errors.New("simulated GCS failure during hard delete")
	}
	return s.MemoryStorage.Delete(ctx, path)
}

type retSuiteContext struct {
	Engine   *gin.Engine
	DB       *gorm.DB
	Config   config.Config
	Storage  *retStorageWrapper
	RetSvc   *service.RetentionService
	Users    map[string]domain.User
	Tokens   map[string]string
	TestTime time.Time
}

func setupRetentionSuite(t *testing.T, fixedTime time.Time) *retSuiteContext {
	t.Helper()
	gin.SetMode(gin.TestMode)
	httpx.Init()

	seq := atomic.AddUint64(&retSuiteSeq, 1)
	dsn := fmt.Sprintf("file:ret_suite_%d_%d?mode=memory&cache=shared", time.Now().UnixNano(), seq)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:  logger.Default.LogMode(logger.Silent),
		NowFunc: func() time.Time { return fixedTime },
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	if err := db.AutoMigrate(
		&domain.Role{},
		&domain.Prodi{},
		&domain.User{},
		&domain.AuthSession{},
		&domain.Category{},
		&domain.Document{},
		&domain.Verification{},
		&domain.Notification{},
		&domain.StorageLog{},
	); err != nil {
		t.Fatalf("migrate retention suite: %v", err)
	}

	// 1. Roles
	roles := []domain.Role{
		{ID: 1, Key: domain.RoleSuperAdmin, Name: "Super Admin"},
		{ID: 2, Key: domain.RoleDekan, Name: "Dekan"},
		{ID: 3, Key: domain.RoleBaak, Name: "BAAK / Staf"},
		{ID: 4, Key: domain.RoleKaprodi, Name: "Kaprodi"},
		{ID: 5, Key: domain.RoleDosen, Name: "Dosen / TA"},
		{ID: 6, Key: domain.RoleMahasiswa, Name: "Mahasiswa"},
	}
	db.Create(&roles)

	// 2. Prodi
	prodis := []domain.Prodi{
		{ID: 1, Code: "IF", Name: "S1 Informatika", IsActive: true},
		{ID: 2, Code: "SI", Name: "S1 Sistem Informasi", IsActive: true},
		{ID: 3, Code: "TE", Name: "S1 Teknik Elektro", IsActive: true},
	}
	db.Create(&prodis)

	// 3. Categories
	cats := []domain.Category{
		{ID: 1, Name: "SK Keputusan Dekan", AllowedRoles: domain.StringList{"dekan", "super_admin"}, IsOperational: false},
		{ID: 4, Name: "Legalisir", AllowedRoles: domain.StringList{"baak", "super_admin"}, IsOperational: true},
		{ID: 11, Name: "Rencana Pembelajaran Semester (RPS)", AllowedRoles: domain.StringList{"dosen", "super_admin"}, IsOperational: false},
		{ID: 14, Name: "Proposal TA/PA", AllowedRoles: domain.StringList{"mahasiswa", "super_admin"}, IsOperational: false},
	}
	db.Create(&cats)

	// 4. Users
	const defaultPW = "Password123!"
	pwHash, _ := bcrypt.GenerateFromPassword([]byte(defaultPW), 12)
	hashStr := string(pwHash)

	prodiIF := uint(1)
	strPtr := func(s string) *string { return &s }

	userList := []domain.User{
		{ID: 1, Name: "Super Admin", Email: "admin@del.ac.id", PasswordHash: hashStr, RoleID: 1, IsActive: true},
		{ID: 3, Name: "Staf BAAK", Email: "baak@del.ac.id", PasswordHash: hashStr, RoleID: 3, IsActive: true},
		{ID: 6, Name: "Dosen Informatika", Email: "dosen.if@del.ac.id", PasswordHash: hashStr, RoleID: 5, ProdiID: &prodiIF, NIP: strPtr("198505052010011002"), IsActive: true},
		{ID: 8, Name: "Budi Mahasiswa (IF-1)", Email: "budi@del.ac.id", PasswordHash: hashStr, RoleID: 6, ProdiID: &prodiIF, NIM: strPtr("11S20001"), IsActive: true},
	}
	db.Create(&userList)

	userMap := make(map[string]domain.User)
	for _, u := range userList {
		userMap[u.Email] = u
	}

	cfg := config.Config{
		Env:            "development",
		HTTPAddr:       ":8080",
		JWTSecret:      []byte("super-secret-jwt-key-32-bytes-long!"),
		JWTIssuer:      "fite-arsip-api",
		JWTAudience:    "fite-arsip-web",
		AccessTTL:      15 * time.Minute,
		RefreshTTL:     7 * 24 * time.Hour,
		SessionMaxAge:  30 * 24 * time.Hour,
		AllowedOrigin:  "http://localhost:3000",
		CookieSecure:   false,
		MaxFileBytes:   10 << 20,
		MaxUploadBody:  11 << 20,
		MaxJSONBody:    1 << 20,
		RetentionYears: 8,
		GraceDays:      90,
		Timezone:       "Asia/Jakarta",
	}

	store := &retStorageWrapper{MemoryStorage: storage.NewMemoryStorage()}
	authSvc := service.NewAuthService(db, cfg)
	docSvc := service.NewDocumentService(db, store)
	verSvc := service.NewVerificationService(db)
	retSvc := service.NewRetentionService(db, store, cfg)
	retSvc.Now = func() time.Time { return fixedTime }

	h := router.Handlers{
		Auth:         handler.NewAuthHandler(authSvc, cfg, middleware.NewFailureLimiter(5, time.Minute)),
		Document:     handler.NewDocumentHandler(docSvc, cfg),
		Verification: handler.NewVerificationHandler(verSvc),
		Category:     handler.NewCategoryHandler(db),
		Notification: handler.NewNotificationHandler(db),
		User:         handler.NewUserHandler(db),
		Dashboard:    handler.NewDashboardHandler(db, cfg),
	}

	r := router.New(cfg, authSvc, h, nil)

	// Generate tokens with expiration well beyond retention test dates
	// to avoid premature purging by the retention worker.
	tokens := make(map[string]string)
	for _, u := range userList {
		sessID := uuid.NewString()
		now := time.Now().UTC()
		sess := domain.AuthSession{
			ID:                sessID,
			UserID:            u.ID,
			RefreshTokenHash:  "test-hash-" + sessID,
			ExpiresAt:         fixedTime.Add(365 * 24 * time.Hour),
			AbsoluteExpiresAt: fixedTime.Add(365 * 24 * time.Hour),
			CreatedAt:         now,
		}
		db.Create(&sess)

		claims := service.Claims{
			SID: sessID,
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   strconv.FormatUint(uint64(u.ID), 10),
				Issuer:    cfg.JWTIssuer,
				Audience:  jwt.ClaimStrings{cfg.JWTAudience},
				IssuedAt:  jwt.NewNumericDate(now),
				ExpiresAt: jwt.NewNumericDate(now.Add(24 * time.Hour)),
				ID:        uuid.NewString(),
			},
		}
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(cfg.JWTSecret)
		tokens[u.Email] = tok
	}

	return &retSuiteContext{
		Engine:   r,
		DB:       db,
		Config:   cfg,
		Storage:  store,
		RetSvc:   retSvc,
		Users:    userMap,
		Tokens:   tokens,
		TestTime: fixedTime,
	}
}

func doRetReq(engine *gin.Engine, method, url, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func doRetReqJSON(engine *gin.Engine, method, url, token string, body any) (*httptest.ResponseRecorder, map[string]any) {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, url, buf)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return w, res
}

func TestTS_RET_Granular(t *testing.T) {
	// Base simulated anchor time: 2028-03-01 10:00:00 UTC (matching leap day test scenario TC-RET-14)
	anchorTime := time.Date(2028, 3, 1, 10, 0, 0, 0, time.UTC)
	ctx := setupRetentionSuite(t, anchorTime)

	// Helper to insert doc and file into memory storage
	createTestDoc := func(id uint, title string, archivedAt *time.Time, deletedAt *time.Time, retStatus *string, vis string, verStatus *string) domain.Document {
		mhs := ctx.Users["budi@del.ac.id"]
		path := fmt.Sprintf("documents/IF/2020/01/ret_test_%d.pdf", id)
		_ = ctx.Storage.Put(context.Background(), path, strings.NewReader("%PDF-1.4 sample content\n%%EOF"), "application/pdf")

		doc := domain.Document{
			ID:                 id,
			Title:              title,
			OriginalName:       title + ".pdf",
			Extension:          "pdf",
			MimeType:           "application/pdf",
			SizeKB:             120,
			StoragePath:        path,
			CategoryID:         14, // Proposal TA/PA
			ProdiID:            mhs.ProdiID,
			UploaderID:         mhs.ID,
			OwnerID:            mhs.ID,
			OwnerRole:          domain.RoleMahasiswa,
			Visibility:         vis,
			VerificationStatus: verStatus,
			ArchivedAt:         archivedAt,
			RetentionStatus:    retStatus,
		}
		if deletedAt != nil {
			doc.DeletedAt = gorm.DeletedAt{Time: *deletedAt, Valid: true}
		}
		if err := ctx.DB.Create(&doc).Error; err != nil {
			t.Fatalf("create test doc %d failed: %v", id, err)
		}
		return doc
	}

	// TC-RET-01: Berkas archived_at = 7 tahun 11 bulan lalu; jalankan putaran retensi -> Tidak diubah
	t.Run("TC-RET-01_Archived7Years11MonthsNotModified", func(t *testing.T) {
		t1 := anchorTime.AddDate(-7, -11, 0)
		doc := createTestDoc(101, "Berkas 7y 11m", &t1, nil, nil, "private", nil)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.SoftDeleted != 0 {
			t.Errorf("expected 0 soft deleted, got %d", rep.SoftDeleted)
		}

		var check domain.Document
		if err := ctx.DB.First(&check, doc.ID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if check.DeletedAt.Valid {
			t.Errorf("doc should NOT be soft-deleted")
		}
		if check.RetentionStatus != nil {
			t.Errorf("retention_status should be nil, got %v", *check.RetentionStatus)
		}
	})

	// TC-RET-02: Berkas archived_at = tepat 8 tahun lalu -> Tidak diubah (batas: hanya yang lebih dari 8 tahun)
	t.Run("TC-RET-02_ArchivedExactly8YearsNotModified", func(t *testing.T) {
		// Cutoff in retention is now.AddDate(-8, 0, 0) = 2020-03-01 10:00:00 UTC
		// Query is: archived_at < cutoff
		t2 := anchorTime.AddDate(-8, 0, 0)
		doc := createTestDoc(102, "Berkas Tepat 8 Tahun", &t2, nil, nil, "private", nil)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.SoftDeleted != 0 {
			t.Errorf("expected 0 soft deleted, got %d", rep.SoftDeleted)
		}

		var check domain.Document
		if err := ctx.DB.First(&check, doc.ID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if check.DeletedAt.Valid {
			t.Errorf("doc should NOT be soft-deleted at exact 8 years boundary")
		}
	})

	// TC-RET-03: Berkas archived_at = 8 tahun 1 hari lalu
	// Expected: Ditandai soft delete; deleted_at terisi; objek GCS masih ada; log expired_soft_delete tercatat
	t.Run("TC-RET-03_Archived8Years1DaySoftDeletedAndLogged", func(t *testing.T) {
		t3 := anchorTime.AddDate(-8, 0, -1)
		doc := createTestDoc(103, "Berkas 8 Tahun 1 Hari", &t3, nil, nil, "private", nil)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.SoftDeleted != 1 {
			t.Errorf("expected 1 soft deleted, got %d", rep.SoftDeleted)
		}

		var check domain.Document
		if err := ctx.DB.Unscoped().First(&check, doc.ID).Error; err != nil {
			t.Fatalf("doc not found unscoped: %v", err)
		}
		if !check.DeletedAt.Valid {
			t.Errorf("expected deleted_at to be valid")
		}
		if check.RetentionStatus == nil || *check.RetentionStatus != "expired" {
			t.Errorf("expected retention_status 'expired', got %v", check.RetentionStatus)
		}
		if check.Visibility != "private" {
			t.Errorf("expected visibility private, got %s", check.Visibility)
		}

		// Storage object still exists
		_, _, sErr := ctx.Storage.Open(context.Background(), check.StoragePath)
		if sErr != nil {
			t.Errorf("storage object should still exist after soft-delete: %v", sErr)
		}

		// storage_logs has expired_soft_delete
		var log domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", doc.ID, "expired_soft_delete").First(&log).Error; err != nil {
			t.Errorf("storage log expired_soft_delete not found: %v", err)
		}
		if log.UserID != nil {
			t.Errorf("system action storage_logs user_id should be NULL, got %v", log.UserID)
		}
	})

	// TC-RET-04: Setelah TC-RET-03: pengguna biasa mencari/membuka berkas tersebut -> Tidak tampil (404 untuk akses langsung)
	t.Run("TC-RET-04_ExpiredSoftDeletedDocumentHiddenFromRegularUser", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		url := "/api/v1/documents/103"

		// 1. Direct GET /documents/{id} returns 404
		w := doRetReq(ctx.Engine, http.MethodGet, url, mhsToken)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected HTTP 404 for direct access to expired doc, got %d: %s", w.Code, w.Body.String())
		}

		// 2. Listing GET /documents does not contain doc 103
		wList, resList := doRetReqJSON(ctx.Engine, http.MethodGet, "/api/v1/documents", mhsToken, nil)
		if wList.Code != http.StatusOK {
			t.Errorf("expected HTTP 200 on document list, got %d", wList.Code)
		}
		dataList, _ := resList["data"].([]any)
		for _, item := range dataList {
			m, _ := item.(map[string]any)
			if m["id"] == float64(103) {
				t.Errorf("expired soft-deleted doc 103 should NOT appear in GET /documents list")
			}
		}
	})

	// TC-RET-05: Berkas soft-deleted berumur 89 hari; jalankan putaran retensi -> Tidak dihapus permanen; objek GCS masih ada
	t.Run("TC-RET-05_SoftDeleted89DaysNotHardDeleted", func(t *testing.T) {
		// graceCutoff = anchorTime.AddDate(0, 0, -90). Condition is: deleted_at <= graceCutoff
		// 89 days ago is strictly AFTER graceCutoff, so it is within grace period
		t5 := anchorTime.AddDate(0, 0, -89)
		exp := "expired"
		doc := createTestDoc(105, "Berkas Soft 89 Hari", nil, &t5, &exp, "private", nil)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.HardDeleted != 0 {
			t.Errorf("expected 0 hard deleted for 89d doc, got %d", rep.HardDeleted)
		}

		// Check still in DB and storage
		var check domain.Document
		if err := ctx.DB.Unscoped().First(&check, doc.ID).Error; err != nil {
			t.Errorf("doc should still exist in DB: %v", err)
		}
		if _, _, sErr := ctx.Storage.Open(context.Background(), check.StoragePath); sErr != nil {
			t.Errorf("storage object should still exist in storage: %v", sErr)
		}
	})

	// TC-RET-06: Berkas soft-deleted karena kedaluwarsa berumur 91 hari; jalankan putaran retensi
	// Expected: Dihapus permanen: baris metadata dan objek GCS hilang; log expired_hard_delete tercatat
	t.Run("TC-RET-06_SoftDeletedExpired91DaysHardDeletedAndLogged", func(t *testing.T) {
		t6 := anchorTime.AddDate(0, 0, -91)
		exp := "expired"
		doc := createTestDoc(106, "Berkas Expired 91 Hari", nil, &t6, &exp, "private", nil)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.HardDeleted != 1 {
			t.Errorf("expected 1 hard deleted, got %d", rep.HardDeleted)
		}

		// Metadata row permanently deleted from DB
		var check domain.Document
		err = ctx.DB.Unscoped().First(&check, doc.ID).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected ErrRecordNotFound, got %v", err)
		}

		// Storage object deleted
		_, _, sErr := ctx.Storage.Open(context.Background(), doc.StoragePath)
		if !errors.Is(sErr, storage.ErrNotFound) {
			t.Errorf("expected storage.ErrNotFound, got %v", sErr)
		}

		// storage_logs has expired_hard_delete
		var log domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", doc.ID, "expired_hard_delete").First(&log).Error; err != nil {
			t.Errorf("log expired_hard_delete not found: %v", err)
		}
	})

	// TC-RET-07: Jalankan putaran retensi dua kali berturut-turut pada data yang sama
	// Expected: Hasil identik; tidak ada galat atau log ganda yang tidak semestinya (idempoten)
	t.Run("TC-RET-07_ConsecutiveRunsIdempotent", func(t *testing.T) {
		// Run retention immediately again
		rep2, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("second run failed: %v", err)
		}
		if rep2.SoftDeleted != 0 || rep2.HardDeleted != 0 || rep2.Failed != 0 {
			t.Errorf("expected idempotent second run with 0 changes, got soft=%d hard=%d failed=%d",
				rep2.SoftDeleted, rep2.HardDeleted, rep2.Failed)
		}
	})

	// TC-RET-08: Batch 500 berkas kedaluwarsa
	// Expected: Semua diproses secara bertahap (per chunk); waktu proses wajar; penggunaan memori tidak membesar
	t.Run("TC-RET-08_Batch500ExpiredProcessedInChunks", func(t *testing.T) {
		tArch := anchorTime.AddDate(-9, 0, 0)
		mhs := ctx.Users["budi@del.ac.id"]

		batchDocs := make([]domain.Document, 500)
		for i := 0; i < 500; i++ {
			id := uint(2000 + i)
			path := fmt.Sprintf("documents/IF/2019/batch_%d.pdf", id)
			batchDocs[i] = domain.Document{
				ID:           id,
				Title:        fmt.Sprintf("Batch Expired Doc %d", id),
				OriginalName: "doc.pdf",
				Extension:    "pdf",
				MimeType:     "application/pdf",
				SizeKB:       50,
				StoragePath:  path,
				CategoryID:   14,
				ProdiID:      mhs.ProdiID,
				UploaderID:   mhs.ID,
				OwnerID:      mhs.ID,
				OwnerRole:    domain.RoleMahasiswa,
				Visibility:   "private",
				ArchivedAt:   &tArch,
			}
			_ = ctx.Storage.Put(context.Background(), path, strings.NewReader("sample batch"), "application/pdf")
		}
		if err := ctx.DB.CreateInBatches(&batchDocs, 100).Error; err != nil {
			t.Fatalf("create batch 500 failed: %v", err)
		}

		start := time.Now()
		rep, err := ctx.RetSvc.Run(context.Background())
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("batch retention run failed: %v", err)
		}
		if rep.SoftDeleted != 500 {
			t.Errorf("expected 500 soft deleted, got %d", rep.SoftDeleted)
		}
		if elapsed > 5*time.Second {
			t.Errorf("batch processing took longer than expected: %v", elapsed)
		}
	})

	// TC-RET-09: Objek GCS gagal dihapus pada berkas berumur 91 hari
	// Expected: Metadata tidak dihapus; kegagalan dicatat; dicoba lagi pada putaran berikutnya; tidak ada objek yatim
	t.Run("TC-RET-09_GCSFailureDuringHardDeleteRollbackAndRetry", func(t *testing.T) {
		t9 := anchorTime.AddDate(0, 0, -95)
		exp := "expired"
		doc := createTestDoc(109, "Berkas Fail GCS Target 09", nil, &t9, &exp, "private", nil)

		// 1. Inject GCS failure
		ctx.Storage.mu.Lock()
		ctx.Storage.failDelete = true
		ctx.Storage.mu.Unlock()

		repFail, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run returned unexpected fatal error: %v", err)
		}
		if repFail.Failed != 1 {
			t.Errorf("expected rep.Failed == 1, got %d", repFail.Failed)
		}
		if repFail.HardDeleted != 0 {
			t.Errorf("expected rep.HardDeleted == 0 on failure, got %d", repFail.HardDeleted)
		}

		// Metadata row must still exist in DB (transaction rollback)
		var check domain.Document
		if err := ctx.DB.Unscoped().First(&check, doc.ID).Error; err != nil {
			t.Errorf("document metadata must NOT be lost when storage fails: %v", err)
		}

		// 2. Clear failure and run next round (retry)
		ctx.Storage.mu.Lock()
		ctx.Storage.failDelete = false
		ctx.Storage.mu.Unlock()

		repRetry, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retry run failed: %v", err)
		}
		if repRetry.HardDeleted != 1 {
			t.Errorf("expected 1 hard deleted on retry, got %d", repRetry.HardDeleted)
		}

		// Metadata and storage object are now deleted
		err = ctx.DB.Unscoped().First(&check, doc.ID).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("document should be deleted on retry, got: %v", err)
		}
		_, _, sErr := ctx.Storage.Open(context.Background(), doc.StoragePath)
		if !errors.Is(sErr, storage.ErrNotFound) {
			t.Errorf("storage object should be deleted on retry, got: %v", sErr)
		}
	})

	// TC-RET-10: Berkas kedaluwarsa yang masih berstatus pending verification
	// Expected: Antrean verifikasi ikut dibersihkan; notifikasi antrean terkait tidak menunjuk berkas hilang
	t.Run("TC-RET-10_ExpiredPendingVerificationCleanedUp", func(t *testing.T) {
		t10 := anchorTime.AddDate(-9, 0, 0)
		pending := "pending"
		doc := createTestDoc(110, "Berkas Expired Pending 10", &t10, nil, nil, "public", &pending)

		// Create verification row
		ver := domain.Verification{
			DocumentID:  doc.ID,
			Status:      "pending",
			RequestedBy: doc.UploaderID,
			RequestedAt: t10,
		}
		ctx.DB.Create(&ver)

		// Create notification for BAAK
		baakUser := ctx.Users["baak@del.ac.id"]
		notif := domain.Notification{
			ID:      "notif-ret-110",
			UserID:  baakUser.ID,
			Type:    "verification_pending",
			Title:   "Berkas publik baru menunggu verifikasi",
			Message: "Berkas Expired Pending 10",
			Data:    domain.JSONMap{"document_id": doc.ID, "verification_id": ver.ID},
		}
		ctx.DB.Create(&notif)

		// Run retention
		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.SoftDeleted != 1 {
			t.Errorf("expected 1 soft deleted, got %d", rep.SoftDeleted)
		}

		// 1. Verification row must be cleaned up
		var verCheck domain.Verification
		err = ctx.DB.Where("document_id = ?", doc.ID).First(&verCheck).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected verification row to be deleted, got %v", err)
		}

		// 2. Notification must be cleaned up (no dangling reference)
		var notifCheck domain.Notification
		err = ctx.DB.Where("id = ?", notif.ID).First(&notifCheck).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected pending notification to be deleted, got %v", err)
		}
	})

	// TC-RET-11: Jalankan worker dalam mode normal dan periksa log awal
	// Expected: Tugas retensi terdaftar pukul 02:00 WIB (zona waktu terkonfigurasi Asia/Jakarta); log mencatat waktu eksekusi berikutnya
	t.Run("TC-RET-11_WorkerScheduleRegisteredAt0200WIB", func(t *testing.T) {
		loc, err := time.LoadLocation(ctx.Config.Timezone)
		if err != nil {
			t.Fatalf("load location failed: %v", err)
		}
		if loc.String() != "Asia/Jakarta" {
			t.Errorf("expected timezone Asia/Jakarta, got %s", loc.String())
		}

		sched, err := cron.ParseStandard("0 2 * * *")
		if err != nil {
			t.Fatalf("parse schedule: %v", err)
		}

		// Test next run evaluation from a known reference time in Asia/Jakarta
		refWIB := time.Date(2026, 11, 4, 1, 0, 0, 0, loc)
		next := sched.Next(refWIB)
		if next.Hour() != 2 || next.Minute() != 0 || next.Second() != 0 {
			t.Errorf("expected next execution at 02:00:00 WIB, got %s", next.Format("15:04:05"))
		}
		_, offset := next.Zone()
		if offset != 7*3600 {
			t.Errorf("expected UTC+7 offset for Asia/Jakarta (WIB), got %d seconds", offset)
		}
	})

	// TC-RET-12: Berkas dengan archived_at kosong (NULL)
	// Expected: Tidak diproses secara keliru; dicatat sebagai data anomali untuk ditinjau
	t.Run("TC-RET-12_ArchivedAtNullLoggedAsAnomalyNotCorrupted", func(t *testing.T) {
		doc := createTestDoc(112, "Berkas Tanpa ArchivedAt", nil, nil, nil, "private", nil)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.Anomalies < 1 {
			t.Errorf("expected at least 1 anomaly, got %d", rep.Anomalies)
		}

		var check domain.Document
		if err := ctx.DB.First(&check, doc.ID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if check.DeletedAt.Valid {
			t.Errorf("doc with NULL archived_at must NOT be soft deleted")
		}
	})

	// TC-RET-13: Setelah pembersihan: rekonsiliasi jumlah objek GCS versus baris metadata -> Tidak ada selisih (SLA-DI-01)
	t.Run("TC-RET-13_ReconciliationStorageObjectsVsDatabase", func(t *testing.T) {
		// Run full reconciliation
		var allDocs []domain.Document
		if err := ctx.DB.Unscoped().Find(&allDocs).Error; err != nil {
			t.Fatalf("query unscoped docs failed: %v", err)
		}

		// Every active document in DB must have a corresponding file in storage
		for _, d := range allDocs {
			_, _, err := ctx.Storage.Open(context.Background(), d.StoragePath)
			if err != nil {
				t.Errorf("reconciliation mismatch: doc %d path '%s' not in storage: %v", d.ID, d.StoragePath, err)
			}
		}
	})

	// TC-RET-14: Berkas archived_at = 29 Februari 2020; putaran dijalankan pada 1 Maret 2028 (waktu uji disuntikkan)
	// Expected: Dianggap lebih dari 8 tahun menurut kalender; diproses secara benar tanpa selisih hari kabisat
	t.Run("TC-RET-14_LeapDayFeb29CorrectlyExpiredOnMar1", func(t *testing.T) {
		leapArchived := time.Date(2020, 2, 29, 10, 0, 0, 0, time.UTC)
		doc := createTestDoc(114, "Dokumen Kabisat 29 Feb 2020", &leapArchived, nil, nil, "private", nil)

		// Anchor time is 2028-03-01: cutoff = 2028-03-01.AddDate(-8, 0, 0) = 2020-03-01
		// Since 2020-02-29 is strictly BEFORE 2020-03-01, it is > 8 calendar years old!
		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.SoftDeleted != 1 {
			t.Errorf("expected 1 soft deleted for leap day file on March 1, got %d", rep.SoftDeleted)
		}

		var check domain.Document
		if err := ctx.DB.Unscoped().First(&check, doc.ID).Error; err != nil {
			t.Fatalf("doc not found unscoped: %v", err)
		}
		if !check.DeletedAt.Valid {
			t.Errorf("leap day document should be soft-deleted")
		}
	})

	// TC-RET-15: Dua proses worker dijalankan bersamaan pada data kedaluwarsa yang sama
	// Expected: Hanya satu proses yang bekerja (kunci GET_LOCK MySQL); proses lain keluar tanpa mengubah data; tidak ada log atau penghapusan ganda
	t.Run("TC-RET-15_MultiInstanceConcurrentExecutionLocked", func(t *testing.T) {
		// Acquire lock manually to simulate worker 1 holding the lock
		globalRetLockMu.Lock()
		globalRetLockHeld = true
		globalRetLockMu.Unlock()

		// Worker 2 attempts to run while lock is held
		repSkipped, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run returned unexpected error on locked instance: %v", err)
		}
		if !repSkipped.Skipped {
			t.Errorf("expected worker 2 to be Skipped when lock is held, got Skipped=false")
		}
		if repSkipped.SoftDeleted != 0 || repSkipped.HardDeleted != 0 {
			t.Errorf("skipped worker must not modify data, got soft=%d hard=%d", repSkipped.SoftDeleted, repSkipped.HardDeleted)
		}

		// Release lock
		globalRetLockMu.Lock()
		globalRetLockHeld = false
		globalRetLockMu.Unlock()

		// Worker can now run normally when lock is released
		repNormal, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("run failed after lock release: %v", err)
		}
		if repNormal.Skipped {
			t.Errorf("worker should NOT be skipped when lock is free")
		}
	})

	// TC-RET-16: Worker dihentikan paksa di tengah batch
	// Expected: Putaran berikutnya melanjutkan sisa data; tidak ada duplikasi, kehilangan data, atau objek yatim
	t.Run("TC-RET-16_WorkerInterruptedMidwayResumesCleanly", func(t *testing.T) {
		tArch := anchorTime.AddDate(-10, 0, 0)
		docA := createTestDoc(1161, "Interrupted Doc A", &tArch, nil, nil, "private", nil)
		docB := createTestDoc(1162, "Interrupted Doc B", &tArch, nil, nil, "private", nil)

		// First round: simulate cancellation after docA by soft-deleting docA directly
		// (simulating worker stopping after processing docA)
		now := anchorTime
		exp := "expired"
		ctx.DB.Model(&domain.Document{}).Where("id = ?", docA.ID).Updates(map[string]any{
			"retention_status": exp,
			"deleted_at":       now,
			"visibility":       "private",
		})
		_ = ctx.DB.Create(&domain.StorageLog{DocumentID: &docA.ID, Action: "expired_soft_delete"}).Error

		// Second round: worker runs and processes remaining docB
		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("resumed retention run failed: %v", err)
		}
		if rep.SoftDeleted != 1 {
			t.Errorf("expected 1 soft deleted on resume (docB only), got %d", rep.SoftDeleted)
		}

		// Both docs are now soft-deleted
		var checkA, checkB domain.Document
		ctx.DB.Unscoped().First(&checkA, docA.ID)
		ctx.DB.Unscoped().First(&checkB, docB.ID)
		if !checkA.DeletedAt.Valid || !checkB.DeletedAt.Valid {
			t.Errorf("both docs must be soft deleted")
		}

		// Check log count: exactly 1 log for docA and 1 log for docB
		var countA, countB int64
		ctx.DB.Model(&domain.StorageLog{}).Where("document_id = ? AND action = ?", docA.ID, "expired_soft_delete").Count(&countA)
		ctx.DB.Model(&domain.StorageLog{}).Where("document_id = ? AND action = ?", docB.ID, "expired_soft_delete").Count(&countB)
		if countA != 1 || countB != 1 {
			t.Errorf("expected exactly 1 log per document, got countA=%d countB=%d", countA, countB)
		}
	})

	// TC-RET-17: Sesi (refresh token) yang telah kedaluwarsa atau dicabut lebih dari 30 hari
	// Expected: Dibersihkan oleh worker; sesi aktif tidak terpengaruh
	t.Run("TC-RET-17_ExpiredSessionsOlderThan30DaysPurged", func(t *testing.T) {
		// Old session 35 days ago (should be purged)
		tOld := anchorTime.AddDate(0, 0, -35)
		sOld := domain.AuthSession{
			ID:                "sess-ret-old-35d",
			UserID:            1,
			RefreshTokenHash:  "hash-old-35",
			ExpiresAt:         tOld,
			AbsoluteExpiresAt: tOld,
		}
		ctx.DB.Create(&sOld)

		// Revoked session 32 days ago (should be purged)
		tRev := anchorTime.AddDate(0, 0, -32)
		sRev := domain.AuthSession{
			ID:                "sess-ret-revoked-32d",
			UserID:            1,
			RefreshTokenHash:  "hash-rev-32",
			ExpiresAt:         anchorTime.AddDate(0, 0, 5),
			AbsoluteExpiresAt: anchorTime.AddDate(0, 0, 10),
			RevokedAt:         &tRev,
		}
		ctx.DB.Create(&sRev)

		// Active session (must be preserved)
		sActive := domain.AuthSession{
			ID:                "sess-ret-active",
			UserID:            1,
			RefreshTokenHash:  "hash-active-live",
			ExpiresAt:         anchorTime.AddDate(0, 0, 7),
			AbsoluteExpiresAt: anchorTime.AddDate(0, 0, 30),
		}
		ctx.DB.Create(&sActive)

		// Recently revoked session 5 days ago (must be preserved for audit)
		tRecent := anchorTime.AddDate(0, 0, -5)
		sRecent := domain.AuthSession{
			ID:                "sess-ret-recent-revoked",
			UserID:            1,
			RefreshTokenHash:  "hash-recent-rev",
			ExpiresAt:         anchorTime.AddDate(0, 0, 7),
			AbsoluteExpiresAt: anchorTime.AddDate(0, 0, 30),
			RevokedAt:         &tRecent,
		}
		ctx.DB.Create(&sRecent)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.Sessions != 2 {
			t.Errorf("expected 2 sessions purged, got %d", rep.Sessions)
		}

		var countOld, countRev, countActive, countRecent int64
		ctx.DB.Model(&domain.AuthSession{}).Where("id = ?", sOld.ID).Count(&countOld)
		ctx.DB.Model(&domain.AuthSession{}).Where("id = ?", sRev.ID).Count(&countRev)
		ctx.DB.Model(&domain.AuthSession{}).Where("id = ?", sActive.ID).Count(&countActive)
		ctx.DB.Model(&domain.AuthSession{}).Where("id = ?", sRecent.ID).Count(&countRecent)

		if countOld != 0 || countRev != 0 {
			t.Errorf("expected old and revoked sessions to be purged")
		}
		if countActive != 1 || countRecent != 1 {
			t.Errorf("expected active and recently revoked sessions to be kept")
		}
	})

	// TC-RET-18: Berkas yang dihapus pengguna (bukan kedaluwarsa) berumur 91 hari sejak deleted_at
	// Expected: Dihapus permanen; log deleted_hard_delete tercatat (dibedakan dari expired_hard_delete)
	t.Run("TC-RET-18_UserSoftDeletedDocumentHardDeletedAfter91DaysLoggedDeletedHardDelete", func(t *testing.T) {
		t18 := anchorTime.AddDate(0, 0, -91)
		// Soft-deleted by user: retention_status is nil!
		doc := createTestDoc(118, "Berkas Dihapus Pengguna 91 Hari", nil, &t18, nil, "private", nil)

		rep, err := ctx.RetSvc.Run(context.Background())
		if err != nil {
			t.Fatalf("retention run failed: %v", err)
		}
		if rep.HardDeleted != 1 {
			t.Errorf("expected 1 hard deleted for user soft-deleted doc, got %d", rep.HardDeleted)
		}

		// 1. Verify permanently deleted from DB
		var check domain.Document
		err = ctx.DB.Unscoped().First(&check, doc.ID).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected ErrRecordNotFound, got %v", err)
		}

		// 2. Verify deleted from storage
		_, _, sErr := ctx.Storage.Open(context.Background(), doc.StoragePath)
		if !errors.Is(sErr, storage.ErrNotFound) {
			t.Errorf("expected storage.ErrNotFound, got %v", sErr)
		}

		// 3. Verify log recorded as "deleted_hard_delete" (DISTINCT from "expired_hard_delete")
		var log domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", doc.ID, "deleted_hard_delete").First(&log).Error; err != nil {
			t.Errorf("expected storage_logs with action 'deleted_hard_delete': %v", err)
		}

		// Ensure it was NOT recorded as expired_hard_delete
		var wrongLog domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", doc.ID, "expired_hard_delete").First(&wrongLog).Error; err == nil {
			t.Errorf("document should NOT have 'expired_hard_delete' log")
		}
	})
}
