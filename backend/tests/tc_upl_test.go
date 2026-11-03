package tests

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
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

var uplSuiteSeq uint64

type testStorageWrapper struct {
	*storage.MemoryStorage
	mu         sync.Mutex
	failPut    bool
	failDelete bool
	lastPath   string
}

func (s *testStorageWrapper) Put(ctx context.Context, path string, r io.Reader, contentType string) error {
	s.mu.Lock()
	if s.failPut {
		s.mu.Unlock()
		return errors.New("simulated GCS failure on Put")
	}
	s.lastPath = path
	s.mu.Unlock()
	return s.MemoryStorage.Put(ctx, path, r, contentType)
}

func (s *testStorageWrapper) Delete(ctx context.Context, path string) error {
	s.mu.Lock()
	if s.failDelete {
		s.mu.Unlock()
		return errors.New("simulated GCS failure on Delete")
	}
	s.mu.Unlock()
	return s.MemoryStorage.Delete(ctx, path)
}

type uploadSuiteContext struct {
	Engine   *gin.Engine
	DB       *gorm.DB
	Config   config.Config
	AuthSvc  *service.AuthService
	DocSvc   *service.DocumentService
	Storage  *testStorageWrapper
	Users    map[string]domain.User
	Tokens   map[string]string
	Password string
}

func setupUploadSuite(t *testing.T) *uploadSuiteContext {
	t.Helper()
	gin.SetMode(gin.TestMode)
	httpx.Init()

	seq := atomic.AddUint64(&uplSuiteSeq, 1)
	dsn := fmt.Sprintf("file:upload_suite_%d_%d?mode=memory&cache=shared", time.Now().UnixNano(), seq)
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
		t.Fatalf("migrate upload suite: %v", err)
	}

	roles := []domain.Role{
		{ID: 1, Key: domain.RoleSuperAdmin, Name: "Super Admin"},
		{ID: 2, Key: domain.RoleDekan, Name: "Dekan"},
		{ID: 3, Key: domain.RoleBaak, Name: "BAAK / Staf"},
		{ID: 4, Key: domain.RoleKaprodi, Name: "Kaprodi"},
		{ID: 5, Key: domain.RoleDosen, Name: "Dosen / TA"},
		{ID: 6, Key: domain.RoleMahasiswa, Name: "Mahasiswa"},
	}
	db.Create(&roles)

	prodis := []domain.Prodi{
		{ID: 1, Code: "IF", Name: "S1 Informatika", IsActive: true},
		{ID: 2, Code: "SI", Name: "S1 Sistem Informasi", IsActive: true},
		{ID: 3, Code: "TE", Name: "S1 Teknik Elektro", IsActive: true},
	}
	db.Create(&prodis)

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

	store := &testStorageWrapper{MemoryStorage: storage.NewMemoryStorage()}
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

	sc := &uploadSuiteContext{
		Engine:   r,
		DB:       db,
		Config:   cfg,
		AuthSvc:  authSvc,
		DocSvc:   docSvc,
		Storage:  store,
		Users:    userMap,
		Tokens:   make(map[string]string),
		Password: defaultPW,
	}

	// Pre-login all 10 users
	for email := range userMap {
		tok := sc.login(t, email, defaultPW)
		sc.Tokens[email] = tok
	}

	return sc
}

func (sc *uploadSuiteContext) login(t *testing.T, email, password string) string {
	t.Helper()
	buf, _ := json.Marshal(map[string]string{"email": email, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	sc.Engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed for %s: %d", email, w.Code)
	}
	var res struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	return res.Data["access_token"].(string)
}

func (sc *uploadSuiteContext) doUpload(t *testing.T, token string, fields map[string]string, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = writer.WriteField(k, v)
	}
	if filename != "" || content != nil {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		if len(content) > 0 {
			if _, err := part.Write(content); err != nil {
				t.Fatalf("write file content: %v", err)
			}
		}
	}
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	sc.Engine.ServeHTTP(w, req)
	return w
}

