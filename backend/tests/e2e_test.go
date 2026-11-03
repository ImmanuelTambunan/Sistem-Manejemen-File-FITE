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


func TestTS_AUTH_TokenRotationAndReuseDetection(t *testing.T) {
	r, db, _ := setupE2E(t)
	_ = db

	// 1. Initial login
	loginBody := map[string]string{
		"email":    "baak@del.ac.id",
		"password": "InitPass123!",
	}
	wLogin := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	if wLogin.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", wLogin.Code, wLogin.Body.String())
	}

	// Extract refresh_token cookie
	cookies := wLogin.Result().Cookies()
	var initialCookie *http.Cookie
	for _, ck := range cookies {
		if ck.Name == "refresh_token" {
			initialCookie = ck
			break
		}
	}
	if initialCookie == nil || initialCookie.Value == "" {
		t.Fatalf("expected refresh_token cookie set on login")
	}

	// 2. Perform legitimate token rotation
	refreshReq1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshReq1.AddCookie(initialCookie)
	wRefresh1 := httptest.NewRecorder()
	r.ServeHTTP(wRefresh1, refreshReq1)

	if wRefresh1.Code != http.StatusOK {
		t.Fatalf("expected 200 on 1st refresh, got %d: %s", wRefresh1.Code, wRefresh1.Body.String())
	}

	// Extract rotated cookie
	var rotatedCookie *http.Cookie
	for _, ck := range wRefresh1.Result().Cookies() {
		if ck.Name == "refresh_token" {
			rotatedCookie = ck
			break
		}
	}
	if rotatedCookie == nil || rotatedCookie.Value == initialCookie.Value {
		t.Fatalf("expected new rotated refresh token cookie")
	}

	// 3. Reuse Attack: Replaying initialCookie must be detected and rejected
	reuseReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	reuseReq.AddCookie(initialCookie)
	wReuse := httptest.NewRecorder()
	r.ServeHTTP(wReuse, reuseReq)

	if wReuse.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on token reuse attempt, got %d", wReuse.Code)
	}

	// 4. Session Invalidation: Even rotatedCookie is now revoked because reuse triggered session invalidation
	refreshReq2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshReq2.AddCookie(rotatedCookie)
	wRefresh2 := httptest.NewRecorder()
	r.ServeHTTP(wRefresh2, refreshReq2)

	if wRefresh2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 on rotated token after session revoked by reuse detection, got %d", wRefresh2.Code)
	}
}

func TestTS_UPL_ValidationAndLimits(t *testing.T) {
	r, _, _ := setupE2E(t)

	// Login as BAAK
	loginBody := map[string]string{"email": "baak@del.ac.id", "password": "InitPass123!"}
	wLogin := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", loginBody)
	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wLogin.Body.Bytes(), &loginResp)
	token := loginResp.Data.AccessToken

	// Helper for multipart uploads
	uploadFile := func(filename string, content []byte) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		_ = writer.WriteField("title", "Test File "+filename)
		_ = writer.WriteField("category_id", "2") // SK
		_ = writer.WriteField("visibility", "private")
		part, _ := writer.CreateFormFile("file", filename)
		_, _ = part.Write(content)
		_ = writer.Close()

		req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	// 1. Fake PDF content -> rejected 422
	wFake := uploadFile("fake.pdf", []byte("plain text disguised as pdf"))
	if wFake.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for fake pdf, got %d", wFake.Code)
	}

	// 2. Dangerous double extension -> rejected 422
	wDouble := uploadFile("report.pdf.exe", []byte("%PDF-1.4 sample"))
	if wDouble.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for double extension, got %d", wDouble.Code)
	}

	// 3. Unsupported extension (.sh) -> rejected 422
	wScript := uploadFile("script.sh", []byte("#!/bin/bash\necho test"))
	if wScript.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for .sh file, got %d", wScript.Code)
	}
}

