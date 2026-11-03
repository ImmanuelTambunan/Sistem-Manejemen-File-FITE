package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var verSuiteSeq uint64

type verSuiteContext struct {
	Engine  *gin.Engine
	DB      *gorm.DB
	Config  config.Config
	Storage *storage.MemoryStorage
	Users   map[string]domain.User
	Tokens  map[string]string
}

func setupVerificationSuite(t *testing.T) *verSuiteContext {
	t.Helper()
	gin.SetMode(gin.TestMode)
	httpx.Init()

	seq := atomic.AddUint64(&verSuiteSeq, 1)
	dsn := fmt.Sprintf("file:ver_suite_%d_%d?mode=memory&cache=shared", time.Now().UnixNano(), seq)
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
		t.Fatalf("migrate verification suite: %v", err)
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
		AllowedOrigin:  "http://localhost:3000",
		AccessTTL:      15 * time.Minute,
		RefreshTTL:     7 * 24 * time.Hour,
		SessionMaxAge:  30 * 24 * time.Hour,
		MaxUploadBody:  11 << 20,
		MaxFileBytes:   10 << 20,
		MaxJSONBody:    1 << 20,
		RetentionYears: 8,
		GraceDays:      90,
	}

	memStore := storage.NewMemoryStorage()
	authSvc := service.NewAuthService(db, cfg)
	docSvc := service.NewDocumentService(db, memStore)
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

	engine := router.New(cfg, authSvc, h, func(ctx context.Context) error { return nil })

	tokenMap := make(map[string]string)
	for _, u := range userList {
		body, _ := json.Marshal(map[string]string{"email": u.Email, "password": defaultPW})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("setup login failed for %s: %d %s", u.Email, w.Code, w.Body.String())
		}
		var resp struct {
			Data struct {
				AccessToken string `json:"access_token"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &resp)
		tokenMap[u.Email] = resp.Data.AccessToken
	}

	return &verSuiteContext{
		Engine:  engine,
		DB:      db,
		Config:  cfg,
		Storage: memStore,
		Users:   userMap,
		Tokens:  tokenMap,
	}
}

func doVerReqJSON(engine *gin.Engine, method, path, token string, bodyObj any) (*httptest.ResponseRecorder, map[string]any) {
	var bodyReader io.Reader
	if bodyObj != nil {
		b, _ := json.Marshal(bodyObj)
		bodyReader = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, bodyReader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if bodyObj != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	var res map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return w, res
}

func doUploadDocPublic(t *testing.T, ctx *verSuiteContext, token, title, categoryID string) (uint, uint) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("title", title)
	_ = writer.WriteField("category_id", categoryID)
	_ = writer.WriteField("visibility", "public")

	part, err := writer.CreateFormFile("file", "sample_document.pdf")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	_, _ = part.Write([]byte("%PDF-1.4 sample pdf content for verification tests\n%%EOF"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	w := httptest.NewRecorder()
	ctx.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("upload document failed with status %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	docID := resp.Data.ID

	var v domain.Verification
	if err := ctx.DB.Where("document_id = ?", docID).First(&v).Error; err != nil {
		t.Fatalf("verification row not found for doc ID %d: %v", docID, err)
	}
	return docID, v.ID
}

// -----------------------------------------------------------------------------
// TestTS_VER_Granular executes all 19 official test cases defined in SRS Section 3.4
// -----------------------------------------------------------------------------
func TestTS_VER_Granular(t *testing.T) {
	ctx := setupVerificationSuite(t)

	var sharedDocID uint
	var sharedVerID uint

	// TC-VER-01: MHS mengunggah berkas visibility=public; kemudian BAK dan SA GET /notifications
	// -> Notifikasi “berkas publik baru” muncul pada BAK dan SA; tidak muncul pada DSN/KPR/DKN
	t.Run("TC-VER-01_MahasiswaUploadPublicNotifiesBAAKAndSAOnly", func(t *testing.T) {
		mhsToken := ctx.Tokens["budi@del.ac.id"]
		docID, verID := doUploadDocPublic(t, ctx, mhsToken, "Proposal TA Budi Publik 01", "14")
		sharedDocID = docID
		sharedVerID = verID

		// 1. Verify BAAK received notification
		wBAAK, resBAAK := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens["baak@del.ac.id"], nil)
		if wBAAK.Code != http.StatusOK {
			t.Fatalf("BAAK notifications failed with %d: %v", wBAAK.Code, resBAAK)
		}
		dataBAAK, _ := resBAAK["data"].([]any)
		foundBAAK := false
		for _, item := range dataBAAK {
			m, _ := item.(map[string]any)
			if m["type"] == "verification_pending" {
				foundBAAK = true
				break
			}
		}
		if !foundBAAK {
			t.Errorf("BAAK did not receive 'verification_pending' notification")
		}

		// 2. Verify Super Admin received notification
		wSA, resSA := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens["admin@del.ac.id"], nil)
		if wSA.Code != http.StatusOK {
			t.Fatalf("SA notifications failed with %d: %v", wSA.Code, resSA)
		}
		dataSA, _ := resSA["data"].([]any)
		foundSA := false
		for _, item := range dataSA {
			m, _ := item.(map[string]any)
			if m["type"] == "verification_pending" {
				foundSA = true
				break
			}
		}
		if !foundSA {
			t.Errorf("SA did not receive 'verification_pending' notification")
		}

		// 3. Verify non-verifiers (DSN, KPR, DKN) did NOT receive the notification
		nonVerifiers := []string{"dosen.if@del.ac.id", "kaprodi.if@del.ac.id", "dekan@del.ac.id"}
		for _, email := range nonVerifiers {
			w, res := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens[email], nil)
			if w.Code != http.StatusOK {
				t.Fatalf("notification query for %s failed with %d", email, w.Code)
			}
			items, _ := res["data"].([]any)
			for _, item := range items {
				m, _ := item.(map[string]any)
				if m["type"] == "verification_pending" {
					t.Errorf("unauthorized recipient %s received verification_pending notification", email)
				}
			}
		}
	})

	// TC-VER-02: BAK GET /verifications?status=pending -> 200; berkas dari TC-VER-01 ada dalam antrean
	t.Run("TC-VER-02_BAAKGetPendingQueueContainsDocument", func(t *testing.T) {
		w, res := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/verifications?status=pending", ctx.Tokens["baak@del.ac.id"], nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", w.Code)
		}
		items, _ := res["data"].([]any)
		found := false
		for _, item := range items {
			m, _ := item.(map[string]any)
			if uint(m["id"].(float64)) == sharedVerID {
				found = true
				doc, _ := m["document"].(map[string]any)
				if uint(doc["id"].(float64)) != sharedDocID {
					t.Errorf("verification ID %d references doc ID %v, expected %d", sharedVerID, doc["id"], sharedDocID)
				}
				if m["status"] != "pending" {
					t.Errorf("expected status 'pending', got '%v'", m["status"])
				}
				break
			}
		}
		if !found {
			t.Errorf("verification record %d from TC-VER-01 not found in pending queue", sharedVerID)
		}
	})

	// TC-VER-03: Sebelum diverifikasi: MHS lain GET /documents/public -> Berkas pending tidak tampil
	t.Run("TC-VER-03_PendingDocumentHiddenFromPublicExplorer", func(t *testing.T) {
		w, res := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/documents/public", ctx.Tokens["mhs.if2@del.ac.id"], nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", w.Code)
		}
		items, _ := res["data"].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			if uint(m["id"].(float64)) == sharedDocID {
				t.Errorf("pending document %d should NOT appear in public explorer before approval", sharedDocID)
			}
		}
	})

	// TC-VER-04: BAK PATCH /verifications/{id} {"status":"approved"} -> 200; status approved; verified_by dan verified_at terisi
	t.Run("TC-VER-04_BAAKApproveVerificationSuccess", func(t *testing.T) {
		url := fmt.Sprintf("/api/v1/verifications/%d", sharedVerID)
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "approved",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %v", w.Code, res)
		}
		data, _ := res["data"].(map[string]any)
		if data["status"] != "approved" {
			t.Errorf("expected status 'approved', got '%v'", data["status"])
		}
		verifier, _ := data["verified_by"].(map[string]any)
		if verifier == nil || uint(verifier["id"].(float64)) != ctx.Users["baak@del.ac.id"].ID {
			t.Errorf("expected verified_by ID %d, got %v", ctx.Users["baak@del.ac.id"].ID, verifier)
		}
		if data["verified_at"] == nil || data["verified_at"] == "" {
			t.Errorf("expected verified_at timestamp to be filled")
		}

		// Verify database state of the document
		var doc domain.Document
		if err := ctx.DB.First(&doc, sharedDocID).Error; err != nil {
			t.Fatalf("query document: %v", err)
		}
		if doc.Visibility != "public" || doc.VerificationStatus == nil || *doc.VerificationStatus != "approved" {
			t.Errorf("document DB state invalid: visibility=%s, verification_status=%v", doc.Visibility, doc.VerificationStatus)
		}
	})

	// TC-VER-05: Setelah TC-VER-04: pengguna terautentikasi lain (mis. MHS-SI-1) membuka explorer publik -> Berkas tampil dan dapat diunduh
	t.Run("TC-VER-05_ApprovedDocumentAppearsInPublicExplorerAndDownloadable", func(t *testing.T) {
		mhsSIToken := ctx.Tokens["iss24034@del.ac.id"]
		wPub, resPub := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/documents/public", mhsSIToken, nil)
		if wPub.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", wPub.Code)
		}
		items, _ := resPub["data"].([]any)
		found := false
		for _, item := range items {
			m, _ := item.(map[string]any)
			if uint(m["id"].(float64)) == sharedDocID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("approved document ID %d did not appear in public explorer for MHS-SI-1", sharedDocID)
		}

		// Test downloading the approved document as MHS-SI-1
		dlURL := fmt.Sprintf("/api/v1/documents/%d/download", sharedDocID)
		req := httptest.NewRequest(http.MethodGet, dlURL, nil)
		req.Header.Set("Authorization", "Bearer "+mhsSIToken)
		wDL := httptest.NewRecorder()
		ctx.Engine.ServeHTTP(wDL, req)
		if wDL.Code != http.StatusOK {
			t.Errorf("downloading public approved document by MHS-SI-1 returned status %d, expected 200", wDL.Code)
		}
	})

	// TC-VER-06: Setelah TC-VER-04: pengunggah menerima notifikasi -> Notifikasi “berkas disetujui” diterima pengunggah
	t.Run("TC-VER-06_UploaderReceivesApprovalNotification", func(t *testing.T) {
		w, res := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens["budi@del.ac.id"], nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", w.Code)
		}
		items, _ := res["data"].([]any)
		found := false
		for _, item := range items {
			m, _ := item.(map[string]any)
			if m["type"] == "verification_approved" {
				dataMap, _ := m["data"].(map[string]any)
				if dataMap != nil && uint(dataMap["document_id"].(float64)) == sharedDocID {
					found = true
					if !strings.Contains(m["title"].(string), "disetujui") {
						t.Errorf("expected approval title, got: %s", m["title"])
					}
					break
				}
			}
		}
		if !found {
			t.Errorf("uploader did not receive 'verification_approved' notification for document %d", sharedDocID)
		}
	})

	var rejectedDocID uint
	var rejectedVerID uint

	// TC-VER-07: BAK PATCH {"status":"rejected","reason":"Berkas tidak sesuai ketentuan"} -> 200; status rejected; alasan tersimpan
	t.Run("TC-VER-07_BAAKRejectVerificationWithReasonSuccess", func(t *testing.T) {
		// Student uploads new document for rejection testing
		docID, verID := doUploadDocPublic(t, ctx, ctx.Tokens["mhs.if2@del.ac.id"], "Proposal TA Mahasiswa IF-2 Rejection Target", "14")
		rejectedDocID = docID
		rejectedVerID = verID

		url := fmt.Sprintf("/api/v1/verifications/%d", rejectedVerID)
		reasonText := "Berkas tidak sesuai ketentuan tata tulis Del"
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "rejected",
			"reason": reasonText,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d: %v", w.Code, res)
		}
		data, _ := res["data"].(map[string]any)
		if data["status"] != "rejected" {
			t.Errorf("expected status 'rejected', got '%v'", data["status"])
		}
		if data["reason"] != reasonText {
			t.Errorf("expected reason '%s', got '%v'", reasonText, data["reason"])
		}

		// Verify database document state: reset to private and rejected
		var doc domain.Document
		if err := ctx.DB.First(&doc, rejectedDocID).Error; err != nil {
			t.Fatalf("query rejected doc: %v", err)
		}
		if doc.Visibility != "private" || doc.VerificationStatus == nil || *doc.VerificationStatus != "rejected" {
			t.Errorf("document state mismatch: visibility=%s, status=%v", doc.Visibility, doc.VerificationStatus)
		}
	})

	// TC-VER-08: Setelah TC-VER-07: cek explorer publik dan notifikasi pengunggah -> Berkas tidak tampil publik; pengunggah menerima notifikasi berisi alasan; berkas tetap dapat diakses pemilik (Private)
	t.Run("TC-VER-08_RejectedDocumentHiddenPublicNotifiesUploaderAccessibleAsPrivate", func(t *testing.T) {
		// 1. Not in public explorer
		wPub, resPub := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/documents/public", ctx.Tokens["budi@del.ac.id"], nil)
		if wPub.Code != http.StatusOK {
			t.Fatalf("public explorer failed: %d", wPub.Code)
		}
		items, _ := resPub["data"].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			if uint(m["id"].(float64)) == rejectedDocID {
				t.Errorf("rejected document %d appeared in public explorer", rejectedDocID)
			}
		}

		// 2. Uploader receives notification containing reason
		wNotif, resNotif := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens["mhs.if2@del.ac.id"], nil)
		if wNotif.Code != http.StatusOK {
			t.Fatalf("notifications query failed: %d", wNotif.Code)
		}
		notifItems, _ := resNotif["data"].([]any)
		foundNotif := false
		for _, item := range notifItems {
			m, _ := item.(map[string]any)
			if m["type"] == "verification_rejected" {
				msg := m["message"].(string)
				if strings.Contains(msg, "Berkas tidak sesuai ketentuan tata tulis Del") {
					foundNotif = true
					break
				}
			}
		}
		if !foundNotif {
			t.Errorf("uploader did not receive rejection notification with specified reason")
		}

		// 3. Document still accessible by owner as private document
		detailURL := fmt.Sprintf("/api/v1/documents/%d", rejectedDocID)
		wDetail, resDetail := doVerReqJSON(ctx.Engine, http.MethodGet, detailURL, ctx.Tokens["mhs.if2@del.ac.id"], nil)
		if wDetail.Code != http.StatusOK {
			t.Errorf("owner cannot access rejected private document: code %d, body %v", wDetail.Code, resDetail)
		}
	})

	// TC-VER-09: BAK PATCH {"status":"rejected"} tanpa reason -> 422; errors.reason wajib
	t.Run("TC-VER-09_RejectWithoutReasonFails422", func(t *testing.T) {
		_, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc for Missing Reason Test", "14")
		url := fmt.Sprintf("/api/v1/verifications/%d", verID)
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "rejected",
		})
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected HTTP 422 for rejection without reason, got %d", w.Code)
		}
		if res["code"] != "validation_error" {
			t.Errorf("expected code 'validation_error', got '%v'", res["code"])
		}
		errorsMap, _ := res["errors"].(map[string]any)
		if errorsMap["reason"] == nil {
			t.Errorf("expected validation error on 'reason' field, got errors: %v", errorsMap)
		}
	})

	// TC-VER-10: SA PATCH {"status":"approved"} pada berkas pending -> 200; berhasil (Super Admin berwenang)
	t.Run("TC-VER-10_SuperAdminApprovePendingVerificationSuccess", func(t *testing.T) {
		_, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc for Super Admin Verification", "14")
		url := fmt.Sprintf("/api/v1/verifications/%d", verID)
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["admin@del.ac.id"], map[string]string{
			"status": "approved",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("expected HTTP 200 for Super Admin approval, got %d: %v", w.Code, res)
		}
		data, _ := res["data"].(map[string]any)
		if data["status"] != "approved" {
			t.Errorf("expected status 'approved', got '%v'", data["status"])
		}
	})

	// TC-VER-11: DSN, KPR, DKN, dan MHS masing-masing PATCH /verifications/{id} -> 403 untuk semuanya
	t.Run("TC-VER-11_NonVerifiersForbiddenFromVerificationPATCH", func(t *testing.T) {
		_, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc for RBAC Verification Test", "14")
		url := fmt.Sprintf("/api/v1/verifications/%d", verID)

		unauthorizedEmails := []string{
			"dosen.if@del.ac.id",
			"kaprodi.if@del.ac.id",
			"dekan@del.ac.id",
			"budi@del.ac.id",
		}
		for _, email := range unauthorizedEmails {
			w, _ := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens[email], map[string]string{
				"status": "approved",
			})
			if w.Code != http.StatusForbidden {
				t.Errorf("expected HTTP 403 Forbidden for role %s, got %d", email, w.Code)
			}
		}
	})

	// TC-VER-12: BAK menyetujui berkas yang sudah approved -> 409; status tidak berubah
	t.Run("TC-VER-12_ReApproveAlreadyApprovedReturns409Conflict", func(t *testing.T) {
		// sharedVerID is already approved from TC-VER-04
		url := fmt.Sprintf("/api/v1/verifications/%d", sharedVerID)
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "approved",
		})
		if w.Code != http.StatusConflict {
			t.Fatalf("expected HTTP 409 Conflict, got %d: %v", w.Code, res)
		}
		if res["code"] != "conflict" {
			t.Errorf("expected code 'conflict', got '%v'", res["code"])
		}
	})

	// TC-VER-13: PATCH dengan status tidak valid ("maybe") -> 422
	t.Run("TC-VER-13_InvalidStatusEnumRejected422", func(t *testing.T) {
		_, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc for Invalid Status Test", "14")
		url := fmt.Sprintf("/api/v1/verifications/%d", verID)
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "maybe",
		})
		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected HTTP 422 for invalid status 'maybe', got %d", w.Code)
		}
		if res["code"] != "validation_error" {
			t.Errorf("expected code 'validation_error', got '%v'", res["code"])
		}
	})

	// TC-VER-14: PATCH /verifications/999999 (tidak ada) -> 404
	t.Run("TC-VER-14_NonExistentVerificationReturns404NotFound", func(t *testing.T) {
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, "/api/v1/verifications/999999", ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "approved",
		})
		if w.Code != http.StatusNotFound {
			t.Fatalf("expected HTTP 404, got %d: %v", w.Code, res)
		}
		if res["code"] != "not_found" {
			t.Errorf("expected code 'not_found', got '%v'", res["code"])
		}
	})

	// TC-VER-15: BAK dan SA menyetujui/menolak berkas yang sama bersamaan (dua permintaan paralel dengan keputusan berbeda)
	// -> Hanya satu keputusan tersimpan; permintaan lain 409; tidak ada data ganda; status akhir konsisten
	t.Run("TC-VER-15_ConcurrentDecisionsOneWinsOtherReturns409", func(t *testing.T) {
		_, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc for Concurrency Conflict Test", "14")
		url := fmt.Sprintf("/api/v1/verifications/%d", verID)

		var wg sync.WaitGroup
		wg.Add(2)

		var code1, code2 int
		var res1, res2 map[string]any

		go func() {
			defer wg.Done()
			w, r := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
				"status": "approved",
			})
			code1 = w.Code
			res1 = r
		}()

		go func() {
			defer wg.Done()
			w, r := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["admin@del.ac.id"], map[string]string{
				"status": "rejected",
				"reason": "Alasan penolakan dari SA paralel",
			})
			code2 = w.Code
			res2 = r
		}()

		wg.Wait()

		// Verify exactly one is 200 and the other is 409
		codes := []int{code1, code2}
		has200 := false
		has409 := false
		for _, c := range codes {
			if c == http.StatusOK {
				has200 = true
			} else if c == http.StatusConflict {
				has409 = true
			}
		}

		if !has200 || !has409 {
			t.Errorf("expected exactly one 200 and one 409 in concurrent decisions, got code1=%d (%v), code2=%d (%v)", code1, res1, code2, res2)
		}

		// Verify that exactly one verification record exists in DB
		var verCount int64
		ctx.DB.Model(&domain.Verification{}).Where("id = ?", verID).Count(&verCount)
		if verCount != 1 {
			t.Errorf("expected exactly 1 verification record, got %d", verCount)
		}
	})

	// TC-VER-16: Pengunggah mengembalikan visibilitas dari Public ke Private saat masih pending
	// -> Berkas keluar dari antrean; tidak ada verifikasi atau notifikasi menggantung
	t.Run("TC-VER-16_UploaderRevertsPendingPublicToPrivateRemovesQueueAndNotifications", func(t *testing.T) {
		docID, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc to Cancel While Pending", "14")

		// Verify pending verification exists
		var v domain.Verification
		if err := ctx.DB.First(&v, verID).Error; err != nil {
			t.Fatalf("verification not found: %v", err)
		}

		// Owner switches visibility back to private
		visURL := fmt.Sprintf("/api/v1/documents/%d/visibility", docID)
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, visURL, ctx.Tokens["budi@del.ac.id"], map[string]string{
			"visibility": "private",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("revert visibility failed with code %d: %v", w.Code, res)
		}

		// 1. Verify verification row deleted
		var vAfter domain.Verification
		err := ctx.DB.Where("id = ?", verID).First(&vAfter).Error
		if !strings.Contains(err.Error(), "record not found") {
			t.Errorf("expected verification row to be deleted, got err: %v", err)
		}

		// 2. Verify queue GET /verifications?status=pending does NOT contain doc
		wQ, resQ := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/verifications?status=pending", ctx.Tokens["baak@del.ac.id"], nil)
		if wQ.Code != http.StatusOK {
			t.Fatalf("verifications queue failed: %d", wQ.Code)
		}
		items, _ := resQ["data"].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			if uint(m["id"].(float64)) == verID {
				t.Errorf("cancelled verification %d still found in pending queue", verID)
			}
		}

		// 3. Verify document state
		var doc domain.Document
		ctx.DB.First(&doc, docID)
		if doc.Visibility != "private" || doc.VerificationStatus != nil {
			t.Errorf("expected visibility='private' and verification_status=nil, got vis=%s, stat=%v", doc.Visibility, doc.VerificationStatus)
		}
	})

	// TC-VER-17: Berkas approved diubah menjadi Private oleh pemilik -> Berkas hilang dari explorer publik; verifikasi lama dibersihkan
	t.Run("TC-VER-17_ApprovedDocumentRevertedToPrivateRemovedFromPublicExplorer", func(t *testing.T) {
		// 1. Upload public and approve
		docID, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc Approved Then Reverted To Private", "14")
		urlVer := fmt.Sprintf("/api/v1/verifications/%d", verID)
		wApp, _ := doVerReqJSON(ctx.Engine, http.MethodPatch, urlVer, ctx.Tokens["baak@del.ac.id"], map[string]string{"status": "approved"})
		if wApp.Code != http.StatusOK {
			t.Fatalf("approve failed: %d", wApp.Code)
		}

		// 2. Owner changes visibility to private
		visURL := fmt.Sprintf("/api/v1/documents/%d/visibility", docID)
		wPriv, resPriv := doVerReqJSON(ctx.Engine, http.MethodPatch, visURL, ctx.Tokens["budi@del.ac.id"], map[string]string{"visibility": "private"})
		if wPriv.Code != http.StatusOK {
			t.Fatalf("change visibility to private failed: %d %v", wPriv.Code, resPriv)
		}

		// 3. Disappears from public explorer
		wPub, resPub := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/documents/public", ctx.Tokens["iss24034@del.ac.id"], nil)
		if wPub.Code != http.StatusOK {
			t.Fatalf("public explorer failed: %d", wPub.Code)
		}
		items, _ := resPub["data"].([]any)
		for _, item := range items {
			m, _ := item.(map[string]any)
			if uint(m["id"].(float64)) == docID {
				t.Errorf("reverted document %d still visible in public explorer", docID)
			}
		}

		// 4. Old verification cleared
		var vCount int64
		ctx.DB.Model(&domain.Verification{}).Where("document_id = ?", docID).Count(&vCount)
		if vCount != 0 {
			t.Errorf("expected 0 verification rows after reverting to private, got %d", vCount)
		}
	})

	// TC-VER-18: Berkas rejected diajukan Public kembali oleh pemilik -> Verifikasi baru berstatus pending (alasan lama dikosongkan); notifikasi baru ke BAK dan SA
	t.Run("TC-VER-18_RejectedDocumentResubmittedAsPublicNewPendingVerification", func(t *testing.T) {
		// rejectedDocID is rejected from TC-VER-07
		visURL := fmt.Sprintf("/api/v1/documents/%d/visibility", rejectedDocID)
		w, res := doVerReqJSON(ctx.Engine, http.MethodPatch, visURL, ctx.Tokens["mhs.if2@del.ac.id"], map[string]string{
			"visibility": "public",
		})
		if w.Code != http.StatusOK {
			t.Fatalf("resubmitting rejected doc as public failed: code %d, body %v", w.Code, res)
		}

		// 1. Verify verification row exists and is pending with reason cleared
		var v domain.Verification
		if err := ctx.DB.Where("document_id = ?", rejectedDocID).First(&v).Error; err != nil {
			t.Fatalf("verification not found after resubmission: %v", err)
		}
		if v.Status != "pending" {
			t.Errorf("expected status 'pending', got '%s'", v.Status)
		}
		if v.Reason != nil && *v.Reason != "" {
			t.Errorf("expected reason to be cleared (nil), got '%v'", *v.Reason)
		}

		// 2. Verify new notification sent to BAAK and SA
		for _, email := range []string{"baak@del.ac.id", "admin@del.ac.id"} {
			wN, resN := doVerReqJSON(ctx.Engine, http.MethodGet, "/api/v1/notifications", ctx.Tokens[email], nil)
			if wN.Code != http.StatusOK {
				t.Fatalf("notifications query failed for %s: %d", email, wN.Code)
			}
			nItems, _ := resN["data"].([]any)
			found := false
			for _, item := range nItems {
				m, _ := item.(map[string]any)
				if m["type"] == "verification_pending" {
					dMap, _ := m["data"].(map[string]any)
					if dMap != nil && uint(dMap["document_id"].(float64)) == rejectedDocID {
						found = true
						break
					}
				}
			}
			if !found {
				t.Errorf("expected new verification_pending notification for %s on resubmitted doc %d", email, rejectedDocID)
			}
		}
	})

	// TC-VER-19: Menolak dengan reason kurang dari 5 karakter atau lebih dari 500 karakter -> 422
	t.Run("TC-VER-19_RejectReasonLengthBoundariesRejected422", func(t *testing.T) {
		_, verID := doUploadDocPublic(t, ctx, ctx.Tokens["budi@del.ac.id"], "Doc for Reason Length Validation", "14")
		url := fmt.Sprintf("/api/v1/verifications/%d", verID)

		// 1. Less than 5 characters ("abcd" = 4 chars)
		wShort, resShort := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "rejected",
			"reason": "abcd",
		})
		if wShort.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected HTTP 422 for reason < 5 chars, got %d: %v", wShort.Code, resShort)
		}
		if resShort["code"] != "validation_error" {
			t.Errorf("expected code 'validation_error', got '%v'", resShort["code"])
		}

		// 2. More than 500 characters (501 chars)
		longReason := strings.Repeat("A", 501)
		wLong, resLong := doVerReqJSON(ctx.Engine, http.MethodPatch, url, ctx.Tokens["baak@del.ac.id"], map[string]string{
			"status": "rejected",
			"reason": longReason,
		})
		if wLong.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected HTTP 422 for reason > 500 chars, got %d: %v", wLong.Code, resLong)
		}
		if resLong["code"] != "validation_error" {
			t.Errorf("expected code 'validation_error', got '%v'", resLong["code"])
		}
	})
}
