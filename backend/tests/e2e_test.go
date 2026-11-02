package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
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

var e2eSeq uint64

func setupE2E(t *testing.T) (*gin.Engine, *gorm.DB, config.Config) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	httpx.Init()

	seq := atomic.AddUint64(&e2eSeq, 1)
	dsn := fmt.Sprintf("file:e2e_%d_%d?mode=memory&cache=shared", time.Now().UnixNano(), seq)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
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
		t.Fatalf("migrate: %v", err)
	}

	// Seed roles
	roles := []domain.Role{
		{ID: 1, Key: domain.RoleSuperAdmin, Name: "Super Admin"},
		{ID: 2, Key: domain.RoleDekan, Name: "Dekan"},
		{ID: 3, Key: domain.RoleBaak, Name: "BAAK"},
		{ID: 4, Key: domain.RoleKaprodi, Name: "Kaprodi"},
		{ID: 5, Key: domain.RoleDosen, Name: "Dosen"},
		{ID: 6, Key: domain.RoleMahasiswa, Name: "Mahasiswa"},
	}
	db.Create(&roles)

	// Seed prodis
	prodis := []domain.Prodi{
		{ID: 1, Name: "Informatika", Code: "IF", IsActive: true},
		{ID: 2, Name: "Sistem Informasi", Code: "SI", IsActive: true},
		{ID: 3, Name: "Teknik Elektro", Code: "TE", IsActive: true},
	}
	db.Create(&prodis)

	// Seed categories
	cats := []domain.Category{
		{ID: 1, Name: "Tugas Akhir / Skripsi", AllowedRoles: domain.StringList{"mahasiswa", "dosen", "kaprodi"}},
		{ID: 2, Name: "Surat Keputusan", AllowedRoles: domain.StringList{"baak", "dekan", "super_admin"}, IsOperational: true},
		{ID: 3, Name: "Silabus & RPS", AllowedRoles: domain.StringList{"dosen", "kaprodi"}},
	}
	db.Create(&cats)

	// Hash password
	pw, _ := bcrypt.GenerateFromPassword([]byte("InitPass123!"), bcrypt.DefaultCost)
	prodiIF := uint(1)

	// Seed users
	users := []domain.User{
		{
			ID:                 1,
			Name:               "Budi Mahasiswa",
			Email:              "budi@del.ac.id",
			PasswordHash:       string(pw),
			RoleID:             6,
			ProdiID:            &prodiIF,
			IsActive:           true,
			MustChangePassword: true, // initial state
		},
		{
			ID:                 2,
			Name:               "Staf BAAK",
			Email:              "baak@del.ac.id",
			PasswordHash:       string(pw),
			RoleID:             3,
			IsActive:           true,
			MustChangePassword: false,
		},
		{
			ID:                 3,
			Name:               "Super Admin",
			Email:              "admin@del.ac.id",
			PasswordHash:       string(pw),
			RoleID:             1,
			IsActive:           true,
			MustChangePassword: false,
		},
	}
	db.Create(&users)

	cfg := config.Config{
		HTTPAddr:        ":8080",
		JWTSecret:       []byte("e2e-test-super-secret-key-at-least-32-bytes-long"),
		JWTIssuer:       "fite-arsip-api",
		JWTAudience:     "fite-arsip-web",
		AccessTTL:       15 * time.Minute,
		RefreshTTL:      7 * 24 * time.Hour,
		SessionMaxAge:   7 * 24 * time.Hour,
		MaxUploadBody:   11 << 20,
		MaxFileBytes:    10 << 20,
		MaxJSONBody:     1 << 20,
		RetentionYears:  8,
		GraceDays:       90,
	}

	memStore := storage.NewMemoryStorage()
	authSvc := service.NewAuthService(db, cfg)
	docSvc := service.NewDocumentService(db, memStore)
	verSvc := service.NewVerificationService(db)
	limiter := middleware.NewFailureLimiter(5, time.Minute)

	h := router.Handlers{
		Auth:         handler.NewAuthHandler(authSvc, cfg, limiter),
		Document:     handler.NewDocumentHandler(docSvc, cfg),
		Verification: handler.NewVerificationHandler(verSvc),
		Category:     handler.NewCategoryHandler(db),
		Notification: handler.NewNotificationHandler(db),
		User:         handler.NewUserHandler(db),
		Dashboard:    handler.NewDashboardHandler(db, cfg),
	}

	r := router.New(cfg, authSvc, h, nil)
	return r, db, cfg
}

