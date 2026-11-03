package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var delSuiteSeq uint64

type delStorageWrapper struct {
	*storage.MemoryStorage
	mu         sync.Mutex
	failDelete bool
}

func (s *delStorageWrapper) Delete(ctx context.Context, path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failDelete {
		return errors.New("simulated GCS failure during hard delete")
	}
	return s.MemoryStorage.Delete(ctx, path)
}

type delSuiteContext struct {
	Engine  *gin.Engine
	DB      *gorm.DB
	Config  config.Config
	Storage *delStorageWrapper
	Users   map[string]domain.User
	Tokens  map[string]string
}

func setupDeleteSuite(t *testing.T) *delSuiteContext {
	t.Helper()
	gin.SetMode(gin.TestMode)
	httpx.Init()

	seq := atomic.AddUint64(&delSuiteSeq, 1)
	dsn := fmt.Sprintf("file:del_suite_%d_%d?mode=memory&cache=shared", time.Now().UnixNano(), seq)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

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
		t.Fatalf("migrate delete suite: %v", err)
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

	// 3. Categories (allowed_roles JSON and is_operational flag)
	cats := []domain.Category{
		{ID: 1, Name: "SK Keputusan Dekan", AllowedRoles: domain.StringList{"dekan", "super_admin"}, IsOperational: false},
		{ID: 2, Name: "Dokumen Akreditasi Fakultas", AllowedRoles: domain.StringList{"dekan", "super_admin"}, IsOperational: false},
		{ID: 3, Name: "Arsip Khusus", AllowedRoles: domain.StringList{"dekan", "super_admin"}, IsOperational: false},
		{ID: 4, Name: "Legalisir", AllowedRoles: domain.StringList{"baak", "super_admin"}, IsOperational: true},
		{ID: 5, Name: "Transkrip Nilai Akademik", AllowedRoles: domain.StringList{"baak", "super_admin"}, IsOperational: true},
		{ID: 6, Name: "Dokumen BAAK", AllowedRoles: domain.StringList{"baak", "super_admin"}, IsOperational: true},
		{ID: 7, Name: "Berkas Mahasiswa/Dosen", AllowedRoles: domain.StringList{"baak", "super_admin"}, IsOperational: true},
		{ID: 8, Name: "SK Keputusan Prodi", AllowedRoles: domain.StringList{"kaprodi", "super_admin"}, IsOperational: false},
		{ID: 9, Name: "Dokumen Akreditasi Prodi", AllowedRoles: domain.StringList{"kaprodi", "super_admin"}, IsOperational: false},
		{ID: 10, Name: "Kurikulum", AllowedRoles: domain.StringList{"kaprodi", "super_admin"}, IsOperational: false},
		{ID: 11, Name: "Rencana Pembelajaran Semester (RPS)", AllowedRoles: domain.StringList{"dosen", "super_admin"}, IsOperational: false},
		{ID: 12, Name: "SK Mengajar", AllowedRoles: domain.StringList{"dosen", "super_admin"}, IsOperational: false},
		{ID: 13, Name: "Berita Acara Perkuliahan (BAP)", AllowedRoles: domain.StringList{"dosen", "super_admin"}, IsOperational: false},
		{ID: 14, Name: "Proposal TA/PA", AllowedRoles: domain.StringList{"mahasiswa", "super_admin"}, IsOperational: false},
		{ID: 15, Name: "Berkas Yudisium", AllowedRoles: domain.StringList{"mahasiswa", "super_admin"}, IsOperational: false},
		{ID: 16, Name: "Surat Keterangan", AllowedRoles: domain.StringList{"mahasiswa", "super_admin"}, IsOperational: false},
	}
	db.Create(&cats)

	// 4. Users
	const defaultPW = "Password123!"
	pwHash, _ := bcrypt.GenerateFromPassword([]byte(defaultPW), 12)
	hashStr := string(pwHash)

	prodiIF := uint(1)
	prodiSI := uint(2)
	strPtr := func(s string) *string { return &s }

	userList := []domain.User{
		{ID: 1, Name: "Super Admin", Email: "admin@del.ac.id", PasswordHash: hashStr, RoleID: 1, IsActive: true},
		{ID: 2, Name: "Dekan FITE", Email: "dekan@del.ac.id", PasswordHash: hashStr, RoleID: 2, IsActive: true},
		{ID: 3, Name: "Staf BAAK", Email: "baak@del.ac.id", PasswordHash: hashStr, RoleID: 3, IsActive: true},
		{ID: 4, Name: "Kaprodi Informatika", Email: "kaprodi.if@del.ac.id", PasswordHash: hashStr, RoleID: 4, ProdiID: &prodiIF, NIP: strPtr("198001012005011001"), IsActive: true},
		{ID: 5, Name: "Kaprodi Sistem Informasi", Email: "kaprodi.si@del.ac.id", PasswordHash: hashStr, RoleID: 4, ProdiID: &prodiSI, NIP: strPtr("198202022008011003"), IsActive: true},
		{ID: 6, Name: "Dosen Informatika", Email: "dosen.if@del.ac.id", PasswordHash: hashStr, RoleID: 5, ProdiID: &prodiIF, NIP: strPtr("198505052010011002"), IsActive: true},
		{ID: 7, Name: "Dosen Sistem Informasi", Email: "dosen.si@del.ac.id", PasswordHash: hashStr, RoleID: 5, ProdiID: &prodiSI, NIP: strPtr("198707072012011004"), IsActive: true},
		{ID: 8, Name: "Budi Mahasiswa (IF-1)", Email: "budi@del.ac.id", PasswordHash: hashStr, RoleID: 6, ProdiID: &prodiIF, NIM: strPtr("11S20001"), IsActive: true},
		{ID: 9, Name: "Mahasiswa Informatika (IF-2)", Email: "mhs.if2@del.ac.id", PasswordHash: hashStr, RoleID: 6, ProdiID: &prodiIF, NIM: strPtr("11S20002"), IsActive: true},
		{ID: 10, Name: "Mahasiswa Sistem Informasi (SI-1)", Email: "iss24034@del.ac.id", PasswordHash: hashStr, RoleID: 6, ProdiID: &prodiSI, NIM: strPtr("iss24034"), IsActive: true},
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
	}

	store := &delStorageWrapper{MemoryStorage: storage.NewMemoryStorage()}
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

	r := router.New(cfg, authSvc, h, nil)

	// Generate tokens
	tokens := make(map[string]string)
	for _, u := range userList {
		tp, err := authSvc.Login(context.Background(), u.Email, defaultPW, "del-test-agent", "192.0.2.1")
		if err != nil {
			t.Fatalf("login seed user %s: %v", u.Email, err)
		}
		tokens[u.Email] = tp.AccessToken
	}

	return &delSuiteContext{
		Engine:  r,
		DB:      db,
		Config:  cfg,
		Storage: store,
		Users:   userMap,
		Tokens:  tokens,
	}
}