func TestTS_SRC_MultiCriteriaFilterAndRBACScope(t *testing.T) {
	r, db, _ := setupE2E(t)

	// Seed Mahasiswa SI and Dosen IF
	prodiSI := uint(2)
	prodiIF := uint(1)
	pw, _ := bcrypt.GenerateFromPassword([]byte("InitPass123!"), bcrypt.DefaultCost)

	userMhsSI := domain.User{
		ID:                 10,
		Name:               "Siti SI",
		Email:              "siti@del.ac.id",
		PasswordHash:       string(pw),
		RoleID:             6, // Mahasiswa
		ProdiID:            &prodiSI,
		IsActive:           true,
		MustChangePassword: false,
	}
	db.Create(&userMhsSI)

	userDosenIF := domain.User{
		ID:                 11,
		Name:               "Pak Dosen IF",
		Email:              "dosen.if@del.ac.id",
		PasswordHash:       string(pw),
		RoleID:             5, // Dosen
		ProdiID:            &prodiIF,
		IsActive:           true,
		MustChangePassword: false,
	}
	db.Create(&userDosenIF)

	// Seed private document owned by Budi (Mahasiswa IF)
	docBudi := domain.Document{
		ID:           101,
		Title:        "Tesis Budi Private",
		OriginalName: "tesis_budi.pdf",
		Extension:    "pdf",
		MimeType:     "application/pdf",
		SizeKB:       50,
		StoragePath:  "path_budi.pdf",
		CategoryID:   1,
		ProdiID:      &prodiIF,
		UploaderID:   1, // Budi
		OwnerID:      1,
		OwnerRole:    domain.RoleMahasiswa,
		Visibility:   "private",
	}
	db.Create(&docBudi)

	// 1. Login as Siti (Mahasiswa SI)
	wSitiLogin := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "siti@del.ac.id", "password": "InitPass123!",
	})
	var sitiResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wSitiLogin.Body.Bytes(), &sitiResp)

	// Siti queries documents -> MUST NOT see Budi's private document (RBAC isolation)
	wSitiDocs := doJSON(t, r, http.MethodGet, "/api/v1/documents", sitiResp.Data.AccessToken, nil)
	if strings.Contains(wSitiDocs.Body.String(), "Tesis Budi Private") {
		t.Errorf("Mahasiswa SI should not see Mahasiswa IF's private document")
	}

	// 2. Login as Dosen IF
	wDosenLogin := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "dosen.if@del.ac.id", "password": "InitPass123!",
	})
	var dosenResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wDosenLogin.Body.Bytes(), &dosenResp)

	// Dosen IF queries documents -> CAN see Budi's document (same prodi student)
	wDosenDocs := doJSON(t, r, http.MethodGet, "/api/v1/documents", dosenResp.Data.AccessToken, nil)
	if !strings.Contains(wDosenDocs.Body.String(), "Tesis Budi Private") {
		t.Errorf("Dosen IF should be able to view documents of student in their own prodi")
	}

	// 3. Multi-criteria filtering by prodi
	wFiltered := doJSON(t, r, http.MethodGet, "/api/v1/documents?prodi=IF&extension=pdf", dosenResp.Data.AccessToken, nil)
	if !strings.Contains(wFiltered.Body.String(), "Tesis Budi Private") {
		t.Errorf("expected document to match prodi=IF filter")
	}
}

func TestTS_DEL_AuthorizationAndHardDelete(t *testing.T) {
	r, db, _ := setupE2E(t)

	prodiIF := uint(1)
	pw, _ := bcrypt.GenerateFromPassword([]byte("InitPass123!"), bcrypt.DefaultCost)

	// Seed another user
	userAttacker := domain.User{
		ID:                 20,
		Name:               "Attacker",
		Email:              "attacker@del.ac.id",
		PasswordHash:       string(pw),
		RoleID:             6, // Mahasiswa
		ProdiID:            &prodiIF,
		IsActive:           true,
		MustChangePassword: false,
	}
	db.Create(&userAttacker)

	docToDelete := domain.Document{
		ID:           201,
		Title:        "Doc Delete Test",
		OriginalName: "del.pdf",
		Extension:    "pdf",
		MimeType:     "application/pdf",
		SizeKB:       10,
		StoragePath:  "del_path.pdf",
		CategoryID:   1,
		ProdiID:      &prodiIF,
		UploaderID:   1, // Budi
		OwnerID:      1,
		OwnerRole:    domain.RoleMahasiswa,
		Visibility:   "private",
	}
	db.Create(&docToDelete)

	// Login Attacker
	wAtk := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "attacker@del.ac.id", "password": "InitPass123!",
	})
	var atkResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wAtk.Body.Bytes(), &atkResp)

	// Attacker tries to delete Budi's document -> 403 Forbidden
	wDelAtk := doJSON(t, r, http.MethodDelete, "/api/v1/documents/201", atkResp.Data.AccessToken, nil)
	if wDelAtk.Code != http.StatusForbidden {
		t.Errorf("expected 403 on unauthorized delete attempt, got %d", wDelAtk.Code)
	}

	// Login Super Admin
	wSA := doJSON(t, r, http.MethodPost, "/api/v1/auth/login", "", map[string]string{
		"email": "admin@del.ac.id", "password": "InitPass123!",
	})
	var saResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wSA.Body.Bytes(), &saResp)

	// Super Admin hard deletes with ?force=true -> 204 No Content
	wHardDel := doJSON(t, r, http.MethodDelete, "/api/v1/documents/201?force=true", saResp.Data.AccessToken, nil)
	if wHardDel.Code != http.StatusNoContent {
		t.Errorf("expected 204 on hard delete by super admin, got %d: %s", wHardDel.Code, wHardDel.Body.String())
	}

	// Document physically deleted from database
	var count int64
	db.Unscoped().Model(&domain.Document{}).Where("id = ?", 201).Count(&count)
	if count != 0 {
		t.Errorf("expected document to be permanently deleted from database")
	}
}