func doJSON(t *testing.T, r *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rd = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestE2E_FullLifecycle_SRSCompliance(t *testing.T) {
	r, db, _ := setupE2E(t)

	// =========================================================================
	// 1. LOGIN & MUST_CHANGE_PASSWORD ENFORCEMENT
	// =========================================================================
	loginBody := map[string]string{
		"email":    "budi@del.ac.id",
		"password": "InitPass123!",
	}
	w := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
			User        struct {
				MustChangePassword bool `json:"must_change_password"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	studentToken := loginResp.Data.AccessToken
	if !loginResp.Data.User.MustChangePassword {
		t.Fatalf("expected must_change_password=true for new user")
	}

	// Try to access protected documents endpoint -> MUST receive 403 password_change_required
	w = doJSON(t, r, http.MethodGet, "/api/v1/documents", studentToken, nil)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 forbidden before password change, got %d", w.Code)
	}

	// Change password
	changeBody := map[string]string{
		"current_password":      "InitPass123!",
		"password":             "NewPass456#",
		"password_confirmation": "NewPass456#",
	}
	w = doJSON(t, r, http.MethodPut, "/api/v1/auth/password", studentToken, changeBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on change password, got %d: %s", w.Code, w.Body.String())
	}

	// Now access documents -> MUST succeed (200 OK)
	w = doJSON(t, r, http.MethodGet, "/api/v1/documents", studentToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on documents after password change, got %d: %s", w.Code, w.Body.String())
	}

	// =========================================================================
	// 2. DOCUMENT UPLOAD (MULTIPART) & VERIFICATION TRIGGER
	// =========================================================================
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	_ = writer.WriteField("title", "Skripsi Budi Bab 1-3")
	_ = writer.WriteField("category_id", "1")
	_ = writer.WriteField("visibility", "public") // public triggers verification
	part, _ := writer.CreateFormFile("file", "skripsi_budi.pdf")
	_, _ = part.Write([]byte("%PDF-1.4 test dummy content for pdf"))
	_ = writer.Close()

	uploadReq := httptest.NewRequest(http.MethodPost, "/api/v1/documents", &buf)
	uploadReq.Header.Set("Content-Type", writer.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+studentToken)
	uploadRec := httptest.NewRecorder()
	r.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 created on upload, got %d: %s", uploadRec.Code, uploadRec.Body.String())
	}

	var uploadResp struct {
		Data struct {
			ID                 uint    `json:"id"`
			Visibility         string  `json:"visibility"`
			VerificationStatus *string `json:"verification_status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp)
	docID := uploadResp.Data.ID
	if uploadResp.Data.VerificationStatus == nil || *uploadResp.Data.VerificationStatus != "pending" {
		t.Fatalf("expected verification_status=pending for public upload")
	}

	// Check that public documents endpoint DOES NOT show it yet
	w = doJSON(t, r, http.MethodGet, "/api/v1/documents/public", studentToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on public docs, got %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "Skripsi Budi Bab 1-3") {
		t.Fatalf("unapproved document should NOT be in public explorer")
	}

	// =========================================================================
	// 3. BAAK VERIFICATION WORKFLOW
	// =========================================================================
	baakLogin := map[string]string{"email": "baak@del.ac.id", "password": "InitPass123!"}
	w = doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", baakLogin)
	var baakResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &baakResp)
	baakToken := baakResp.Data.AccessToken

	// BAAK lists verifications
	w = doJSON(t, r, http.MethodGet, "/api/v1/verifications?status=pending", baakToken, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on verifications list, got %d", w.Code)
	}

	var verList struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &verList)
	if len(verList.Data) == 0 {
		t.Fatalf("expected pending verification in list")
	}
	verID := verList.Data[0].ID

	// BAAK rejects without reason (< 5 chars) -> 422 Unprocessable Entity
	badReject := map[string]string{"status": "rejected", "reason": "no"}
	w = doJSON(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/verifications/%d", verID), baakToken, badReject)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on reject reason < 5 chars, got %d", w.Code)
	}

	// BAAK approves document
	approveBody := map[string]string{"status": "approved"}
	w = doJSON(t, r, http.MethodPatch, fmt.Sprintf("/api/v1/verifications/%d", verID), baakToken, approveBody)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on approve, got %d: %s", w.Code, w.Body.String())
	}

	// Document now appears in /api/v1/documents/public!
	w = doJSON(t, r, http.MethodGet, "/api/v1/documents/public", studentToken, nil)
	if !strings.Contains(w.Body.String(), "Skripsi Budi Bab 1-3") {
		t.Fatalf("approved document should now be visible in public explorer")
	}

	// Student received notification of approval
	w = doJSON(t, r, http.MethodGet, "/api/v1/notifications", studentToken, nil)
	if !strings.Contains(w.Body.String(), "disetujui") {
		t.Fatalf("student should receive approval notification")
	}

	// =========================================================================
	// 4. RATE LIMITER: 5 FAILED LOGINS TRIGGER 429
	// =========================================================================
	wrongBody := map[string]string{"email": "budi@del.ac.id", "password": "WrongPassword123!"}
	for i := 1; i <= 5; i++ {
		w = doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", wrongBody)
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: expected 401, got %d", i, w.Code)
		}
	}
	// 6th attempt should be blocked with 429
	w = doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", wrongBody)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("attempt 6: expected 429 rate limit, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header on 429")
	}

	// =========================================================================
	// 5. RETENTION & CLEANUP ENGINE TEST
	// =========================================================================
	// Artificially age a document by 8 years + 1 day
	ancientDate := time.Now().AddDate(-8, 0, -2)
	db.Model(&domain.Document{}).Where("id = ?", docID).Update("archived_at", ancientDate)

	memStore := storage.NewMemoryStorage()
	retSvc := service.NewRetentionService(db, memStore, config.Config{RetentionYears: 8, GraceDays: 90})
	res, err := retSvc.Run(uploadReq.Context())
	if err != nil {
		t.Fatalf("retention run once: %v", err)
	}
	if res.SoftDeleted != 1 {
		t.Fatalf("expected 1 document soft-deleted by 8-year retention rule, got %d", res.SoftDeleted)
	}

	var d domain.Document
	db.Unscoped().First(&d, docID)
	if !d.DeletedAt.Valid {
		t.Fatalf("expected document to be soft-deleted")
	}
}