func doDelReq(engine *gin.Engine, method, url, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func doDelReqJSON(engine *gin.Engine, method, url, token string, body any) (*httptest.ResponseRecorder, map[string]any) {
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

// uploadDocForm performs a standard multipart upload and returns document ID.
// For BAAK or Super Admin, prodiID is mandatory unless specified.
func uploadDocForm(t *testing.T, ctx *delSuiteContext, token, title, categoryID, visibility, prodiID string) uint {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", title)
	_ = writer.WriteField("category_id", categoryID)
	if visibility != "" {
		_ = writer.WriteField("visibility", visibility)
	}
	if prodiID != "" {
		_ = writer.WriteField("prodi_id", prodiID)
	}

	part, err := writer.CreateFormFile("file", "del_test_file.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("%PDF-1.4 test deletion content\n%%EOF"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	w := httptest.NewRecorder()
	ctx.Engine.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("uploadDocForm failed status %d: %s", w.Code, w.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("unmarshal upload response: %v", err)
	}
	data, ok := res["data"].(map[string]any)
	if !ok {
		t.Fatalf("no data object in response: %v", res)
	}
	idFloat, ok := data["id"].(float64)
	if !ok {
		t.Fatalf("no id in response data: %v", data)
	}
	return uint(idFloat)
}

func TestTS_DEL_Granular(t *testing.T) {
	ctx := setupDeleteSuite(t)

	// TC-DEL-01: MHS-IF-1 DELETE /documents/{id} berkas miliknya
	// Expected: 204; berkas tidak tampil lagi; log storage_logs mencatat penghapusan
	t.Run("TC-DEL-01_MahasiswaDeleteOwnDocumentSoftDeleteSuccess204", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		mhsUser := ctx.Users["budi@del.ac.id"]
		docID := uploadDocForm(t, ctx, mhsToken, "Proposal TA Budi Soft Delete 01", "14", "private", "")

		// 1. Send DELETE
		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, mhsToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204, got %d: %s", w.Code, w.Body.String())
		}

		// 2. Verify soft-deleted in DB: deleted_at is not null
		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("document not found in unscoped query: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("expected DeletedAt to be set, but it was NULL")
		}

		// 3. Verify normal GET /documents/{id} returns 404
		wGet := doDelReq(ctx.Engine, http.MethodGet, url, mhsToken)
		if wGet.Code != http.StatusNotFound {
			t.Errorf("expected HTTP 404 on deleted document, got %d", wGet.Code)
		}

		// 4. Verify storage_logs records soft_delete
		var log domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", docID, "soft_delete").First(&log).Error; err != nil {
			t.Errorf("expected storage_logs record for soft_delete: %v", err)
		}
		if log.UserID == nil || *log.UserID != mhsUser.ID {
			t.Errorf("expected storage_logs user_id to be %d, got %v", mhsUser.ID, log.UserID)
		}

		// 5. Verify physical storage file is still intact (soft delete does not delete file)
		_, _, err := ctx.Storage.Open(context.Background(), doc.StoragePath)
		if err != nil {
			t.Errorf("storage object should still exist after soft delete, got err: %v", err)
		}
	})

	// TC-DEL-02: MHS-IF-1 menghapus berkas milik MHS-IF-2
	// Expected: 403; berkas utuh
	t.Run("TC-DEL-02_MahasiswaDeleteOtherMahasiswaDocForbidden403", func(t *testing.T) {
		mhs2Token := ctx.Tokens["mhs.if2@del.ac.id"]
		mhs1Token := ctx.Tokens["budi@del.ac.id"]
		docID := uploadDocForm(t, ctx, mhs2Token, "Proposal Milik MHS IF 2", "14", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, mhs1Token)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected HTTP 403, got %d: %s", w.Code, w.Body.String())
		}

		// Verify document remains intact
		var doc domain.Document
		if err := ctx.DB.First(&doc, docID).Error; err != nil {
			t.Fatalf("expected document to remain intact, got err: %v", err)
		}
		if doc.DeletedAt.Valid {
			t.Errorf("document should not be deleted")
		}
	})

	// TC-DEL-03: MHS menghapus berkas milik dosen
	// Expected: 403
	t.Run("TC-DEL-03_MahasiswaDeleteDosenDocForbidden403", func(t *testing.T) {
		dsnToken := ctx.Tokens["dosen.if@del.ac.id"]
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		docID := uploadDocForm(t, ctx, dsnToken, "RPS Dosen IF Target 03", "11", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, mhsToken)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected HTTP 403, got %d: %s", w.Code, w.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.First(&doc, docID).Error; err != nil {
			t.Fatalf("expected document to remain intact: %v", err)
		}
		if doc.DeletedAt.Valid {
			t.Errorf("document should not be deleted")
		}
	})

	// TC-DEL-04: DSN-IF menghapus berkas miliknya
	// Expected: 204
	t.Run("TC-DEL-04_DosenDeleteOwnDocumentSuccess204", func(t *testing.T) {
		dsnToken := ctx.Tokens["dosen.if@del.ac.id"]
		docID := uploadDocForm(t, ctx, dsnToken, "RPS Dosen IF Milik Sendiri 04", "11", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, dsnToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204, got %d: %s", w.Code, w.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("expected document to be soft-deleted")
		}
	})

	// TC-DEL-05: DSN-IF menghapus berkas mahasiswa prodi IF (bisa dibaca, bukan miliknya)
	// Expected: 403
	t.Run("TC-DEL-05_DosenDeleteMahasiswaInSameProdiForbidden403", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		dsnToken := ctx.Tokens["dosen.if@del.ac.id"]
		docID := uploadDocForm(t, ctx, mhsToken, "Berkas Budi Mahasiswa IF 05", "14", "private", "")

		// Verify DSN-IF CAN view it
		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		wGet := doDelReq(ctx.Engine, http.MethodGet, url, dsnToken)
		if wGet.Code != http.StatusOK {
			t.Fatalf("precondition failed: DSN-IF should be able to view MHS-IF doc, got %d", wGet.Code)
		}

		// DSN-IF tries to delete it
		wDel := doDelReq(ctx.Engine, http.MethodDelete, url, dsnToken)
		if wDel.Code != http.StatusForbidden {
			t.Fatalf("expected HTTP 403, got %d: %s", wDel.Code, wDel.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.First(&doc, docID).Error; err != nil {
			t.Fatalf("expected doc to remain intact: %v", err)
		}
		if doc.DeletedAt.Valid {
			t.Errorf("doc should not be deleted")
		}
	})

	// TC-DEL-06: KPR-IF menghapus berkas miliknya
	// Expected: 204
	t.Run("TC-DEL-06_KaprodiDeleteOwnDocumentSuccess204", func(t *testing.T) {
		kprToken := ctx.Tokens["kaprodi.if@del.ac.id"]
		docID := uploadDocForm(t, ctx, kprToken, "Kurikulum IF 2026", "10", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, kprToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204, got %d: %s", w.Code, w.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("expected document to be soft-deleted")
		}
	})

	// TC-DEL-07: KPR-IF menghapus berkas dosen atau mahasiswa
	// Expected: 403
	t.Run("TC-DEL-07_KaprodiDeleteDosenOrMahasiswaDocForbidden403", func(t *testing.T) {
		dsnToken := ctx.Tokens["dosen.if@del.ac.id"]
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		kprToken := ctx.Tokens["kaprodi.if@del.ac.id"]

		docDsnID := uploadDocForm(t, ctx, dsnToken, "RPS Dosen Untuk KPR Test", "11", "private", "")
		docMhsID := uploadDocForm(t, ctx, mhsToken, "Proposal MHS Untuk KPR Test", "14", "private", "")

		// Try deleting dosen file
		w1 := doDelReq(ctx.Engine, http.MethodDelete, fmt.Sprintf("/api/v1/documents/%d", docDsnID), kprToken)
		if w1.Code != http.StatusForbidden {
			t.Errorf("expected HTTP 403 on KPR deleting DSN doc, got %d", w1.Code)
		}

		// Try deleting mahasiswa file
		w2 := doDelReq(ctx.Engine, http.MethodDelete, fmt.Sprintf("/api/v1/documents/%d", docMhsID), kprToken)
		if w2.Code != http.StatusForbidden {
			t.Errorf("expected HTTP 403 on KPR deleting MHS doc, got %d", w2.Code)
		}
	})

	// TC-DEL-08: DKN menghapus berkas yang diunggahnya sendiri
	// Expected: 204
	t.Run("TC-DEL-08_DekanDeleteOwnDocumentSuccess204", func(t *testing.T) {
		dknToken := ctx.Tokens["dekan@del.ac.id"]
		docID := uploadDocForm(t, ctx, dknToken, "SK Keputusan Dekan 08", "1", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, dknToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204, got %d: %s", w.Code, w.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("expected document to be soft-deleted")
		}
	})

	// TC-DEL-09: DKN menghapus berkas milik aktor lain (walau hak baca semua)
	// Expected: 403
	t.Run("TC-DEL-09_DekanDeleteOtherActorDocForbidden403", func(t *testing.T) {
		dknToken := ctx.Tokens["dekan@del.ac.id"]
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		docID := uploadDocForm(t, ctx, mhsToken, "Proposal Mahasiswa Dekan View Only", "14", "private", "")

		// Verify Dekan CAN view it
		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		wGet := doDelReq(ctx.Engine, http.MethodGet, url, dknToken)
		if wGet.Code != http.StatusOK {
			t.Fatalf("precondition failed: Dekan should view all faculty docs, got %d", wGet.Code)
		}

		// Dekan tries to delete it
		wDel := doDelReq(ctx.Engine, http.MethodDelete, url, dknToken)
		if wDel.Code != http.StatusForbidden {
			t.Fatalf("expected HTTP 403, got %d: %s", wDel.Code, wDel.Body.String())
		}
	})

	// TC-DEL-10: BAK menghapus berkas miliknya
	// Expected: 204
	t.Run("TC-DEL-10_BAAKDeleteOwnDocumentSuccess204", func(t *testing.T) {
		bakToken := ctx.Tokens["baak@del.ac.id"]
		// BAAK requires prodi_id on upload
		docID := uploadDocForm(t, ctx, bakToken, "Dokumen BAAK Milik BAAK 10", "6", "private", "1")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, bakToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204, got %d: %s", w.Code, w.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("expected document to be soft-deleted")
		}
	})

	// TC-DEL-11: BAK menghapus berkas operasional prodi (Legalisir, Transkrip, Dokumen BAAK, Berkas Mahasiswa/Dosen)
	// Expected: 204
	t.Run("TC-DEL-11_BAAKDeleteOperationalCategoryDocSuccess204", func(t *testing.T) {
		bakToken := ctx.Tokens["baak@del.ac.id"]
		bakUser := ctx.Users["baak@del.ac.id"]

		// Category 4 = Legalisir (IsOperational: true)
		// Uploaded by SA (another user), requires prodi_id = "1"
		saToken := ctx.Tokens["admin@del.ac.id"]
		docID := uploadDocForm(t, ctx, saToken, "Berkas Legalisir Operasional 11", "4", "private", "1")

		// BAAK deletes it
		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, bakToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204 for BAAK deleting operational category file, got %d: %s", w.Code, w.Body.String())
		}

		// Verify soft deleted and storage log has BAAK user ID
		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("expected document to be soft-deleted")
		}

		var log domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", docID, "soft_delete").First(&log).Error; err != nil {
			t.Fatalf("storage_logs not recorded: %v", err)
		}
		if log.UserID == nil || *log.UserID != bakUser.ID {
			t.Errorf("expected log user_id to be %d, got %v", bakUser.ID, log.UserID)
		}
	})

	// TC-DEL-12: BAK menghapus berkas pribadi milik Dekan
	// Expected: 403
	t.Run("TC-DEL-12_BAAKDeleteDekanPersonalDocForbidden403", func(t *testing.T) {
		dknToken := ctx.Tokens["dekan@del.ac.id"]
		bakToken := ctx.Tokens["baak@del.ac.id"]

		// Category 1 = SK Keputusan Dekan (IsOperational: false)
		docID := uploadDocForm(t, ctx, dknToken, "SK Keputusan Dekan Rahasia 12", "1", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, bakToken)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected HTTP 403 for BAAK deleting non-operational file of Dekan, got %d: %s", w.Code, w.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.First(&doc, docID).Error; err != nil {
			t.Fatalf("expected doc to remain intact: %v", err)
		}
		if doc.DeletedAt.Valid {
			t.Errorf("document should not be deleted")
		}
	})

	// TC-DEL-13: SA menghapus berkas milik aktor mana pun (soft delete)
	// Expected: 204
	t.Run("TC-DEL-13_SuperAdminDeleteAnyDocumentSoftDeleteSuccess204", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		saToken := ctx.Tokens["admin@del.ac.id"]
		saUser := ctx.Users["admin@del.ac.id"]

		docID := uploadDocForm(t, ctx, mhsToken, "Proposal Budi Dihapus SA 13", "14", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, saToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204 for SA soft delete, got %d: %s", w.Code, w.Body.String())
		}

		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("doc not found: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("expected document to be soft-deleted")
		}

		// Verify storage_logs records soft_delete with SA user_id
		var log domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", docID, "soft_delete").First(&log).Error; err != nil {
			t.Fatalf("storage_logs not recorded: %v", err)
		}
		if log.UserID == nil || *log.UserID != saUser.ID {
			t.Errorf("expected log user_id to be %d, got %v", saUser.ID, log.UserID)
		}

		// File in storage is still intact
		_, _, err := ctx.Storage.Open(context.Background(), doc.StoragePath)
		if err != nil {
			t.Errorf("file in storage must still be intact after soft delete, err: %v", err)
		}
	})

	// TC-DEL-14: SA menjalankan hard delete (DELETE /documents/{id}?force=true)
	// Expected: 204; baris metadata dan objek GCS terhapus permanen; log tercatat
	t.Run("TC-DEL-14_SuperAdminHardDeleteForceTrueSuccess204", func(t *testing.T) {
		saToken := ctx.Tokens["admin@del.ac.id"]
		saUser := ctx.Users["admin@del.ac.id"]
		docID := uploadDocForm(t, ctx, saToken, "Hard Delete Target Doc 14", "1", "private", "1")

		var beforeDoc domain.Document
		if err := ctx.DB.First(&beforeDoc, docID).Error; err != nil {
			t.Fatalf("beforeDoc not found: %v", err)
		}
		storagePath := beforeDoc.StoragePath

		// Confirm file exists in storage before hard delete
		if _, _, err := ctx.Storage.Open(context.Background(), storagePath); err != nil {
			t.Fatalf("file not in storage before hard delete: %v", err)
		}

		// Execute hard delete
		url := fmt.Sprintf("/api/v1/documents/%d?force=true", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, saToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204, got %d: %s", w.Code, w.Body.String())
		}

		// 1. Verify document metadata permanently deleted from DB
		var afterDoc domain.Document
		err := ctx.DB.Unscoped().First(&afterDoc, docID).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected ErrRecordNotFound for hard-deleted doc, got err: %v", err)
		}

		// 2. Verify file object permanently deleted from storage
		_, _, sErr := ctx.Storage.Open(context.Background(), storagePath)
		if !errors.Is(sErr, storage.ErrNotFound) {
			t.Errorf("expected storage.ErrNotFound for hard-deleted storage object, got %v", sErr)
		}

		// 3. Verify storage_logs records hard_delete
		var log domain.StorageLog
		if err := ctx.DB.Where("document_id = ? AND action = ?", docID, "hard_delete").First(&log).Error; err != nil {
			t.Fatalf("hard_delete storage log not found: %v", err)
		}
		if log.UserID == nil || *log.UserID != saUser.ID {
			t.Errorf("expected storage log user_id = %d, got %v", saUser.ID, log.UserID)
		}
	})

	// TC-DEL-15: Non-Super Admin memakai ?force=true pada berkas miliknya
	// Expected: 403 (hard delete hanya Super Admin); berkas utuh
	t.Run("TC-DEL-15_NonSuperAdminForceTrueForbidden403", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		docID := uploadDocForm(t, ctx, mhsToken, "Proposal Budi Force True Target 15", "14", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d?force=true", docID)
		w, res := doDelReqJSON(ctx.Engine, http.MethodDelete, url, mhsToken, nil)
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected HTTP 403 for non-SA using force=true, got %d: %v", w.Code, res)
		}
		if res["code"] != "forbidden" {
			t.Errorf("expected code 'forbidden', got '%v'", res["code"])
		}

		// Verify document and storage remain intact
		var doc domain.Document
		if err := ctx.DB.First(&doc, docID).Error; err != nil {
			t.Fatalf("document not intact in DB: %v", err)
		}
		if doc.DeletedAt.Valid {
			t.Errorf("document should not be deleted")
		}
		if _, _, err := ctx.Storage.Open(context.Background(), doc.StoragePath); err != nil {
			t.Errorf("storage object should still exist: %v", err)
		}
	})

	// TC-DEL-16: DELETE tanpa token
	// Expected: 401
	t.Run("TC-DEL-16_DeleteWithoutTokenUnauthorized401", func(t *testing.T) {
		url := "/api/v1/documents/1"
		w := doDelReq(ctx.Engine, http.MethodDelete, url, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected HTTP 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	// TC-DEL-17: DELETE /documents/999999 (tidak ada)
	// Expected: 404
	t.Run("TC-DEL-17_DeleteNonExistentNotFound404", func(t *testing.T) {
		saToken := ctx.Tokens["admin@del.ac.id"]
		url := "/api/v1/documents/999999"
		w := doDelReq(ctx.Engine, http.MethodDelete, url, saToken)
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected HTTP 404, got %d: %s", w.Code, w.Body.String())
		}
	})

	// TC-DEL-18: DELETE berkas yang sudah di-soft delete oleh pengguna biasa
	// Expected: 404
	t.Run("TC-DEL-18_DeleteAlreadySoftDeletedByRegularUserNotFound404", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		docID := uploadDocForm(t, ctx, mhsToken, "Doc Already Soft Deleted 18", "14", "private", "")

		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		// First delete: success 204
		w1 := doDelReq(ctx.Engine, http.MethodDelete, url, mhsToken)
		if w1.Code != http.StatusNoContent {
			t.Fatalf("first delete failed: %d", w1.Code)
		}

		// Second delete by regular user: must return 404
		w2 := doDelReq(ctx.Engine, http.MethodDelete, url, mhsToken)
		if w2.Code != http.StatusNotFound {
			t.Fatalf("expected HTTP 404 on deleting already soft-deleted file by regular user, got %d: %s", w2.Code, w2.Body.String())
		}
	})

	// TC-DEL-19: Hapus berkas pending verifikasi oleh pemiliknya
	// Expected: Berkas terhapus dan baris antrean verifikasi ikut dibersihkan; notifikasi antrean tidak menunjuk berkas hilang
	t.Run("TC-DEL-19_DeletePendingPublicDocCleansVerificationAndNotifications", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		// Upload public doc: triggers pending verification and pending notification to BAAK & SA
		docID := uploadDocForm(t, ctx, mhsToken, "Proposal Pending Verifikasi Dihapus 19", "14", "public", "")

		// Verify pending verification exists
		var v domain.Verification
		if err := ctx.DB.Where("document_id = ?", docID).First(&v).Error; err != nil {
			t.Fatalf("verification row not found before delete: %v", err)
		}
		if v.Status != "pending" {
			t.Fatalf("expected verification status 'pending', got '%s'", v.Status)
		}

		// Verify notification exists for BAAK via API
		wNotifBefore, resNotifBefore := doDelReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens["baak@del.ac.id"], nil)
		if wNotifBefore.Code != http.StatusOK {
			t.Fatalf("get notifications failed: %d", wNotifBefore.Code)
		}
		dataNotifBefore, _ := resNotifBefore["data"].([]any)
		foundPendingBefore := false
		for _, item := range dataNotifBefore {
			m, _ := item.(map[string]any)
			dMap, _ := m["data"].(map[string]any)
			if dMap != nil && dMap["document_id"] == float64(docID) {
				foundPendingBefore = true
				break
			}
		}
		if !foundPendingBefore {
			t.Fatalf("expected pending verification notification to exist before delete in BAAK queue")
		}

		// Owner deletes the document
		url := fmt.Sprintf("/api/v1/documents/%d", docID)
		w := doDelReq(ctx.Engine, http.MethodDelete, url, mhsToken)
		if w.Code != http.StatusNoContent {
			t.Fatalf("expected HTTP 204, got %d: %s", w.Code, w.Body.String())
		}

		// 1. Verify document is soft-deleted
		var doc domain.Document
		if err := ctx.DB.Unscoped().First(&doc, docID).Error; err != nil {
			t.Fatalf("document not found: %v", err)
		}
		if !doc.DeletedAt.Valid {
			t.Errorf("document should be marked as soft-deleted")
		}

		// 2. Verify verification row is cleaned up (deleted)
		var vAfter domain.Verification
		err := ctx.DB.Where("document_id = ?", docID).First(&vAfter).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected verification row to be deleted, got err: %v", err)
		}

		// 3. Verify notification row for this document is cleaned up from BAAK
		wNotifAfter, resNotifAfter := doDelReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens["baak@del.ac.id"], nil)
		if wNotifAfter.Code != http.StatusOK {
			t.Fatalf("get notifications after delete failed: %d", wNotifAfter.Code)
		}
		dataNotifAfter, _ := resNotifAfter["data"].([]any)
		foundPendingAfter := false
		for _, item := range dataNotifAfter {
			m, _ := item.(map[string]any)
			dMap, _ := m["data"].(map[string]any)
			if dMap != nil && dMap["document_id"] == float64(docID) {
				foundPendingAfter = true
				break
			}
		}
		if foundPendingAfter {
			t.Errorf("pending verification notification still exists in BAAK inbox after doc deletion")
		}
	})

	// TC-DEL-20: Simulasikan GCS gagal saat hard delete
	// Expected: Galat ditangani; metadata tidak hilang lebih dulu (tidak ada objek yatim di GCS); operasi dapat diulang
	t.Run("TC-DEL-20_SimulateGCSFailureDuringHardDeleteRollbackAndRetry", func(t *testing.T) {
		saToken := ctx.Tokens["admin@del.ac.id"]
		docID := uploadDocForm(t, ctx, saToken, "Doc Simulation GCS Failure 20", "1", "private", "1")

		var docBefore domain.Document
		if err := ctx.DB.First(&docBefore, docID).Error; err != nil {
			t.Fatalf("docBefore not found: %v", err)
		}
		storagePath := docBefore.StoragePath

		// Verify file in storage
		if _, _, err := ctx.Storage.Open(context.Background(), storagePath); err != nil {
			t.Fatalf("storage file not found before delete: %v", err)
		}

		// 1. Inject failure into Storage.Delete
		ctx.Storage.mu.Lock()
		ctx.Storage.failDelete = true
		ctx.Storage.mu.Unlock()

		url := fmt.Sprintf("/api/v1/documents/%d?force=true", docID)
		wFail := doDelReq(ctx.Engine, http.MethodDelete, url, saToken)
		if wFail.Code != http.StatusInternalServerError {
			t.Fatalf("expected HTTP 500 when storage fails, got %d: %s", wFail.Code, wFail.Body.String())
		}

		// 2. Verify database rollback: metadata row still exists
		var docAfterFail domain.Document
		if err := ctx.DB.Unscoped().First(&docAfterFail, docID).Error; err != nil {
			t.Fatalf("metadata should NOT be deleted when storage fails (transaction rollback expected), got err: %v", err)
		}

		// File in storage is also still there
		if _, _, err := ctx.Storage.Open(context.Background(), storagePath); err != nil {
			t.Fatalf("file in storage must still be present: %v", err)
		}

		// 3. Restore storage healthy state and retry
		ctx.Storage.mu.Lock()
		ctx.Storage.failDelete = false
		ctx.Storage.mu.Unlock()

		wRetry := doDelReq(ctx.Engine, http.MethodDelete, url, saToken)
		if wRetry.Code != http.StatusNoContent {
			t.Fatalf("retry after restoring storage should succeed with 204, got %d: %s", wRetry.Code, wRetry.Body.String())
		}

		// 4. Verify hard delete succeeded on retry
		var docFinal domain.Document
		err := ctx.DB.Unscoped().First(&docFinal, docID).Error
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Errorf("expected ErrRecordNotFound after successful retry, got: %v", err)
		}

		// File in storage is now deleted
		_, _, sErr := ctx.Storage.Open(context.Background(), storagePath)
		if !errors.Is(sErr, storage.ErrNotFound) {
			t.Errorf("expected storage.ErrNotFound after successful retry, got: %v", sErr)
		}
	})
}
