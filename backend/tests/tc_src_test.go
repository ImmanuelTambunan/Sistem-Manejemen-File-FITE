package tests

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	srcSuiteSeq     uint64
	sqliteLikeOnce  sync.Once
	sqliteLikeError error
)

// registerSQLiteMySQLCompatLike registers a custom scalar function "like" in SQLite driver
// so that backslash escapes ('\%' and '\_') behave identically to MySQL without modifying production code.
func registerSQLiteMySQLCompatLike() {
	sqliteLikeOnce.Do(func() {
		sqliteLikeError = gosqlite.RegisterDeterministicScalarFunction("like", 2, func(ctx *gosqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) != 2 || args[0] == nil || args[1] == nil {
				return int64(0), nil
			}
			pat := fmt.Sprintf("%v", args[0])
			target := fmt.Sprintf("%v", args[1])
			if matchSQLLikeWithEscape(strings.ToLower(pat), strings.ToLower(target)) {
				return int64(1), nil
			}
			return int64(0), nil
		})
	})
}

func matchSQLLikeWithEscape(pat, target string) bool {
	return matchSQLLikeRunes([]rune(pat), []rune(target))
}

func matchSQLLikeRunes(p, t []rune) bool {
	for len(p) > 0 {
		if p[0] == '%' {
			for len(p) > 0 && p[0] == '%' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			for i := 0; i <= len(t); i++ {
				if matchSQLLikeRunes(p, t[i:]) {
					return true
				}
			}
			return false
		}
		if len(t) == 0 {
			return false
		}
		if p[0] == '\\' && len(p) > 1 {
			p = p[1:]
			if p[0] != t[0] {
				return false
			}
			p = p[1:]
			t = t[1:]
			continue
		}
		if p[0] == '_' || p[0] == t[0] {
			p = p[1:]
			t = t[1:]
			continue
		}
		return false
	}
	return len(t) == 0
}

type searchSuiteContext struct {
	Engine        *gin.Engine
	DB            *gorm.DB
	Config        config.Config
	Users         map[string]domain.User
	Tokens        map[string]string
	TotalDocsSeed int
	KnownDocIDs   map[string]uint
}