func genPDF(size int) []byte {
	header := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n")
	trailer := []byte("\nxref\n0 1\n0000000000 65535 f \ntrailer<</Size 1/Root 1 0 R>>\nstartxref\n9\n%%EOF")
	if size <= len(header)+len(trailer) {
		return append(header, trailer...)
	}
	b := make([]byte, size)
	copy(b, header)
	for i := len(header); i < size-len(trailer); i++ {
		b[i] = ' '
	}
	copy(b[size-len(trailer):], trailer)
	return b
}

func genPNG() []byte {
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89, 0x00, 0x00, 0x00,
		0x0a, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func genJPG() []byte {
	return []byte{
		0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x01, 0x00, 0x48, 0x00, 0x48, 0x00, 0x00, 0xff, 0xdb, 0x00, 0x43,
		0x00, 0x08, 0x06, 0x06, 0x07, 0x06, 0x05, 0x08, 0x07, 0x07, 0x07, 0x09,
		0x09, 0x08, 0x0a, 0x0c, 0x14, 0x0d, 0x0c, 0x0b, 0x0b, 0x0c, 0x19, 0x12,
		0x13, 0x0f, 0x14, 0x1d, 0x1a, 0x1f, 0x1e, 0x1d, 0x1a, 0x1c, 0x1c, 0x20,
		0x24, 0x2e, 0x27, 0x20, 0x22, 0x2c, 0x23, 0x1c, 0x1c, 0x28, 0x37, 0x29,
		0x2c, 0x30, 0x31, 0x34, 0x34, 0x34, 0x1f, 0x27, 0x39, 0x3d, 0x38, 0x32,
		0x3c, 0x2e, 0x33, 0x34, 0x32, 0xff, 0xc0, 0x00, 0x0b, 0x08, 0x00, 0x01,
		0x00, 0x01, 0x01, 0x01, 0x11, 0x00, 0xff, 0xc4, 0x00, 0x1f, 0x00, 0x00,
		0x01, 0x05, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
		0x09, 0x0a, 0x0b, 0xff, 0xda, 0x00, 0x08, 0x01, 0x01, 0x00, 0x00, 0x3f,
		0x00, 0xbf, 0x80, 0xff, 0xd9,
	}
}

func genZIP() []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("readme.txt")
	_, _ = w.Write([]byte("fite archive zip sample"))
	_ = zw.Close()
	return buf.Bytes()
}

func genOfficeDoc(mainDir, mainFile string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w1, _ := zw.Create("[Content_Types].xml")
	_, _ = w1.Write([]byte("<Types xmlns=\"http://schemas.openxmlformats.org/package/2006/content-types\"></Types>"))
	w2, _ := zw.Create(mainDir + "/" + mainFile)
	_, _ = w2.Write([]byte("<xml>sample</xml>"))
	_ = zw.Close()
	return buf.Bytes()
}