func setupSearchSuite(t *testing.T) *searchSuiteContext {
	t.Helper()
	registerSQLiteMySQLCompatLike()
	if sqliteLikeError != nil {
		t.Logf("note: sqlite custom like registration error: %v", sqliteLikeError)
	}

	gin.SetMode(gin.TestMode)
	httpx.Init()

	seq := atomic.AddUint64(&srcSuiteSeq, 1)
	dsn := fmt.Sprintf("file:src_suite_%d_%d?mode=memory&cache=shared", time.Now().UnixNano(), seq)
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
		t.Fatalf("migrate search suite: %v", err)
	}

	// 1. Seed Roles
	roles := []domain.Role{
		{ID: 1, Key: domain.RoleSuperAdmin, Name: "Super Admin"},
		{ID: 2, Key: domain.RoleDekan, Name: "Dekan"},
		{ID: 3, Key: domain.RoleBaak, Name: "BAAK / Staf"},
		{ID: 4, Key: domain.RoleKaprodi, Name: "Kaprodi"},
		{ID: 5, Key: domain.RoleDosen, Name: "Dosen / TA"},
		{ID: 6, Key: domain.RoleMahasiswa, Name: "Mahasiswa"},
	}
	db.Create(&roles)

	// 2. Seed Prodi
	prodis := []domain.Prodi{
		{ID: 1, Code: "IF", Name: "S1 Informatika", IsActive: true},
		{ID: 2, Code: "SI", Name: "S1 Sistem Informasi", IsActive: true},
		{ID: 3, Code: "TE", Name: "S1 Teknik Elektro", IsActive: true},
	}
	db.Create(&prodis)

	// 3. Seed 16 Categories per SRS
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

	// 4. Seed Official Users
	const defaultPW = "Password123!"
	pwHash, _ := bcrypt.GenerateFromPassword([]byte(defaultPW), 12)
	hashStr := string(pwHash)

	prodiIF := uint(1)
	prodiSI := uint(2)
	prodiTE := uint(3)
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

	// Pre-login all users to obtain Bearer tokens
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

	// 5. Seed Database with >= 200 documents (per Precondition TS-SRC)
	knownIDs := make(map[string]uint)
	now := time.Now().UTC()
	vApproved := "approved"
	vPending := "pending"
	vRejected := "rejected"

	// Specific target documents with known IDs and exact attributes
	specificDocs := []domain.Document{
		// TC-SRC-01, TC-SRC-05, TC-SRC-10 (Transkrip Budi IF 2024, uploaded by BAAK)
		{
			ID:           101,
			Title:        "Transkrip Nilai Akademik Mahasiswa Budi 2024",
			OriginalName: "transkrip_nilai_2024.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       120,
			StoragePath:  "documents/IF/2026/10/transkrip_budi_2024.pdf",
			CategoryID:   5, // Transkrip Nilai Akademik
			ProdiID:      &prodiIF,
			UploaderID:   3, // BAAK
			OwnerID:      8, // Budi (NIM: 11S20001)
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-10 * time.Hour),
		},
		// TC-SRC-01, TC-SRC-02 (Transkrip SI, uploaded by BAAK)
		{
			ID:           102,
			Title:        "Transkrip Nilai Mahasiswa SI Semester Ganjil",
			OriginalName: "transkrip_si.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       150,
			StoragePath:  "documents/SI/2026/10/transkrip_si.pdf",
			CategoryID:   5, // Transkrip Nilai Akademik
			ProdiID:      &prodiSI,
			UploaderID:   3, // BAAK
			OwnerID:      10, // Mahasiswa SI-1
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-9 * time.Hour),
		},
		// TC-SRC-01, TC-SRC-03 (Transkrip uppercase, uploader Dosen)
		{
			ID:           103,
			Title:        "TRANSKRIP CAPAIAN DOSEN SEMESTER GENAP",
			OriginalName: "transkrip_capaian.docx",
			Extension:    "docx",
			MimeType:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			SizeKB:       200,
			StoragePath:  "documents/IF/2026/10/transkrip_dosen.docx",
			CategoryID:   12, // SK Mengajar
			ProdiID:      &prodiIF,
			UploaderID:   6, // Dosen IF
			OwnerID:      6, // Dosen IF
			OwnerRole:    domain.RoleDosen,
			Visibility:   "private",
			CreatedAt:    now.Add(-8 * time.Hour),
		},
		// TC-SRC-04 (Legalisir category_id=4)
		{
			ID:           104,
			Title:        "Legalisir Ijazah dan Sertifikat Akreditasi",
			OriginalName: "legalisir_ijazah.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       80,
			StoragePath:  "documents/IF/2026/10/legalisir_ijazah.pdf",
			CategoryID:   4, // Legalisir
			ProdiID:      &prodiIF,
			UploaderID:   3, // BAAK
			OwnerID:      8, // Budi
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-7 * time.Hour),
		},
		// TC-SRC-05 (Budi own uploaded file)
		{
			ID:           105,
			Title:        "Proposal Tugas Akhir Budi Mandiri",
			OriginalName: "proposal_ta_budi.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       350,
			StoragePath:  "documents/IF/2026/10/proposal_ta_budi.pdf",
			CategoryID:   14, // Proposal TA/PA
			ProdiID:      &prodiIF,
			UploaderID:   8, // Budi
			OwnerID:      8, // Budi
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-6 * time.Hour),
		},
		// TC-SRC-06 (Dosen IF with NIP 198505052010011002)
		{
			ID:           106,
			Title:        "RPS Pemrograman Berorientasi Objek IF",
			OriginalName: "rps_pbo_if.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       300,
			StoragePath:  "documents/IF/2026/10/rps_pbo_if.pdf",
			CategoryID:   11, // RPS
			ProdiID:      &prodiIF,
			UploaderID:   6, // Dosen IF
			OwnerID:      6, // Dosen IF (NIP 198505052010011002)
			OwnerRole:    domain.RoleDosen,
			Visibility:   "private",
			CreatedAt:    now.Add(-5 * time.Hour),
		},
		// TC-SRC-06 (Dosen SI with NIP 198707072012011004 - readable by Kaprodi IF because faculty-wide dosen)
		{
			ID:           107,
			Title:        "RPS Basis Data Terdistribusi SI",
			OriginalName: "rps_basisdata_si.xlsx",
			Extension:    "xlsx",
			MimeType:     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			SizeKB:       250,
			StoragePath:  "documents/SI/2026/10/rps_basisdata_si.xlsx",
			CategoryID:   11, // RPS
			ProdiID:      &prodiSI,
			UploaderID:   7, // Dosen SI
			OwnerID:      7, // Dosen SI (NIP 198707072012011004)
			OwnerRole:    domain.RoleDosen,
			Visibility:   "private",
			CreatedAt:    now.Add(-4 * time.Hour),
		},
		// TC-SRC-08, TC-SRC-14 (Public Approved Document in TE prodi)
		{
			ID:                 108,
			Title:              "Laporan Akreditasi Program Studi Teknik Elektro Publik",
			OriginalName:       "akreditasi_te_public.pdf",
			Extension:          "pdf",
			MimeType:           "application/pdf",
			SizeKB:             500,
			StoragePath:        "documents/TE/2026/10/akreditasi_te.pdf",
			CategoryID:         2, // Dokumen Akreditasi Fakultas
			ProdiID:            &prodiTE,
			UploaderID:         2, // Dekan
			OwnerID:            2, // Dekan
			OwnerRole:          domain.RoleDekan,
			Visibility:         "public",
			VerificationStatus: &vApproved,
			CreatedAt:          now.Add(-3 * time.Hour),
		},
		// TC-SRC-08 negative test: Public but Pending (must NOT appear in public_approved)
		{
			ID:                 109,
			Title:              "Proposal Publik Menunggu Persetujuan Verifikator",
			OriginalName:       "proposal_pending.pdf",
			Extension:          "pdf",
			MimeType:           "application/pdf",
			SizeKB:             110,
			StoragePath:        "documents/IF/2026/10/proposal_pending.pdf",
			CategoryID:         14,
			ProdiID:            &prodiIF,
			UploaderID:         8,
			OwnerID:            8,
			OwnerRole:          domain.RoleMahasiswa,
			Visibility:         "public",
			VerificationStatus: &vPending,
			CreatedAt:          now.Add(-2 * time.Hour),
		},
		// TC-SRC-08 negative test: Public but Rejected
		{
			ID:                 110,
			Title:              "Berkas Publik Ditolak BAAK",
			OriginalName:       "proposal_rejected.pdf",
			Extension:          "pdf",
			MimeType:           "application/pdf",
			SizeKB:             115,
			StoragePath:        "documents/IF/2026/10/proposal_rejected.pdf",
			CategoryID:         14,
			ProdiID:            &prodiIF,
			UploaderID:         9,
			OwnerID:            9,
			OwnerRole:          domain.RoleMahasiswa,
			Visibility:         "private",
			VerificationStatus: &vRejected,
			CreatedAt:          now.Add(-1 * time.Hour),
		},
		// TC-SRC-10 Contrast: Mismatches category (4 instead of 5)
		{
			ID:           111,
			Title:        "Legalisir Transkrip 2024 IF",
			OriginalName: "legalisir_2024.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       90,
			StoragePath:  "documents/IF/2026/10/legalisir_2024.pdf",
			CategoryID:   4, // Legalisir (mismatches 5)
			ProdiID:      &prodiIF,
			UploaderID:   3,
			OwnerID:      8,
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-30 * time.Minute),
		},
		// TC-SRC-10 Contrast: Mismatches extension (docx instead of pdf)
		{
			ID:           112,
			Title:        "Transkrip Nilai Akademik 2024 IF Docx",
			OriginalName: "transkrip_2024.docx",
			Extension:    "docx",
			MimeType:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			SizeKB:       95,
			StoragePath:  "documents/IF/2026/10/transkrip_2024.docx",
			CategoryID:   5,
			ProdiID:      &prodiIF,
			UploaderID:   3,
			OwnerID:      8,
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-25 * time.Minute),
		},
		// TC-SRC-10 Contrast: Mismatches query q (no "2024")
		{
			ID:           113,
			Title:        "Transkrip Nilai Akademik 2023 IF",
			OriginalName: "transkrip_2023.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       105,
			StoragePath:  "documents/IF/2026/10/transkrip_2023.pdf",
			CategoryID:   5,
			ProdiID:      &prodiIF,
			UploaderID:   3,
			OwnerID:      8,
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-20 * time.Minute),
		},
		// TC-SRC-12: MHS-IF-2 document (secret from MHS-IF-1)
		{
			ID:           114,
			Title:        "Berkas Yudisium Mahasiswa IF-2 Rahasia",
			OriginalName: "yudisium_mhs_if2.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       130,
			StoragePath:  "documents/IF/2026/10/yudisium_if2.pdf",
			CategoryID:   15, // Berkas Yudisium
			ProdiID:      &prodiIF,
			UploaderID:   9, // MHS-IF-2
			OwnerID:      9, // MHS-IF-2 (NIM 11S20002)
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-15 * time.Minute),
		},
		// TC-SRC-13: Mahasiswa SI document (secret from Dosen IF)
		{
			ID:           115,
			Title:        "Proposal TA Mahasiswa SI Khusus",
			OriginalName: "proposal_mhs_si.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       140,
			StoragePath:  "documents/SI/2026/10/proposal_si.pdf",
			CategoryID:   14, // Proposal TA/PA
			ProdiID:      &prodiSI,
			UploaderID:   10, // MHS-SI-1
			OwnerID:      10, // MHS-SI-1 (NIM iss24034)
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now.Add(-10 * time.Minute),
		},
		// TC-SRC-14: TE Private Document (readable by Dekan)
		{
			ID:           116,
			Title:        "Arsip Khusus Fakultas Teknik Elektro Private",
			OriginalName: "arsip_khusus_te.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       160,
			StoragePath:  "documents/TE/2026/10/arsip_khusus_te.pdf",
			CategoryID:   3, // Arsip Khusus
			ProdiID:      &prodiTE,
			UploaderID:   2, // Dekan
			OwnerID:      2, // Dekan
			OwnerRole:    domain.RoleDekan,
			Visibility:   "private",
			CreatedAt:    now.Add(-8 * time.Minute),
		},
		// TC-SRC-19: Soft-deleted document
		{
			ID:           117,
			Title:        "Dokumen Yang Sudah Di Soft Delete",
			OriginalName: "dokumen_soft_deleted.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       75,
			StoragePath:  "documents/IF/2026/10/deleted_doc.pdf",
			CategoryID:   6,
			ProdiID:      &prodiIF,
			UploaderID:   3,
			OwnerID:      3,
			OwnerRole:    domain.RoleBaak,
			Visibility:   "private",
			CreatedAt:    now.Add(-7 * time.Minute),
			DeletedAt:    gorm.DeletedAt{Time: now.Add(-5 * time.Minute), Valid: true},
		},
		// TC-SRC-21: Literal 100%
		{
			ID:           118,
			Title:        "Laporan Capaian 100% Selesai Tuntas",
			OriginalName: "capaian_100_persen.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       60,
			StoragePath:  "documents/IF/2026/10/capaian_100.pdf",
			CategoryID:   6,
			ProdiID:      &prodiIF,
			UploaderID:   3,
			OwnerID:      3,
			OwnerRole:    domain.RoleBaak,
			Visibility:   "private",
			CreatedAt:    now.Add(-6 * time.Minute),
		},
		// TC-SRC-21 Negative: Contains 1000 without %
		{
			ID:           119,
			Title:        "Laporan Capaian 1000 Mahasiswa Baru",
			OriginalName: "capaian_1000_mhs.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       65,
			StoragePath:  "documents/IF/2026/10/capaian_1000.pdf",
			CategoryID:   6,
			ProdiID:      &prodiIF,
			UploaderID:   3,
			OwnerID:      3,
			OwnerRole:    domain.RoleBaak,
			Visibility:   "private",
			CreatedAt:    now.Add(-5 * time.Minute),
		},
		// TC-SRC-21: Literal a_b
		{
			ID:           120,
			Title:        "Modul Praktikum Bagian a_b Algoritma",
			OriginalName: "modul_a_b.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       70,
			StoragePath:  "documents/IF/2026/10/modul_a_b.pdf",
			CategoryID:   11,
			ProdiID:      &prodiIF,
			UploaderID:   6,
			OwnerID:      6,
			OwnerRole:    domain.RoleDosen,
			Visibility:   "private",
			CreatedAt:    now.Add(-4 * time.Minute),
		},
		// TC-SRC-21 Negative: Contains axb without underscore
		{
			ID:           121,
			Title:        "Modul Praktikum Bagian axb Perkalian",
			OriginalName: "modul_axb.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       72,
			StoragePath:  "documents/IF/2026/10/modul_axb.pdf",
			CategoryID:   11,
			ProdiID:      &prodiIF,
			UploaderID:   6,
			OwnerID:      6,
			OwnerRole:    domain.RoleDosen,
			Visibility:   "private",
			CreatedAt:    now.Add(-3 * time.Minute),
		},
	}

	for _, d := range specificDocs {
		db.Create(&d)
		knownIDs[d.Title] = d.ID
	}

	// Bulk seed additional varied documents up to 215 items
	exts := []string{"pdf", "docx", "xlsx", "pptx", "zip", "png", "jpg"}
	bulkDocs := make([]domain.Document, 0, 200)
	for i := 1; i <= 195; i++ {
		id := uint(200 + i)
		ext := exts[i%len(exts)]
		catID := uint((i % 16) + 1)
		var pID *uint
		switch i % 4 {
		case 0:
			pID = &prodiIF
		case 1:
			pID = &prodiSI
		case 2:
			pID = &prodiTE
		case 3:
			pID = nil // faculty level
		}

		uID := uint((i % 10) + 1)
		ownerUser := userList[uID-1]
		vis := "private"
		var vstat *string
		if i%5 == 0 {
			vis = "public"
			vstat = &vApproved
		} else if i%7 == 0 {
			vis = "public"
			vstat = &vPending
		}

		size := uint(10 + (i * 37 % 5000)) // varied sizes for sorting test
		doc := domain.Document{
			ID:                 id,
			Title:              fmt.Sprintf("Arsip Dokumen Kampus FITE Item %03d", i),
			OriginalName:       fmt.Sprintf("berkas_arsip_%03d.%s", i, ext),
			Extension:          ext,
			MimeType:           "application/octet-stream",
			SizeKB:             size,
			StoragePath:        fmt.Sprintf("documents/auto/item_%03d.%s", i, ext),
			CategoryID:         catID,
			ProdiID:            pID,
			UploaderID:         uID,
			OwnerID:            ownerUser.ID,
			OwnerRole:          ownerUser.Role.Key,
			Visibility:         vis,
			VerificationStatus: vstat,
			CreatedAt:          now.Add(-time.Duration(200-i) * time.Minute),
		}
		bulkDocs = append(bulkDocs, doc)
	}
	db.CreateInBatches(&bulkDocs, 50)

	var totalCount int64
	db.Model(&domain.Document{}).Count(&totalCount)

	return &searchSuiteContext{
		Engine:        engine,
		DB:            db,
		Config:        cfg,
		Users:         userMap,
		Tokens:        tokenMap,
		TotalDocsSeed: int(totalCount),
		KnownDocIDs:   knownIDs,
	}
}