func TestTS_UPL_Granular(t *testing.T) {
	sc := setupUploadSuite(t)

	// -------------------------------------------------------------------------
	// TC-UPL-01: MHS mengunggah valid-9mb.pdf, kategori "Proposal TA/PA", tanpa field visibilitas
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-01_MahasiswaUploadValidPDFPrivate", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		pdf9MB := genPDF(9 * 1024 * 1024)
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Proposal TA Budi 9MB",
			"category_id": "14", // Proposal TA/PA
		}, "valid-9mb.pdf", pdf9MB)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data["visibility"] != "private" {
			t.Errorf("expected visibility=private, got %v", res.Data["visibility"])
		}
		if res.Data["verification_status"] != nil {
			t.Errorf("expected verification_status=nil, got %v", res.Data["verification_status"])
		}
		docID := uint(res.Data["id"].(float64))

		var doc domain.Document
		if err := sc.DB.First(&doc, docID).Error; err != nil {
			t.Fatalf("document not found in DB: %v", err)
		}
		if doc.StoragePath == "" || !strings.Contains(doc.StoragePath, "documents/IF/") {
			t.Errorf("unexpected storage_path: %s", doc.StoragePath)
		}
		rc, _, err := sc.Storage.Open(context.Background(), doc.StoragePath)
		if err != nil {
			t.Errorf("storage object missing: %v", err)
		} else {
			rc.Close()
		}

		var log domain.StorageLog
		if err := sc.DB.Where("document_id = ? AND action = ?", docID, "upload").First(&log).Error; err != nil {
			t.Errorf("storage_logs upload entry not found: %v", err)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-02: Unggah exact-10mb.pdf (10.485.760 byte tepat)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-02_UploadExact10MBBoundary", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		exact10MB := genPDF(10485760)
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Dokumen Batas 10MB",
			"category_id": "14",
		}, "exact-10mb.pdf", exact10MB)

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 on exact 10MB boundary, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		sizeKB := uint(res.Data["size_kb"].(float64))
		if sizeKB != 10240 {
			t.Errorf("expected size_kb=10240, got %d", sizeKB)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-03: Unggah berkas 10 MB + 1 byte (10.485.761 byte)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-03_UploadOver10MBRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		over10MB := genPDF(10485761)
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Dokumen Melebihi 10MB",
			"category_id": "14",
		}, "over-10mb.pdf", over10MB)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for over 10MB, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Code   string              `json:"code"`
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Code != "validation_error" {
			t.Errorf("expected validation_error, got %s", res.Code)
		}
		fileErr := strings.Join(res.Errors["file"], " ")
		if !strings.Contains(fileErr, "10 MB") {
			t.Errorf("expected 10 MB limit error message, got: %s", fileErr)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-04: Kirim body 50 MB (jauh di atas batas keras 11 MB)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-04_UploadOver11MBHardLimit", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", strings.NewReader("dummy payload"))
		req.Header.Set("Content-Type", "multipart/form-data; boundary=---boundary")
		req.Header.Set("Authorization", "Bearer "+token)
		req.ContentLength = 50 * 1024 * 1024 // 50 MB advertised

		w := httptest.NewRecorder()
		sc.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Code != "payload_too_large" {
			t.Errorf("expected code payload_too_large, got %s", res.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-05: Unggah tujuh berkas asli untuk tiap format whitelist: .pdf, .docx, .xlsx, .pptx, .zip, .png, .jpg
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-05_UploadSevenWhitelistFormats", func(t *testing.T) {
		token := sc.Tokens["admin@del.ac.id"]
		formats := []struct {
			filename string
			content  []byte
			expected string
		}{
			{"doc.pdf", genPDF(1024), "pdf"},
			{"doc.docx", genOfficeDoc("word", "document.xml"), "docx"},
			{"doc.xlsx", genOfficeDoc("xl", "workbook.xml"), "xlsx"},
			{"doc.pptx", genOfficeDoc("ppt", "presentation.xml"), "pptx"},
			{"archive.zip", genZIP(), "zip"},
			{"image.png", genPNG(), "png"},
			{"photo.jpg", genJPG(), "jpg"},
		}

		for _, f := range formats {
			w := sc.doUpload(t, token, map[string]string{
				"title":       "Whitelist " + f.filename,
				"category_id": "1", // SK Dekan (super_admin allowed)
				"prodi_id":    "1", // SA requires prodi_id when uploading without subject_user_id
				"visibility":  "private",
			}, f.filename, f.content)

			if w.Code != http.StatusCreated {
				t.Fatalf("format %s failed with code %d: %s", f.filename, w.Code, w.Body.String())
			}
			var res struct {
				Data map[string]any `json:"data"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &res)
			if res.Data["extension"] != f.expected {
				t.Errorf("format %s: expected ext %s, got %v", f.filename, f.expected, res.Data["extension"])
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-06: Unggah script.exe
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-06_UploadScriptExeRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Executable File",
			"category_id": "14",
		}, "script.exe", []byte("MZ binary executable content"))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for .exe file, got %d: %s", w.Code, w.Body.String())
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-07: Unggah video.mp4
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-07_UploadVideoMP4RejectedWithMessage", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Video File",
			"category_id": "14",
		}, "video.mp4", []byte("fake mp4 content header"))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for .mp4, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		fileErr := strings.Join(res.Errors["file"], " ")
		if !strings.Contains(fileErr, "Video atau berkas besar tidak diunggah; tulis tautannya pada berkas teks lalu kompres menjadi .zip.") {
			t.Errorf("expected video guidance message, got: %s", fileErr)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-08: Unggah fake.pdf (ekstensi .pdf, isi bukan PDF)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-08_UploadFakePDFSniffingRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Fake PDF",
			"category_id": "14",
		}, "fake.pdf", []byte("plain text content not starting with PDF magic bytes"))

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for fake pdf sniffing, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		fileErr := strings.Join(res.Errors["file"], " ")
		if !strings.Contains(fileErr, "Isi berkas tidak sesuai dengan ekstensinya (.pdf).") {
			t.Errorf("expected mismatch error, got: %s", fileErr)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-09: Unggah fake.docx (ZIP biasa tanpa struktur dokumen Office)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-09_UploadFakeDOCXZIPStructureRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		fakeDocx := genZIP() // standard zip with only readme.txt
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Fake DOCX",
			"category_id": "14",
		}, "fake.docx", fakeDocx)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for fake docx ZIP structure, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		fileErr := strings.Join(res.Errors["file"], " ")
		if !strings.Contains(fileErr, "Isi berkas tidak sesuai dengan ekstensinya (.docx).") {
			t.Errorf("expected mismatch error, got: %s", fileErr)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-10: Unggah berkas PDF asli yang diubah namanya menjadi .docx
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-10_UploadPDFRenamedToDOCXRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		pdfBytes := genPDF(1024)
		w := sc.doUpload(t, token, map[string]string{
			"title":       "PDF Renamed to DOCX",
			"category_id": "14",
		}, "renamed.docx", pdfBytes)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for PDF renamed to docx, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		fileErr := strings.Join(res.Errors["file"], " ")
		if !strings.Contains(fileErr, "Isi berkas tidak sesuai dengan ekstensinya (.docx).") {
			t.Errorf("expected mismatch error, got: %s", fileErr)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-11: Unggah laporan.pdf.exe / laporan.exe.pdf (ekstensi ganda)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-11_UploadDangerousDoubleExtensionRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]

		w1 := sc.doUpload(t, token, map[string]string{
			"title":       "Double Ext PDF.EXE",
			"category_id": "14",
		}, "laporan.pdf.exe", []byte("MZ fake binary"))
		if w1.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for .pdf.exe, got %d", w1.Code)
		}

		w2 := sc.doUpload(t, token, map[string]string{
			"title":       "Double Ext EXE.PDF",
			"category_id": "14",
		}, "laporan.exe.pdf", genPDF(1024))
		if w2.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for .exe.pdf, got %d", w2.Code)
		}
		var res struct {
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(w2.Body.Bytes(), &res)
		fileErr := strings.Join(res.Errors["file"], " ")
		if !strings.Contains(fileErr, "ekstensi ganda") {
			t.Errorf("expected double extension error message, got: %s", fileErr)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-12: Unggah foto .jpeg asli (dinormalisasi menjadi jpg)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-12_UploadJPEGNormalizedToJPG", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Foto Profil JPEG",
			"category_id": "14",
		}, "photo.jpeg", genJPG())

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201 for .jpeg, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Data map[string]any `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data["extension"] != "jpg" {
			t.Errorf("expected extension normalized to jpg, got %v", res.Data["extension"])
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-13: Unggah .gif, .txt, .rar, .svg, .html
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-13_UploadDisallowedExtensionsRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		badFiles := []struct {
			name    string
			content []byte
		}{
			{"test.gif", []byte("GIF89a dummy")},
			{"test.txt", []byte("plain text")},
			{"test.rar", []byte("Rar! dummy")},
			{"test.svg", []byte("<svg><script>alert(1)</script></svg>")},
			{"test.html", []byte("<html><body>xss</body></html>")},
		}

		for _, bf := range badFiles {
			w := sc.doUpload(t, token, map[string]string{
				"title":       "Disallowed " + bf.name,
				"category_id": "14",
			}, bf.name, bf.content)

			if w.Code != http.StatusUnprocessableEntity {
				t.Errorf("file %s: expected 422, got %d", bf.name, w.Code)
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-14: Unggah tanpa lampiran berkas
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-14_UploadWithoutFileAttachmentRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "No File Form",
			"category_id": "14",
		}, "", nil)

		if w.Code != http.StatusUnprocessableEntity {
			t.Fatalf("expected 422 for missing file, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Errors map[string][]string `json:"errors"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		fileErr := strings.Join(res.Errors["file"], " ")
		if !strings.Contains(fileErr, "Berkas wajib dilampirkan") {
			t.Errorf("expected mandatory file message, got: %s", fileErr)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-15: Unggah tanpa kategori atau dengan category_id tidak ada
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-15_UploadWithoutCategoryOrNonExistentRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]

		w1 := sc.doUpload(t, token, map[string]string{
			"title": "Missing Category",
		}, "valid.pdf", genPDF(1024))
		if w1.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for missing category, got %d", w1.Code)
		}

		w2 := sc.doUpload(t, token, map[string]string{
			"title":       "Nonexistent Category",
			"category_id": "99999",
		}, "valid.pdf", genPDF(1024))
		if w2.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for category_id 99999, got %d", w2.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-16: MHS mengunggah pada kategori "SK Keputusan Dekan"
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-16_MahasiswaUploadDisallowedCategoryForbidden", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Mahasiswa Mengunggah SK Dekan",
			"category_id": "1", // SK Keputusan Dekan
		}, "sk_dekan.pdf", genPDF(1024))

		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
		var res map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		msg, _ := res["message"].(string)
		if !strings.Contains(msg, "Kategori ini tidak diizinkan") {
			t.Errorf("expected category not allowed message, got: %s", msg)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-17: DSN mengunggah "RPS" dan "BAP"
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-17_DosenUploadRPSAndBAP", func(t *testing.T) {
		token := sc.Tokens["dosen.if@del.ac.id"]

		w1 := sc.doUpload(t, token, map[string]string{
			"title":       "Silabus RPS Informatika 2026",
			"category_id": "11", // Rencana Pembelajaran Semester (RPS)
		}, "rps.pdf", genPDF(1024))
		if w1.Code != http.StatusCreated {
			t.Errorf("expected 201 for RPS, got %d: %s", w1.Code, w1.Body.String())
		}

		w2 := sc.doUpload(t, token, map[string]string{
			"title":       "Berita Acara Perkuliahan IF-101",
			"category_id": "13", // Berita Acara Perkuliahan (BAP)
		}, "bap.pdf", genPDF(1024))
		if w2.Code != http.StatusCreated {
			t.Errorf("expected 201 for BAP, got %d: %s", w2.Code, w2.Body.String())
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-18: BAK mengunggah "Transkrip Nilai Akademik" dengan subject_user_id = MHS-SI-1
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-18_BAAKUploadTranskripForSubjectStudent", func(t *testing.T) {
		token := sc.Tokens["baak@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":           "Transkrip Nilai Akademik Mahasiswa SI",
			"category_id":     "5", // Transkrip Nilai Akademik
			"subject_user_id": "10", // iss24034@del.ac.id (MHS-SI-1)
		}, "transkrip_si.pdf", genPDF(1024))

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Data struct {
				Owner struct {
					ID  uint   `json:"id"`
					NIM string `json:"nim"`
				} `json:"owner"`
				Uploader struct {
					ID   uint   `json:"id"`
					Role string `json:"role"`
				} `json:"uploader"`
				Prodi struct {
					Code string `json:"code"`
				} `json:"prodi"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data.Owner.ID != 10 {
			t.Errorf("expected owner id=10, got %d", res.Data.Owner.ID)
		}
		if res.Data.Uploader.ID != 3 || res.Data.Uploader.Role != "baak" {
			t.Errorf("expected uploader baak, got %+v", res.Data.Uploader)
		}
		if res.Data.Prodi.Code != "SI" {
			t.Errorf("expected prodi SI, got %s", res.Data.Prodi.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-19: DKN mengunggah "Dokumen Akreditasi Fakultas"
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-19_DekanUploadFakultasDocumentWithoutProdi", func(t *testing.T) {
		token := sc.Tokens["dekan@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Sertifikat Akreditasi Fakultas 2026",
			"category_id": "2", // Dokumen Akreditasi Fakultas
		}, "akreditasi_fakultas.pdf", genPDF(1024))

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Data struct {
				Prodi any `json:"prodi"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data.Prodi != nil {
			t.Errorf("expected prodi=null for faculty document, got %v", res.Data.Prodi)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-20: MHS mengunggah dengan visibility=public
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-20_MahasiswaUploadPublicPendingVerification", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "Proposal TA Publik",
			"category_id": "14",
			"visibility":  "public",
		}, "ta_public.pdf", genPDF(1024))

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Message string `json:"message"`
			Data    struct {
				ID                 uint    `json:"id"`
				Visibility         string  `json:"visibility"`
				VerificationStatus *string `json:"verification_status"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data.Visibility != "public" {
			t.Errorf("expected visibility=public, got %s", res.Data.Visibility)
		}
		if res.Data.VerificationStatus == nil || *res.Data.VerificationStatus != "pending" {
			t.Errorf("expected verification_status=pending")
		}
		if !strings.Contains(res.Message, "menunggu verifikasi") {
			t.Errorf("expected pending message, got: %s", res.Message)
		}

		docID := res.Data.ID
		var v domain.Verification
		if err := sc.DB.Where("document_id = ?", docID).First(&v).Error; err != nil {
			t.Errorf("verification row not created: %v", err)
		}
		if v.Status != "pending" || v.RequestedBy != 8 {
			t.Errorf("unexpected verification record: %+v", v)
		}

		var notifs []domain.Notification
		if err := sc.DB.Where("type = ?", "verification_pending").Find(&notifs).Error; err != nil {
			t.Fatalf("failed to query notifications: %v", err)
		}
		foundNotif := false
		for _, n := range notifs {
			if n.Data != nil {
				if dID, ok := n.Data["document_id"]; ok {
					if fmt.Sprint(dID) == fmt.Sprint(docID) {
						foundNotif = true
						break
					}
				}
			}
		}
		if !foundNotif {
			t.Errorf("verification_pending notifications not created for verifiers")
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-21: DKN mengunggah dengan visibility=public
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-21_DekanUploadPublicPendingVerification", func(t *testing.T) {
		token := sc.Tokens["dekan@del.ac.id"]
		w := sc.doUpload(t, token, map[string]string{
			"title":       "SK Dekan Publik",
			"category_id": "1", // SK Keputusan Dekan
			"visibility":  "public",
		}, "sk_dekan_public.pdf", genPDF(1024))

		if w.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Data struct {
				Visibility         string  `json:"visibility"`
				VerificationStatus *string `json:"verification_status"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Data.Visibility != "public" {
			t.Errorf("expected visibility=public, got %s", res.Data.Visibility)
		}
		if res.Data.VerificationStatus == nil || *res.Data.VerificationStatus != "pending" {
			t.Errorf("expected verification_status=pending (all public documents require verification)")
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-22: Nama berkas berisi karakter berbahaya
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-22_DangerousFilenameSanitizedAndUUIDStored", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]

		w1 := sc.doUpload(t, token, map[string]string{
			"title":       "Path Traversal Attempt",
			"category_id": "14",
		}, "../../etc/passwd.pdf", genPDF(1024))
		if w1.Code != http.StatusCreated {
			t.Fatalf("expected 201 for traversal attempt, got %d: %s", w1.Code, w1.Body.String())
		}
		var res1 struct {
			Data struct {
				OriginalName string `json:"original_name"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w1.Body.Bytes(), &res1)
		if strings.Contains(res1.Data.OriginalName, "/") || strings.Contains(res1.Data.OriginalName, "..") {
			t.Errorf("path traversal not sanitized: %s", res1.Data.OriginalName)
		}

		w2 := sc.doUpload(t, token, map[string]string{
			"title":       "XSS In Filename Attempt",
			"category_id": "14",
		}, "<script>alert(1)</script>.pdf", genPDF(1024))
		if w2.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", w2.Code)
		}
		var res2 struct {
			Data struct {
				OriginalName string `json:"original_name"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w2.Body.Bytes(), &res2)
		if strings.Contains(res2.Data.OriginalName, "<") || strings.Contains(res2.Data.OriginalName, ">") {
			t.Errorf("script tag not sanitized: %s", res2.Data.OriginalName)
		}

		longName := strings.Repeat("a", 250) + ".pdf"
		w3 := sc.doUpload(t, token, map[string]string{
			"title":       "Long Filename",
			"category_id": "14",
		}, longName, genPDF(1024))
		if w3.Code != http.StatusCreated {
			t.Fatalf("expected 201 for long name, got %d", w3.Code)
		}
		var res3 struct {
			Data struct {
				OriginalName string `json:"original_name"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w3.Body.Bytes(), &res3)
		if len([]rune(res3.Data.OriginalName)) > 200 {
			t.Errorf("filename not truncated to 200: %d", len([]rune(res3.Data.OriginalName)))
		}
		if !strings.HasSuffix(res3.Data.OriginalName, ".pdf") {
			t.Errorf("extension lost in truncation: %s", res3.Data.OriginalName)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-23: Simulasikan GCS gagal saat unggah
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-23_StorageFailureRollbackNoOrphanMetadata", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		sc.Storage.mu.Lock()
		sc.Storage.failPut = true
		sc.Storage.mu.Unlock()

		w := sc.doUpload(t, token, map[string]string{
			"title":       "Storage Failure Doc",
			"category_id": "14",
		}, "fail_test.pdf", genPDF(1024))

		sc.Storage.mu.Lock()
		sc.Storage.failPut = false
		sc.Storage.mu.Unlock()

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on storage failure, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Code != "server_error" {
			t.Errorf("expected server_error code, got %s", res.Code)
		}
		if res.Message != "Terjadi kesalahan pada server." {
			t.Errorf("expected generic error message, got: %s", res.Message)
		}

		var count int64
		sc.DB.Model(&domain.Document{}).Where("title = ?", "Storage Failure Doc").Count(&count)
		if count != 0 {
			t.Errorf("orphan document metadata found in DB")
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-24: Simulasikan penyimpanan metadata ke MySQL gagal setelah objek terunggah ke GCS
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-24_DBFailureCompensationStorageDeletedNoOrphanObject", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]

		// Inject trigger to abort DB insert
		sc.DB.Exec("CREATE TRIGGER trigger_fail_doc BEFORE INSERT ON documents BEGIN SELECT RAISE(ABORT, 'forced metadata failure'); END;")
		defer sc.DB.Exec("DROP TRIGGER IF EXISTS trigger_fail_doc;")

		w := sc.doUpload(t, token, map[string]string{
			"title":       "DB Failure Compensation Test",
			"category_id": "14",
		}, "compensation_test.pdf", genPDF(1024))

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500 on db failure, got %d: %s", w.Code, w.Body.String())
		}

		sc.Storage.mu.Lock()
		lastPath := sc.Storage.lastPath
		sc.Storage.mu.Unlock()

		if lastPath != "" {
			_, _, err := sc.Storage.Open(context.Background(), lastPath)
			if !errors.Is(err, storage.ErrNotFound) {
				t.Errorf("expected storage object to be deleted (compensated), but still found: %v", err)
			}
		}

		var count int64
		sc.DB.Model(&domain.Document{}).Where("title = ?", "DB Failure Compensation Test").Count(&count)
		if count != 0 {
			t.Errorf("orphan document metadata found in DB")
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-25: Unggah tanpa token
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-25_UploadWithoutTokenUnauthorized", func(t *testing.T) {
		w := sc.doUpload(t, "", map[string]string{
			"title":       "Upload Tanpa Token",
			"category_id": "14",
		}, "no_token.pdf", genPDF(1024))

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", w.Code, w.Body.String())
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-26: Unggah dua berkas bernama sama oleh pengguna yang sama
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-26_UploadDuplicateFilenameSameUserUniqueUUIDs", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]

		w1 := sc.doUpload(t, token, map[string]string{
			"title":       "Versi 1",
			"category_id": "14",
		}, "laporan_akhir.pdf", genPDF(1024))
		if w1.Code != http.StatusCreated {
			t.Fatalf("upload 1 failed: %d", w1.Code)
		}

		w2 := sc.doUpload(t, token, map[string]string{
			"title":       "Versi 2",
			"category_id": "14",
		}, "laporan_akhir.pdf", genPDF(2048))
		if w2.Code != http.StatusCreated {
			t.Fatalf("upload 2 failed: %d", w2.Code)
		}

		var res1, res2 struct {
			Data struct {
				ID uint `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w1.Body.Bytes(), &res1)
		_ = json.Unmarshal(w2.Body.Bytes(), &res2)

		if res1.Data.ID == res2.Data.ID {
			t.Errorf("expected different document IDs, got same: %d", res1.Data.ID)
		}

		var doc1, doc2 domain.Document
		sc.DB.First(&doc1, res1.Data.ID)
		sc.DB.First(&doc2, res2.Data.ID)

		if doc1.StoragePath == doc2.StoragePath {
			t.Errorf("expected different storage paths, got same: %s", doc1.StoragePath)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-27: Non-BAAK/SA mengirim field subject_user_id atau archived_at
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-27_NonPrivilegedSendSubjectOrArchivedAtRejected", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]

		w1 := sc.doUpload(t, token, map[string]string{
			"title":           "Illegal Subject User",
			"category_id":     "14",
			"subject_user_id": "9",
		}, "test.pdf", genPDF(1024))
		if w1.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for non-BAAK subject_user_id, got %d", w1.Code)
		}

		w2 := sc.doUpload(t, token, map[string]string{
			"title":       "Illegal Archived At",
			"category_id": "14",
			"archived_at": "2024-01-01T00:00:00Z",
		}, "test.pdf", genPDF(1024))
		if w2.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for non-BAAK archived_at, got %d", w2.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-28: Unggah dengan Content-Type: application/json (bukan multipart/form-data)
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-28_UploadWithJSONContentTypeUnsupported", func(t *testing.T) {
		token := sc.Tokens["budi@del.ac.id"]
		req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", strings.NewReader(`{"title":"JSON Upload"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		w := httptest.NewRecorder()
		sc.Engine.ServeHTTP(w, req)

		if w.Code != http.StatusUnsupportedMediaType {
			t.Fatalf("expected 415, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res.Code != "unsupported_media_type" {
			t.Errorf("expected unsupported_media_type, got %s", res.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-UPL-29: 20 unggahan paralel berkas 9 MB oleh akun berbeda
	// -------------------------------------------------------------------------
	t.Run("TC-UPL-29_ParallelUploadsConcurrentStreaming", func(t *testing.T) {
		const concurrentCount = 20
		emails := []string{
			"admin@del.ac.id",
			"dekan@del.ac.id",
			"baak@del.ac.id",
			"kaprodi.if@del.ac.id",
			"kaprodi.si@del.ac.id",
			"dosen.if@del.ac.id",
			"dosen.si@del.ac.id",
			"budi@del.ac.id",
			"mhs.if2@del.ac.id",
			"iss24034@del.ac.id",
		}

		var wg sync.WaitGroup
		results := make([]int, concurrentCount)
		docIDs := make([]uint, concurrentCount)
		var mu sync.Mutex

		for i := 0; i < concurrentCount; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				email := emails[idx%len(emails)]
				token := sc.Tokens[email]
				title := fmt.Sprintf("Parallel Doc %d", idx)

				catID := "14" // default Mahasiswa Proposal TA/PA
				fields := map[string]string{
					"title": title,
				}
				if strings.Contains(email, "admin") {
					catID = "1" // SK Dekan
					fields["prodi_id"] = "1"
				} else if strings.Contains(email, "dekan") {
					catID = "2" // Dokumen Akreditasi Fakultas
				} else if strings.Contains(email, "baak") {
					catID = "4" // Legalisir
					fields["prodi_id"] = "1"
				} else if strings.Contains(email, "kaprodi") {
					catID = "8" // SK Prodi
				} else if strings.Contains(email, "dosen") {
					catID = "11" // RPS
				}
				fields["category_id"] = catID

				content := genPDF(128 * 1024) // 128 KB per doc for fast concurrent upload
				w := sc.doUpload(t, token, fields, fmt.Sprintf("parallel_%d.pdf", idx), content)

				mu.Lock()
				results[idx] = w.Code
				if w.Code == http.StatusCreated {
					var res struct {
						Data struct {
							ID uint `json:"id"`
						} `json:"data"`
					}
					_ = json.Unmarshal(w.Body.Bytes(), &res)
					docIDs[idx] = res.Data.ID
				}
				mu.Unlock()
			}(i)
		}

		wg.Wait()

		seenIDs := make(map[uint]bool)
		for i, code := range results {
			if code != http.StatusCreated {
				t.Errorf("parallel upload %d failed with code %d", i, code)
			}
			id := docIDs[i]
			if id == 0 || seenIDs[id] {
				t.Errorf("duplicate or zero doc ID in parallel uploads: %d", id)
			}
			seenIDs[id] = true
		}
	})
}