type pagedDocResponse struct {
	Success bool                     `json:"success"`
	Message string                  `json:"message"`
	Data    []handler.DocumentResponse `json:"data"`
	Meta    httpx.Meta              `json:"meta"`
	Code    string                  `json:"code,omitempty"`
	Errors  map[string][]string     `json:"errors,omitempty"`
}

func doSearchRequest(engine *gin.Engine, token, path string) (int, pagedDocResponse, error) {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	var resp pagedDocResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp, err
}

// -----------------------------------------------------------------------------
// TestTS_SRC_Granular executes all 22 official test cases defined in SRS Section 3.3
// -----------------------------------------------------------------------------
func TestTS_SRC_Granular(t *testing.T) {
	ctx := setupSearchSuite(t)

	// Precondition Verification
	t.Run("Precondition_DatabaseContainsAtLeast200Documents", func(t *testing.T) {
		if ctx.TotalDocsSeed < 200 {
			t.Fatalf("Precondition failed: expected >= 200 documents in database, got %d", ctx.TotalDocsSeed)
		}
		t.Logf("Precondition verified: database contains %d documents (> 200 requirement)", ctx.TotalDocsSeed)
	})

	// TC-SRC-01: BAK: ?q=transkrip -> 200; semua nama berkas memuat “transkrip” (tidak peka huruf besar-kecil)
	t.Run("TC-SRC-01_BAAKSearchKeywordTranskripCaseInsensitive", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?q=transkrip")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected at least 1 document matching 'transkrip', got 0")
		}
		for _, doc := range resp.Data {
			titleLower := strings.ToLower(doc.Title)
			origLower := strings.ToLower(doc.OriginalName)
			if !strings.Contains(titleLower, "transkrip") && !strings.Contains(origLower, "transkrip") {
				t.Errorf("document ID %d ('%s', file '%s') does not contain 'transkrip'", doc.ID, doc.Title, doc.OriginalName)
			}
		}
	})

	// TC-SRC-02: BAK: ?prodi=SI -> 200; seluruh hasil berprodi SI
	t.Run("TC-SRC-02_BAAKFilterProdiSI", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?prodi=SI")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected documents with prodi SI, got 0")
		}
		for _, doc := range resp.Data {
			if doc.Prodi == nil || doc.Prodi.Code != "SI" {
				t.Errorf("document ID %d has non-SI prodi: %+v", doc.ID, doc.Prodi)
			}
		}
	})

	// TC-SRC-03: BAK: ?uploader_role=dosen -> 200; seluruh pengunggah berperan Dosen
	t.Run("TC-SRC-03_BAAKFilterUploaderRoleDosen", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?uploader_role=dosen")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected documents uploaded by dosen, got 0")
		}
		for _, doc := range resp.Data {
			if doc.Uploader.Role != "dosen" {
				t.Errorf("document ID %d has uploader role '%s', expected 'dosen'", doc.ID, doc.Uploader.Role)
			}
		}
	})

	// TC-SRC-04: BAK: ?category_id={Legalisir} -> 200; seluruh hasil berkategori Legalisir
	t.Run("TC-SRC-04_BAAKFilterCategoryIDLegalisir", func(t *testing.T) {
		// Category ID 4 = Legalisir
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?category_id=4")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected documents with category Legalisir, got 0")
		}
		for _, doc := range resp.Data {
			if doc.Category.ID != 4 || doc.Category.Name != "Legalisir" {
				t.Errorf("document ID %d category is %d (%s), expected 4 (Legalisir)", doc.ID, doc.Category.ID, doc.Category.Name)
			}
		}
	})

	// TC-SRC-05: BAK: ?nim={NIM valid} -> 200; hanya berkas milik mahasiswa dengan NIM tersebut (termasuk yang diunggah BAAK atas namanya)
	t.Run("TC-SRC-05_BAAKFilterValidNIMIncludesSubjectUploads", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?nim=11S20001")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected documents for NIM 11S20001, got 0")
		}

		foundBAAKUpload := false
		foundSelfUpload := false
		for _, doc := range resp.Data {
			if doc.Owner.NIM == nil || *doc.Owner.NIM != "11S20001" {
				t.Errorf("document ID %d owner NIM is %v, expected 11S20001", doc.ID, doc.Owner.NIM)
			}
			if doc.Uploader.ID == 3 {
				foundBAAKUpload = true
			}
			if doc.Uploader.ID == 8 {
				foundSelfUpload = true
			}
		}

		if !foundBAAKUpload {
			t.Errorf("expected to find documents uploaded by BAAK on behalf of student NIM 11S20001")
		}
		if !foundSelfUpload {
			t.Errorf("expected to find documents uploaded by student 11S20001 self")
		}
	})

	// TC-SRC-06: KPR: ?nip={NIP dosen valid} -> 200; hanya berkas milik dosen tersebut (se-fakultas)
	t.Run("TC-SRC-06_KaprodiFilterNIPFacultyWideLecturers", func(t *testing.T) {
		// Kaprodi IF queries DSN-IF NIP (own prodi)
		code1, resp1, err := doSearchRequest(ctx.Engine, ctx.Tokens["kaprodi.if@del.ac.id"], "/api/v1/documents?nip=198505052010011002")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code1 != http.StatusOK {
			t.Fatalf("expected HTTP 200 for Kaprodi IF searching Dosen IF NIP, got %d", code1)
		}
		if len(resp1.Data) == 0 {
			t.Fatalf("expected documents for DSN-IF, got 0")
		}
		for _, doc := range resp1.Data {
			if doc.Owner.NIP == nil || *doc.Owner.NIP != "198505052010011002" {
				t.Errorf("document ID %d owner NIP is %v, expected 198505052010011002", doc.ID, doc.Owner.NIP)
			}
		}

		// Kaprodi IF queries DSN-SI NIP (other prodi dosen - permitted because Kaprodi can read all faculty lecturers)
		code2, resp2, err := doSearchRequest(ctx.Engine, ctx.Tokens["kaprodi.if@del.ac.id"], "/api/v1/documents?nip=198707072012011004")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code2 != http.StatusOK {
			t.Fatalf("expected HTTP 200 for Kaprodi IF searching Dosen SI NIP, got %d", code2)
		}
		if len(resp2.Data) == 0 {
			t.Fatalf("expected documents for DSN-SI visible to Kaprodi IF, got 0")
		}
		for _, doc := range resp2.Data {
			if doc.Owner.NIP == nil || *doc.Owner.NIP != "198707072012011004" {
				t.Errorf("document ID %d owner NIP is %v, expected 198707072012011004", doc.ID, doc.Owner.NIP)
			}
		}
	})

	// TC-SRC-07: BAK: ?extension=pdf dan ?extension=xlsx -> 200; hasil sesuai ekstensi
	t.Run("TC-SRC-07_BAAKFilterExtensionPDFAndXLSX", func(t *testing.T) {
		// 1. Filter extension=pdf
		codePDF, respPDF, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?extension=pdf")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if codePDF != http.StatusOK {
			t.Fatalf("expected HTTP 200 for extension=pdf, got %d", codePDF)
		}
		if len(respPDF.Data) == 0 {
			t.Fatalf("expected at least 1 pdf document, got 0")
		}
		for _, doc := range respPDF.Data {
			if doc.Extension != "pdf" {
				t.Errorf("expected extension pdf, got '%s' on doc ID %d", doc.Extension, doc.ID)
			}
		}

		// 2. Filter extension=xlsx
		codeXLSX, respXLSX, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?extension=xlsx")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if codeXLSX != http.StatusOK {
			t.Fatalf("expected HTTP 200 for extension=xlsx, got %d", codeXLSX)
		}
		if len(respXLSX.Data) == 0 {
			t.Fatalf("expected at least 1 xlsx document, got 0")
		}
		for _, doc := range respXLSX.Data {
			if doc.Extension != "xlsx" {
				t.Errorf("expected extension xlsx, got '%s' on doc ID %d", doc.Extension, doc.ID)
			}
		}
	})

	// TC-SRC-08: BAK: ?visibility=public_approved -> 200; hanya berkas Public berstatus Approved
	t.Run("TC-SRC-08_BAAKFilterVisibilityPublicApprovedOnly", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?visibility=public_approved")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected documents with visibility=public_approved, got 0")
		}
		for _, doc := range resp.Data {
			if doc.Visibility != "public" {
				t.Errorf("doc ID %d has visibility '%s', expected 'public'", doc.ID, doc.Visibility)
			}
			if doc.VerificationStatus == nil || *doc.VerificationStatus != "approved" {
				t.Errorf("doc ID %d has verification_status '%v', expected 'approved'", doc.ID, doc.VerificationStatus)
			}
		}
	})

	// TC-SRC-09: BAK: ?visibility=private -> 200; hanya berkas Private
	t.Run("TC-SRC-09_BAAKFilterVisibilityPrivateOnly", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?visibility=private")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected documents with visibility=private, got 0")
		}
		for _, doc := range resp.Data {
			if doc.Visibility != "private" {
				t.Errorf("doc ID %d has visibility '%s', expected 'private'", doc.ID, doc.Visibility)
			}
		}
	})

	// TC-SRC-10: BAK: ?prodi=IF&category_id={Transkrip}&extension=pdf&q=2024 -> 200; logika AND
	t.Run("TC-SRC-10_BAAKFilterMultiCriteriaANDLogic", func(t *testing.T) {
		// Category 5 = Transkrip Nilai Akademik
		path := "/api/v1/documents?prodi=IF&category_id=5&extension=pdf&q=2024"
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], path)
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected at least 1 document matching all 4 criteria, got 0")
		}

		for _, doc := range resp.Data {
			if doc.Prodi == nil || doc.Prodi.Code != "IF" {
				t.Errorf("doc ID %d prodi is not IF", doc.ID)
			}
			if doc.Category.ID != 5 {
				t.Errorf("doc ID %d category is not 5", doc.ID)
			}
			if doc.Extension != "pdf" {
				t.Errorf("doc ID %d extension is not pdf", doc.ID)
			}
			tLower := strings.ToLower(doc.Title)
			oLower := strings.ToLower(doc.OriginalName)
			if !strings.Contains(tLower, "2024") && !strings.Contains(oLower, "2024") {
				t.Errorf("doc ID %d title/orig does not contain '2024'", doc.ID)
			}
		}

		// Verify contrasting documents are excluded (proving AND logic):
		// Doc 102 has prodi=SI (excluded)
		// Doc 111 has category=4 (excluded)
		// Doc 112 has extension=docx (excluded)
		// Doc 113 has q=2023 without 2024 (excluded)
		for _, doc := range resp.Data {
			if doc.ID == 102 || doc.ID == 111 || doc.ID == 112 || doc.ID == 113 {
				t.Errorf("contrast document ID %d unexpectedly matched all criteria", doc.ID)
			}
		}
	})

	// TC-SRC-11: BAK: ?q=zzzzzz_tidak_ada -> 200; data = []; meta.total = 0
	t.Run("TC-SRC-11_BAAKSearchNonExistentKeywordEmptyDataZeroTotal", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?q=zzzzzz_tidak_ada")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) != 0 {
			t.Errorf("expected data = [], got %d items", len(resp.Data))
		}
		if resp.Meta.Total != 0 {
			t.Errorf("expected meta.total = 0, got %d", resp.Meta.Total)
		}
	})

	// TC-SRC-12: MHS-IF-1: ?nim={NIM MHS-IF-2} -> 200 dengan hasil kosong; tidak ada berkas milik MHS-IF-2 yang bocor
	t.Run("TC-SRC-12_MahasiswaSearchOtherStudentNIMNoLeakage", func(t *testing.T) {
		// MHS-IF-1 queries for MHS-IF-2's NIM (11S20002)
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["budi@del.ac.id"], "/api/v1/documents?nim=11S20002")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) != 0 {
			t.Errorf("data leakage: Mahasiswa IF-1 received %d documents for another student's NIM", len(resp.Data))
		}
		if resp.Meta.Total != 0 {
			t.Errorf("expected meta.total = 0, got %d", resp.Meta.Total)
		}
	})

	// TC-SRC-13: DSN-IF: ?prodi=SI -> Tidak ada berkas mahasiswa SI dalam hasil (tetap dibatasi hak akses)
	t.Run("TC-SRC-13_DosenSearchOtherProdiNoStudentLeakage", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["dosen.if@del.ac.id"], "/api/v1/documents?prodi=SI")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		// Dosen IF must NOT see any Mahasiswa SI documents
		for _, doc := range resp.Data {
			if doc.Owner.Role == "mahasiswa" {
				t.Errorf("RBAC breach: Dosen IF received document ID %d owned by student in another prodi: %+v", doc.ID, doc.Owner)
			}
		}
	})

	// TC-SRC-14: DKN: ?prodi=TE -> 200; berkas TE tampil (Dekan berhak baca semua)
	t.Run("TC-SRC-14_DekanReadFacultyWideDocuments", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["dekan@del.ac.id"], "/api/v1/documents?prodi=TE")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		if len(resp.Data) == 0 {
			t.Fatalf("expected Dekan to see documents from prodi TE, got 0")
		}
		for _, doc := range resp.Data {
			if doc.Prodi == nil || doc.Prodi.Code != "TE" {
				t.Errorf("doc ID %d has non-TE prodi: %+v", doc.ID, doc.Prodi)
			}
		}
	})

	// TC-SRC-15: BAK: ?page=2&per_page=20 -> 200; meta memuat current_page, per_page, total, last_page; tidak ada duplikasi
	t.Run("TC-SRC-15_BAAKPaginationMetaAndNoOverlap", func(t *testing.T) {
		// Fetch page 1
		code1, resp1, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?page=1&per_page=20")
		if err != nil || code1 != http.StatusOK {
			t.Fatalf("page 1 failed: code %d, err %v", code1, err)
		}
		// Fetch page 2
		code2, resp2, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?page=2&per_page=20")
		if err != nil || code2 != http.StatusOK {
			t.Fatalf("page 2 failed: code %d, err %v", code2, err)
		}

		if resp2.Meta.CurrentPage != 2 {
			t.Errorf("expected current_page 2, got %d", resp2.Meta.CurrentPage)
		}
		if resp2.Meta.PerPage != 20 {
			t.Errorf("expected per_page 20, got %d", resp2.Meta.PerPage)
		}
		if resp2.Meta.Total < 200 {
			t.Errorf("expected total >= 200, got %d", resp2.Meta.Total)
		}
		expectedLastPage := int(math.Ceil(float64(resp2.Meta.Total) / 20.0))
		if resp2.Meta.LastPage != expectedLastPage {
			t.Errorf("expected last_page %d, got %d", expectedLastPage, resp2.Meta.LastPage)
		}

		// Check for ID duplication between page 1 and page 2
		page1IDs := make(map[uint]bool)
		for _, doc := range resp1.Data {
			page1IDs[doc.ID] = true
		}
		for _, doc := range resp2.Data {
			if page1IDs[doc.ID] {
				t.Errorf("duplicate document ID %d found across page 1 and page 2", doc.ID)
			}
		}
	})

	// TC-SRC-16: BAK: ?per_page=100000 dan ?per_page=0 -> 422 (batas 1 sampai 100)
	t.Run("TC-SRC-16_BAAKPerPageBoundariesZeroAndTooLargeRejected422", func(t *testing.T) {
		codeTooLarge, respTooLarge, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?per_page=100000")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if codeTooLarge != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for per_page=100000, got %d", codeTooLarge)
		}
		if respTooLarge.Code != "validation_error" || respTooLarge.Errors["per_page"] == nil {
			t.Errorf("expected validation_error on per_page, got code=%s, errors=%v", respTooLarge.Code, respTooLarge.Errors)
		}

		codeZero, respZero, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?per_page=0")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if codeZero != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for per_page=0, got %d", codeZero)
		}
		if respZero.Code != "validation_error" || respZero.Errors["per_page"] == nil {
			t.Errorf("expected validation_error on per_page, got code=%s, errors=%v", respZero.Code, respZero.Errors)
		}
	})

	// TC-SRC-17: BAK: ?q=' OR '1'='1 dan ?q=%27;DROP TABLE documents;-- -> 200 atau 422; query terparameter
	t.Run("TC-SRC-17_SQLInjectionSanitizationNoSyntaxErrorTableIntact", func(t *testing.T) {
		injections := []string{
			"/api/v1/documents?q=" + url.QueryEscape("' OR '1'='1"),
			"/api/v1/documents?q=" + url.QueryEscape("';DROP TABLE documents;--"),
		}
		for _, path := range injections {
			code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], path)
			if err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			if code != http.StatusOK && code != http.StatusUnprocessableEntity {
				t.Errorf("path %s returned unexpected HTTP %d: %s", path, code, resp.Message)
			}
			// Verify documents table is still intact and readable
			var docCount int64
			if err := ctx.DB.Model(&domain.Document{}).Count(&docCount).Error; err != nil {
				t.Fatalf("SQL injection damaged database: %v", err)
			}
			if docCount < 200 {
				t.Fatalf("table altered or truncated! expected >= 200, got %d", docCount)
			}
		}
	})

	// TC-SRC-18: BAK: ?prodi=XX (nilai di luar enum) -> 422
	t.Run("TC-SRC-18_InvalidProdiEnumRejected422", func(t *testing.T) {
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?prodi=XX")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusUnprocessableEntity {
			t.Fatalf("expected HTTP 422 for prodi=XX, got %d", code)
		}
		if resp.Code != "validation_error" || resp.Errors["prodi"] == nil {
			t.Errorf("expected validation_error on prodi field, got: code=%s, errors=%v", resp.Code, resp.Errors)
		}
	})

	// TC-SRC-19: Berkas yang di-soft delete -> Tidak muncul pada hasil pencarian pengguna biasa
	t.Run("TC-SRC-19_SoftDeletedDocumentExcludedFromNormalSearch", func(t *testing.T) {
		// Document ID 117 is soft-deleted
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?q="+url.QueryEscape("Dokumen Yang Sudah Di Soft Delete"))
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code != http.StatusOK {
			t.Fatalf("expected HTTP 200, got %d", code)
		}
		for _, doc := range resp.Data {
			if doc.ID == 117 {
				t.Errorf("soft deleted document ID 117 unexpectedly appeared in search results")
			}
		}
	})

	// TC-SRC-20: Pencarian kombinasi pada 100.000 baris metadata (data uji beban) -> Waktu respons <= 1 detik
	t.Run("TC-SRC-20_SearchPerformanceAndExecutionPlanVerification", func(t *testing.T) {
		// Benchmark query latency on the test database
		start := time.Now()
		code, resp, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?prodi=IF&category_id=5&extension=pdf&page=1&per_page=20")
		duration := time.Since(start)

		if err != nil || code != http.StatusOK {
			t.Fatalf("query failed: code %d, err %v", code, err)
		}
		if duration > 1*time.Second {
			t.Errorf("query exceeded SLA-RT-02 limit (1s): took %v", duration)
		}
		t.Logf("Query execution time on %d documents: %v (meta total=%d)", ctx.TotalDocsSeed, duration, resp.Meta.Total)

		// Verification of index utilization via EXPLAIN
		var explainRows []struct {
			Detail string `gorm:"column:detail"`
		}
		err = ctx.DB.Raw("EXPLAIN QUERY PLAN SELECT * FROM documents WHERE prodi_id = 1 AND category_id = 5 AND extension = 'pdf' AND deleted_at IS NULL").Scan(&explainRows).Error
		if err == nil && len(explainRows) > 0 {
			t.Logf("Query plan verified: %s", explainRows[0].Detail)
		}
	})

	// TC-SRC-21: BAK: ?q=100% dan ?q=a_b -> Karakter % dan _ diperlakukan sebagai teks biasa
	t.Run("TC-SRC-21_WildcardCharactersTreatedAsLiteral", func(t *testing.T) {
		// 1. Query q=100%
		code1, resp1, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?q=100%25")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code1 != http.StatusOK {
			t.Fatalf("expected HTTP 200 for q=100%%, got %d", code1)
		}
		if len(resp1.Data) == 0 {
			t.Fatalf("expected to find document with literal '100%%', got 0")
		}
		for _, doc := range resp1.Data {
			if !strings.Contains(doc.Title, "100%") && !strings.Contains(doc.OriginalName, "100%") {
				t.Errorf("doc ID %d ('%s') matched wildcard instead of literal '100%%'", doc.ID, doc.Title)
			}
			if doc.ID == 119 { // Doc 119 has '1000'
				t.Errorf("doc ID 119 ('1000') should NOT match query for '100%%'")
			}
		}

		// 2. Query q=a_b
		code2, resp2, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?q=a_b")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code2 != http.StatusOK {
			t.Fatalf("expected HTTP 200 for q=a_b, got %d", code2)
		}
		if len(resp2.Data) == 0 {
			t.Fatalf("expected to find document with literal 'a_b', got 0")
		}
		for _, doc := range resp2.Data {
			if !strings.Contains(doc.Title, "a_b") && !strings.Contains(doc.OriginalName, "a_b") {
				t.Errorf("doc ID %d ('%s') matched wildcard instead of literal 'a_b'", doc.ID, doc.Title)
			}
			if doc.ID == 121 { // Doc 121 has 'axb'
				t.Errorf("doc ID 121 ('axb') should NOT match query for literal 'a_b'")
			}
		}
	})

	// TC-SRC-22: BAK: ?sort=-size_kb, lalu ?sort=password -> Pertama: 200 urut menurun; kedua: 422
	t.Run("TC-SRC-22_SortWhitelistedOrderAndInvalidSortRejected422", func(t *testing.T) {
		// Valid sort: -size_kb
		code1, resp1, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?sort=-size_kb&per_page=50")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code1 != http.StatusOK {
			t.Fatalf("expected HTTP 200 for sort=-size_kb, got %d", code1)
		}
		if len(resp1.Data) < 2 {
			t.Fatalf("expected at least 2 documents to test sort order, got %d", len(resp1.Data))
		}
		for i := 0; i < len(resp1.Data)-1; i++ {
			if resp1.Data[i].SizeKB < resp1.Data[i+1].SizeKB {
				t.Errorf("sort order violation at index %d: size %d < size %d", i, resp1.Data[i].SizeKB, resp1.Data[i+1].SizeKB)
			}
		}

		// Invalid sort: password
		code2, resp2, err := doSearchRequest(ctx.Engine, ctx.Tokens["baak@del.ac.id"], "/api/v1/documents?sort=password")
		if err != nil {
			t.Fatalf("unmarshal error: %v", err)
		}
		if code2 != http.StatusUnprocessableEntity {
			t.Fatalf("expected HTTP 422 for sort=password, got %d", code2)
		}
		if resp2.Code != "validation_error" || resp2.Errors["sort"] == nil {
			t.Errorf("expected validation_error on sort field, got: code=%s, errors=%v", resp2.Code, resp2.Errors)
		}
	})

	// Additional verification: GET /api/v1/documents/public endpoint
	t.Run("PublicExplorer_AuthenticationAndFilterRules", func(t *testing.T) {
		// 1. Unauthenticated request to /documents/public MUST return 401
		codeUnauth, _, _ := doSearchRequest(ctx.Engine, "", "/api/v1/documents/public")
		if codeUnauth != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized for public documents without Bearer token, got %d", codeUnauth)
		}

		// 2. Authenticated request to /documents/public returns ONLY approved public documents
		codeAuth, respAuth, err := doSearchRequest(ctx.Engine, ctx.Tokens["budi@del.ac.id"], "/api/v1/documents/public")
		if err != nil || codeAuth != http.StatusOK {
			t.Fatalf("authenticated public explorer query failed: code %d, err %v", codeAuth, err)
		}
		for _, doc := range respAuth.Data {
			if doc.Visibility != "public" {
				t.Errorf("non-public document in public explorer: ID %d visibility=%s", doc.ID, doc.Visibility)
			}
			if doc.VerificationStatus == nil || *doc.VerificationStatus != "approved" {
				t.Errorf("unapproved document in public explorer: ID %d status=%v", doc.ID, doc.VerificationStatus)
			}
		}
	})
}
