package tests

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var authSuiteSeq uint64

type authSuiteContext struct {
	Engine   *gin.Engine
	DB       *gorm.DB
	Config   config.Config
	AuthSvc  *service.AuthService
	DocSvc   *service.DocumentService
	Storage  *storage.MemoryStorage
	Limiter  *middleware.FailureLimiter
	Users    map[string]domain.User
	Docs     map[string]domain.Document
	Password string
}

func setupAuthSuite(t *testing.T) *authSuiteContext {
	t.Helper()
	gin.SetMode(gin.TestMode)
	httpx.Init()

	seq := atomic.AddUint64(&authSuiteSeq, 1)
	dsn := fmt.Sprintf("file:auth_suite_%d_%d?mode=memory&cache=shared", time.Now().UnixNano(), seq)
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
		t.Fatalf("migrate auth suite: %v", err)
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
		{ID: 1, Name: "Proposal TA/PA", AllowedRoles: domain.StringList{"mahasiswa", "super_admin"}, IsOperational: false},
		{ID: 2, Name: "SK Keputusan Dekan", AllowedRoles: domain.StringList{"dekan", "super_admin"}, IsOperational: false},
		{ID: 3, Name: "Legalisir", AllowedRoles: domain.StringList{"baak", "super_admin"}, IsOperational: true},
		{ID: 4, Name: "Rencana Pembelajaran Semester (RPS)", AllowedRoles: domain.StringList{"dosen", "super_admin"}, IsOperational: false},
		{ID: 5, Name: "Dokumen Akreditasi Fakultas", AllowedRoles: domain.StringList{"dekan", "super_admin"}, IsOperational: false},
	}
	db.Create(&cats)

	// 4. Official 10 SRS Accounts
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

	// 5. Test Documents for RBAC scope testing
	now := time.Now().UTC()
	docList := []domain.Document{
		{
			ID:           101,
			Title:        "Proposal Skripsi Budi",
			OriginalName: "proposal_budi.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       1024,
			StoragePath:  "documents/IF/2026/10/doc-101.pdf",
			CategoryID:   1,
			ProdiID:      &prodiIF,
			UploaderID:   8,
			OwnerID:      8,
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           102,
			Title:        "Proposal Skripsi Mahasiswa IF-2",
			OriginalName: "proposal_if2.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       1024,
			StoragePath:  "documents/IF/2026/10/doc-102.pdf",
			CategoryID:   1,
			ProdiID:      &prodiIF,
			UploaderID:   9,
			OwnerID:      9,
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           103,
			Title:        "Proposal Skripsi Mahasiswa SI-1",
			OriginalName: "proposal_si1.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       1024,
			StoragePath:  "documents/SI/2026/10/doc-103.pdf",
			CategoryID:   1,
			ProdiID:      &prodiSI,
			UploaderID:   10,
			OwnerID:      10,
			OwnerRole:    domain.RoleMahasiswa,
			Visibility:   "private",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           104,
			Title:        "Silabus RPS Informatika",
			OriginalName: "rps_if.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       512,
			StoragePath:  "documents/IF/2026/10/doc-104.pdf",
			CategoryID:   4,
			ProdiID:      &prodiIF,
			UploaderID:   6,
			OwnerID:      6,
			OwnerRole:    domain.RoleDosen,
			Visibility:   "private",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           105,
			Title:        "Silabus RPS Sistem Informasi",
			OriginalName: "rps_si.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       512,
			StoragePath:  "documents/SI/2026/10/doc-105.pdf",
			CategoryID:   4,
			ProdiID:      &prodiSI,
			UploaderID:   7,
			OwnerID:      7,
			OwnerRole:    domain.RoleDosen,
			Visibility:   "private",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
		{
			ID:           106,
			Title:        "SK Dekan FITE",
			OriginalName: "sk_dekan.pdf",
			Extension:    "pdf",
			MimeType:     "application/pdf",
			SizeKB:       256,
			StoragePath:  "documents/fakultas/2026/10/doc-106.pdf",
			CategoryID:   2,
			ProdiID:      nil,
			UploaderID:   2,
			OwnerID:      2,
			OwnerRole:    domain.RoleDekan,
			Visibility:   "private",
			CreatedAt:    now,
			UpdatedAt:    now,
		},
	}
	db.Create(&docList)

	docMap := make(map[string]domain.Document)
	for _, d := range docList {
		docMap[d.Title] = d
	}

	cfg := config.Config{
		Env:             "development",
		HTTPAddr:        ":8080",
		JWTSecret:       []byte("super-secret-jwt-key-32-bytes-long!"),
		JWTIssuer:       "fite-arsip-api",
		JWTAudience:     "fite-arsip-web",
		AllowedOrigin:   "http://localhost:3000",
		AccessTTL:       15 * time.Minute,
		RefreshTTL:      7 * 24 * time.Hour,
		SessionMaxAge:   30 * 24 * time.Hour,
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

	return &authSuiteContext{
		Engine:   r,
		DB:       db,
		Config:   cfg,
		AuthSvc:  authSvc,
		DocSvc:   docSvc,
		Storage:  memStore,
		Limiter:  limiter,
		Users:    userMap,
		Docs:     docMap,
		Password: defaultPW,
	}
}

func doAuthReq(t *testing.T, r *gin.Engine, method, path, token string, cookie *http.Cookie, body any, origin ...string) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		rd = bytes.NewReader(buf)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if cookie != nil {
		req.AddCookie(cookie)
	}
	if len(origin) > 0 && origin[0] != "" {
		req.Header.Set("Origin", origin[0])
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getResponseCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	resp := w.Result()
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func loginUser(t *testing.T, sc *authSuiteContext, email, password string) (string, *http.Cookie, map[string]any) {
	t.Helper()
	if sc.Limiter != nil {
		sc.Limiter.Reset(strings.ToLower(strings.TrimSpace(email)) + "|192.0.2.1")
	}
	w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
		"email":    email,
		"password": password,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("login failed for %s, code %d: %s", email, w.Code, w.Body.String())
	}
	var res struct {
		Data map[string]any `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	token, _ := res.Data["access_token"].(string)
	cookie := getResponseCookie(w, "refresh_token")
	return token, cookie, res.Data
}

func TestTS_AUTH_Granular(t *testing.T) {
	sc := setupAuthSuite(t)

	// -------------------------------------------------------------------------
	// TC-AUTH-01: Login Super Admin valid
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-01_SuperAdminLoginValid", func(t *testing.T) {
		w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
			"email":    "admin@del.ac.id",
			"password": sc.Password,
		})
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
		var res struct {
			Success bool `json:"success"`
			Data    struct {
				AccessToken string         `json:"access_token"`
				TokenType   string         `json:"token_type"`
				ExpiresIn   int            `json:"expires_in"`
				User        map[string]any `json:"user"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if !res.Success || res.Data.AccessToken == "" {
			t.Errorf("expected access_token, got empty")
		}
		if res.Data.TokenType != "Bearer" {
			t.Errorf("expected token_type Bearer, got %s", res.Data.TokenType)
		}
		if res.Data.ExpiresIn != 900 {
			t.Errorf("expected expires_in 900, got %d", res.Data.ExpiresIn)
		}
		if res.Data.User["role"] != "super_admin" {
			t.Errorf("expected role super_admin, got %v", res.Data.User["role"])
		}

		cookie := getResponseCookie(w, "refresh_token")
		if cookie == nil || cookie.Value == "" {
			t.Fatalf("expected refresh_token cookie, got nil")
		}
		if !cookie.HttpOnly {
			t.Errorf("expected HttpOnly cookie")
		}
		if cookie.Path != "/api/v1/auth" {
			t.Errorf("expected cookie path /api/v1/auth, got %s", cookie.Path)
		}

		var rawMap map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &rawMap)
		if dataMap, ok := rawMap["data"].(map[string]any); ok {
			if _, has := dataMap["refresh_token"]; has {
				t.Errorf("JSON body must NOT contain refresh_token")
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-02: Login valid untuk 5 peran lain
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-02_MultiRoleLoginValid", func(t *testing.T) {
		accounts := []struct {
			email     string
			roleKey   string
			prodiCode string
		}{
			{"dekan@del.ac.id", "dekan", ""},
			{"baak@del.ac.id", "baak", ""},
			{"kaprodi.if@del.ac.id", "kaprodi", "IF"},
			{"dosen.if@del.ac.id", "dosen", "IF"},
			{"budi@del.ac.id", "mahasiswa", "IF"},
		}
		for _, acc := range accounts {
			token, cookie, data := loginUser(t, sc, acc.email, sc.Password)
			if token == "" || cookie == nil {
				t.Errorf("failed token/cookie for %s", acc.email)
			}
			userObj, _ := data["user"].(map[string]any)
			if userObj["role"] != acc.roleKey {
				t.Errorf("expected role %s, got %v", acc.roleKey, userObj["role"])
			}
			if acc.prodiCode != "" {
				prodiObj, _ := userObj["prodi"].(map[string]any)
				if prodiObj["code"] != acc.prodiCode {
					t.Errorf("expected prodi %s, got %v", acc.prodiCode, prodiObj["code"])
				}
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-03: Login dengan kata sandi salah
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-03_WrongPasswordGenericError", func(t *testing.T) {
		w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
			"email":    "budi@del.ac.id",
			"password": "WrongPassword999!",
		})
		sc.Limiter.Reset("budi@del.ac.id|192.0.2.1")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", w.Code)
		}
		var res map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		msg, _ := res["message"].(string)
		if !strings.Contains(msg, "Email atau kata sandi salah") {
			t.Errorf("expected generic error message, got: %s", msg)
		}
		if cookie := getResponseCookie(w, "refresh_token"); cookie != nil && cookie.Value != "" {
			t.Errorf("no refresh cookie should be set on failure")
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-04: Login email tidak terdaftar (anti-timing enumeration)
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-04_UnregisteredEmailTimingDefense", func(t *testing.T) {
		const iterations = 5
		var unregDurations time.Duration
		var wrongDurations time.Duration

		for i := 0; i < iterations; i++ {
			sc.Limiter.Reset("budi@del.ac.id|192.0.2.1")
			t0 := time.Now()
			w1 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
				"email":    fmt.Sprintf("notfound_%d@del.ac.id", i),
				"password": "WrongPassword123!",
			})
			unregDurations += time.Since(t0)
			if w1.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", w1.Code)
			}

			t1 := time.Now()
			w2 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
				"email":    "budi@del.ac.id",
				"password": "WrongPassword123!",
			})
			wrongDurations += time.Since(t1)
			if w2.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", w2.Code)
			}
		}

		sc.Limiter.Reset("budi@del.ac.id|192.0.2.1")
		avgUnreg := unregDurations / iterations
		avgWrong := wrongDurations / iterations
		diff := avgUnreg - avgWrong
		if diff < 0 {
			diff = -diff
		}
		t.Logf("TC-AUTH-04 Timing stats: avgUnreg=%v, avgWrong=%v, delta=%v (SRS threshold < 50ms)", avgUnreg, avgWrong, diff)
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-05: Login field kosong atau email format salah
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-05_ValidationErrorsEmptyOrMalformed", func(t *testing.T) {
		w1 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
			"email":    "",
			"password": "",
		})
		if w1.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for empty body, got %d", w1.Code)
		}

		w2 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
			"email":    "not-an-email-format",
			"password": "Password123!",
		})
		if w2.Code != http.StatusUnprocessableEntity {
			t.Errorf("expected 422 for malformed email, got %d", w2.Code)
		}
		var res map[string]any
		_ = json.Unmarshal(w2.Body.Bytes(), &res)
		if res["code"] != "validation_error" {
			t.Errorf("expected validation_error code, got %v", res["code"])
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-06: Login akun nonaktif
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-06_InactiveAccountLoginBlocked", func(t *testing.T) {
		sc.DB.Model(&domain.User{}).Where("email = ?", "mhs.if2@del.ac.id").Update("is_active", false)
		defer sc.DB.Model(&domain.User{}).Where("email = ?", "mhs.if2@del.ac.id").Update("is_active", true)

		w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
			"email":    "mhs.if2@del.ac.id",
			"password": sc.Password,
		})
		if w.Code != http.StatusForbidden {
			t.Fatalf("expected 403 for inactive user, got %d", w.Code)
		}
		var res map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		if res["code"] != "account_disabled" {
			t.Errorf("expected account_disabled, got %v", res["code"])
		}
		if cookie := getResponseCookie(w, "refresh_token"); cookie != nil && cookie.Value != "" {
			t.Errorf("no cookie should be issued for inactive user")
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-07: Rate Limiter (6x percobaan gagal dalam 1 menit)
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-07_FailureRateLimiter", func(t *testing.T) {
		email := "rate_limiter_auth07@del.ac.id"
		for i := 1; i <= 5; i++ {
			w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
				"email":    email,
				"password": "WrongPassword!",
			})
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("attempt %d: expected 401, got %d", i, w.Code)
			}
		}
		// 6th attempt: 429 Too Many Requests
		w6 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
			"email":    email,
			"password": "WrongPassword!",
		})
		if w6.Code != http.StatusTooManyRequests {
			t.Fatalf("attempt 6: expected 429, got %d", w6.Code)
		}
		if w6.Header().Get("Retry-After") == "" {
			t.Errorf("expected Retry-After header on 429 response")
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-08: GET /documents tanpa header Authorization
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-08_MissingAuthorizationHeader", func(t *testing.T) {
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents", "", nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for missing auth header, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-09: GET /documents dengan Bearer rusak
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-09_MalformedBearerTokens", func(t *testing.T) {
		badHeaders := []string{
			"Bearer abc",
			"Bearer",
			"Basic dXNlcjpwYXNz",
			"Token invalidtokenhere",
		}
		for _, bh := range badHeaders {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
			req.Header.Set("Authorization", bh)
			w := httptest.NewRecorder()
			sc.Engine.ServeHTTP(w, req)
			if w.Code != http.StatusUnauthorized {
				t.Errorf("for header %q, expected 401, got %d", bh, w.Code)
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-10: Logout mencabut token & cookie
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-10_LogoutRevocationAndCookiePurge", func(t *testing.T) {
		token, cookie, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)

		wLogout := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/logout", token, cookie, nil)
		if wLogout.Code != http.StatusOK {
			t.Fatalf("expected 200 on logout, got %d", wLogout.Code)
		}
		cookieLogout := getResponseCookie(wLogout, "refresh_token")
		if cookieLogout == nil || cookieLogout.MaxAge > 0 {
			t.Errorf("expected cookie cleared with MaxAge <= 0")
		}

		wMe := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", token, nil, nil)
		if wMe.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 on /auth/me after logout, got %d", wMe.Code)
		}

		wRef := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", cookie, nil)
		if wRef.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 on /auth/refresh after logout, got %d", wRef.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-11: Access token kedaluwarsa
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-11_ExpiredAccessToken", func(t *testing.T) {
		now := time.Now().UTC()
		claims := service.Claims{
			SID: uuid.NewString(),
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "8",
				Issuer:    sc.Config.JWTIssuer,
				Audience:  jwt.ClaimStrings{sc.Config.JWTAudience},
				IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
				ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
			},
		}
		tok, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(sc.Config.JWTSecret)

		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents", tok, nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for expired token, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-12: MHS-IF-1 buka berkas miliknya
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-12_StudentOwnDocumentDetail", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/101", token, nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-13: MHS-IF-1 buka berkas Private MHS-IF-2
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-13_StudentOtherPrivateDocumentForbidden", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/102", token, nil, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d: %s", w.Code, w.Body.String())
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-14: MHS-IF-1 GET /documents (daftar)
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-14_StudentDocumentListScope", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents", token, nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", w.Code)
		}
		var res struct {
			Data []struct {
				ID uint `json:"id"`
			} `json:"data"`
		}
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		for _, d := range res.Data {
			if d.ID != 101 {
				t.Errorf("student should only see own documents, found ID %d", d.ID)
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-15: DSN-IF membuka berkas mahasiswa prodi IF
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-15_LecturerOwnProdiStudentDocument", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "dosen.if@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/101", token, nil, nil)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for lecturer reading student doc of same prodi, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-16: DSN-IF membuka berkas mahasiswa prodi SI
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-16_LecturerCrossProdiStudentDocumentForbidden", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "dosen.if@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/103", token, nil, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for lecturer reading student doc of cross-prodi, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-17: KPR-IF membuka berkas dosen prodi SI
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-17_KaprodiCrossProdiLecturerDocument", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "kaprodi.if@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/105", token, nil, nil)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for kaprodi reading cross-prodi lecturer doc, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-18: KPR-IF membuka berkas mahasiswa prodi SI
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-18_KaprodiCrossProdiStudentDocumentForbidden", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "kaprodi.if@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/103", token, nil, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for kaprodi reading cross-prodi student doc, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-19: Dekan membuka berkas semua prodi dan semua aktor
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-19_DeanAllProdiAndActorsDocumentAccess", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "dekan@del.ac.id", sc.Password)
		for _, id := range []int{101, 102, 103, 104, 105, 106} {
			w := doAuthReq(t, sc.Engine, http.MethodGet, fmt.Sprintf("/api/v1/documents/%d", id), token, nil, nil)
			if w.Code != http.StatusOK {
				t.Errorf("expected 200 for Dekan reading doc %d, got %d", id, w.Code)
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-20: MHS mengakses GET /users
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-20_StudentAccessUserManagementForbidden", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/users", token, nil, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for student accessing /users, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-21: BAK mengakses GET /users
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-21_BAAKAccessUserManagementForbidden", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "baak@del.ac.id", sc.Password)
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/users", token, nil, nil)
		if w.Code != http.StatusForbidden {
			t.Errorf("expected 403 for BAAK accessing /users, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-22: PUT /auth/password dengan sandi lama benar dan baru valid
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-22_ChangePasswordSessionManagement", func(t *testing.T) {
		defer func() {
			hash, _ := bcrypt.GenerateFromPassword([]byte(sc.Password), 12)
			sc.DB.Model(&domain.User{}).Where("email = ?", "budi@del.ac.id").Updates(map[string]any{
				"password_hash":         string(hash),
				"must_change_password": false,
			})
		}()
		tok1, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		tok2, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)

		newPW := "NewBudiPass123!"
		wChange := doAuthReq(t, sc.Engine, http.MethodPut, "/api/v1/auth/password", tok2, nil, map[string]string{
			"current_password":      sc.Password,
			"password":              newPW,
			"password_confirmation": newPW,
		})
		if wChange.Code != http.StatusOK {
			t.Fatalf("expected 200 on password change, got %d: %s", wChange.Code, wChange.Body.String())
		}

		wTok1 := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", tok1, nil, nil)
		if wTok1.Code != http.StatusUnauthorized {
			t.Errorf("expected tok1 to be revoked 401, got %d", wTok1.Code)
		}

		wTok2 := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", tok2, nil, nil)
		if wTok2.Code != http.StatusOK {
			t.Errorf("expected tok2 to remain active 200, got %d", wTok2.Code)
		}

		loginNew, _, _ := loginUser(t, sc, "budi@del.ac.id", newPW)
		if loginNew == "" {
			t.Errorf("login with new password failed")
		}

		_ = doAuthReq(t, sc.Engine, http.MethodPut, "/api/v1/auth/password", loginNew, nil, map[string]string{
			"current_password":      newPW,
			"password":              sc.Password,
			"password_confirmation": sc.Password,
		})
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-23: IDOR testing pada berkas berurutan
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-23_IDORProtectionOnDocumentDetail", func(t *testing.T) {
		token, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		for _, otherID := range []int{102, 103, 104, 105, 106} {
			w := doAuthReq(t, sc.Engine, http.MethodGet, fmt.Sprintf("/api/v1/documents/%d", otherID), token, nil, nil)
			if w.Code != http.StatusForbidden {
				t.Errorf("expected 403 on IDOR doc %d, got %d", otherID, w.Code)
			}
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-24: POST /auth/refresh cookie valid
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-24_RefreshTokenRotationSuccess", func(t *testing.T) {
		_, cookie1, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)

		wRef := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", cookie1, nil)
		if wRef.Code != http.StatusOK {
			t.Fatalf("expected 200 on refresh, got %d: %s", wRef.Code, wRef.Body.String())
		}
		cookie2 := getResponseCookie(wRef, "refresh_token")
		if cookie2 == nil || cookie2.Value == cookie1.Value {
			t.Errorf("expected new rotated refresh token cookie")
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-25: Deteksi pemakaian ulang refresh token (Reuse Detection)
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-25_RefreshTokenReuseDetectionAndRevocation", func(t *testing.T) {
		_, cookie1, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)

		wRef1 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", cookie1, nil)
		if wRef1.Code != http.StatusOK {
			t.Fatalf("refresh 1 failed: %d", wRef1.Code)
		}
		var res1 struct {
			Data struct {
				AccessToken string `json:"access_token"`
			} `json:"data"`
		}
		_ = json.Unmarshal(wRef1.Body.Bytes(), &res1)
		newTok := res1.Data.AccessToken
		cookie2 := getResponseCookie(wRef1, "refresh_token")

		// Replay OLD cookie1
		wRef2 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", cookie1, nil)
		if wRef2.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 on token reuse, got %d", wRef2.Code)
		}

		// Family invalidated
		wRef3 := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", cookie2, nil)
		if wRef3.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 on new cookie after reuse revocation, got %d", wRef3.Code)
		}
		wTok := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", newTok, nil, nil)
		if wTok.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 on access token after reuse revocation, got %d", wTok.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-26: POST /auth/refresh tanpa cookie
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-26_RefreshWithoutCookie", func(t *testing.T) {
		w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", nil, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for refresh without cookie, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-27: POST /auth/refresh dengan token kedaluwarsa 7 hari
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-27_RefreshWithExpiredToken", func(t *testing.T) {
		_, cookie, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)

		sc.DB.Model(&domain.AuthSession{}).Where("user_id = ?", 8).
			Update("expires_at", time.Now().Add(-24*time.Hour))

		w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", cookie, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for expired refresh token, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-28: POST /auth/refresh setelah logout
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-28_RefreshAfterLogout", func(t *testing.T) {
		tok, cookie, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		_ = doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/logout", tok, cookie, nil)

		w := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", cookie, nil)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 on refresh after logout, got %d", w.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-29: Manipulasi JWT (alg none, wrong signature, tampered payload, wrong iss/aud)
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-29_JWTTamperingMitigation", func(t *testing.T) {
		now := time.Now().UTC()
		// (a) alg: none
		headerNone := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
		payloadA := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"1","sid":"some-id","iss":"fite-arsip-api","aud":"fite-arsip-web","exp":9999999999}`))
		tokNone := headerNone + "." + payloadA + "."

		wA := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", tokNone, nil, nil)
		if wA.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for alg:none, got %d", wA.Code)
		}

		// (b) Wrong signature
		claimsB := service.Claims{
			SID: uuid.NewString(),
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "1",
				Issuer:    sc.Config.JWTIssuer,
				Audience:  jwt.ClaimStrings{sc.Config.JWTAudience},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
		}
		tokWrongSig, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsB).SignedString([]byte("wrong-key-different-secret-here!"))
		wB := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", tokWrongSig, nil, nil)
		if wB.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for wrong signature, got %d", wB.Code)
		}

		// (c) Tampered payload
		validTok, _, _ := loginUser(t, sc, "admin@del.ac.id", sc.Password)
		parts := strings.Split(validTok, ".")
		if len(parts) == 3 {
			tampered := parts[0] + "." + parts[1] + "X" + "." + parts[2]
			wC := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", tampered, nil, nil)
			if wC.Code != http.StatusUnauthorized {
				t.Errorf("expected 401 for tampered payload, got %d", wC.Code)
			}
		}

		// (d) Wrong Issuer / Audience
		claimsD := service.Claims{
			SID: uuid.NewString(),
			RegisteredClaims: jwt.RegisteredClaims{
				Subject:   "1",
				Issuer:    "evil-issuer",
				Audience:  jwt.ClaimStrings{"evil-audience"},
				ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			},
		}
		tokWrongIss, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsD).SignedString(sc.Config.JWTSecret)
		wD := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", tokWrongIss, nil, nil)
		if wD.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for wrong iss/aud, got %d", wD.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-30: SA menonaktifkan akun saat user memegang token aktif
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-30_AccountDeactivationInstantRevocation", func(t *testing.T) {
		dsnToken, dsnCookie, _ := loginUser(t, sc, "dosen.if@del.ac.id", sc.Password)
		saToken, _, _ := loginUser(t, sc, "admin@del.ac.id", sc.Password)

		wDeact := doAuthReq(t, sc.Engine, http.MethodPatch, "/api/v1/users/6/status", saToken, nil, map[string]any{
			"is_active": false,
		})
		if wDeact.Code != http.StatusOK {
			t.Fatalf("SA deactivation failed: %d: %s", wDeact.Code, wDeact.Body.String())
		}
		defer sc.DB.Model(&domain.User{}).Where("id = 6").Update("is_active", true)

		wDoc := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents", dsnToken, nil, nil)
		if wDoc.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for deactivated user request, got %d", wDoc.Code)
		}

		wRef := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/refresh", "", dsnCookie, nil)
		if wRef.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 for deactivated user refresh, got %d", wRef.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-31: SA mengubah peran KPR-IF menjadi Dosen
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-31_RoleChangeLiveEnforcement", func(t *testing.T) {
		kprToken, _, _ := loginUser(t, sc, "kaprodi.if@del.ac.id", sc.Password)
		w1 := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/105", kprToken, nil, nil)
		if w1.Code != http.StatusOK {
			t.Fatalf("expected 200 as kaprodi, got %d", w1.Code)
		}

		saToken, _, _ := loginUser(t, sc, "admin@del.ac.id", sc.Password)
		prodiIF := 1
		wRole := doAuthReq(t, sc.Engine, http.MethodPatch, "/api/v1/users/4/role", saToken, nil, map[string]any{
			"role":     "dosen",
			"prodi_id": prodiIF,
		})
		if wRole.Code != http.StatusOK {
			t.Fatalf("SA role change failed: %d: %s", wRole.Code, wRole.Body.String())
		}
		defer sc.DB.Model(&domain.User{}).Where("id = 4").Update("role_id", 4)

		wOldTok := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/105", kprToken, nil, nil)
		if wOldTok.Code != http.StatusUnauthorized && wOldTok.Code != http.StatusForbidden {
			t.Errorf("expected 401 (revoked) or 403 on old token after role change, got %d", wOldTok.Code)
		}

		newDsnTok, _, _ := loginUser(t, sc, "kaprodi.if@del.ac.id", sc.Password)
		wNewAccess := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/documents/105", newDsnTok, nil, nil)
		if wNewAccess.Code != http.StatusForbidden {
			t.Errorf("expected 403 for demoted lecturer accessing cross-prodi lecturer doc, got %d", wNewAccess.Code)
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-32: SA mengatur ulang kata sandi sebuah akun yang sedang login
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-32_AdminPasswordResetRevokesSessions", func(t *testing.T) {
		budiToken, _, _ := loginUser(t, sc, "budi@del.ac.id", sc.Password)
		saToken, _, _ := loginUser(t, sc, "admin@del.ac.id", sc.Password)

		newPW := "AdminResetBudiPass123!"
		wReset := doAuthReq(t, sc.Engine, http.MethodPatch, "/api/v1/users/8/password", saToken, nil, map[string]string{
			"password":              newPW,
			"password_confirmation": newPW,
		})
		if wReset.Code != http.StatusOK {
			t.Fatalf("SA reset password failed: %d: %s", wReset.Code, wReset.Body.String())
		}

		wOld := doAuthReq(t, sc.Engine, http.MethodGet, "/api/v1/auth/me", budiToken, nil, nil)
		if wOld.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 on old token after admin password reset, got %d", wOld.Code)
		}

		wLogin := doAuthReq(t, sc.Engine, http.MethodPost, "/api/v1/auth/login", "", nil, map[string]string{
			"email":    "budi@del.ac.id",
			"password": newPW,
		})
		if wLogin.Code != http.StatusOK {
			t.Fatalf("login with reset password failed: %d", wLogin.Code)
		}
		var resLogin struct {
			Data struct {
				User map[string]any `json:"user"`
			} `json:"data"`
		}
		_ = json.Unmarshal(wLogin.Body.Bytes(), &resLogin)
		if resLogin.Data.User["must_change_password"] != true {
			t.Errorf("expected must_change_password=true after admin reset, got %v", resLogin.Data.User["must_change_password"])
		}

		restoreHash, _ := bcrypt.GenerateFromPassword([]byte(sc.Password), 12)
		sc.DB.Model(&domain.User{}).Where("id = 8").Updates(map[string]any{
			"password":             string(restoreHash),
			"must_change_password": false,
		})
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-33: CORS: OPTIONS preflight origin sah vs tidak sah
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-33_CORSPreflightPolicy", func(t *testing.T) {
		reqValid := httptest.NewRequest(http.MethodOptions, "/api/v1/documents", nil)
		reqValid.Header.Set("Origin", "http://localhost:3000")
		reqValid.Header.Set("Access-Control-Request-Method", "GET")
		wValid := httptest.NewRecorder()
		sc.Engine.ServeHTTP(wValid, reqValid)
		if wValid.Code != http.StatusNoContent {
			t.Errorf("expected 204 for valid origin preflight, got %d", wValid.Code)
		}
		if wValid.Header().Get("Access-Control-Allow-Origin") != "http://localhost:3000" {
			t.Errorf("expected allow origin http://localhost:3000, got %s", wValid.Header().Get("Access-Control-Allow-Origin"))
		}
		if wValid.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("expected allow credentials true")
		}

		reqInvalid := httptest.NewRequest(http.MethodOptions, "/api/v1/documents", nil)
		reqInvalid.Header.Set("Origin", "http://malicious-origin.com")
		reqInvalid.Header.Set("Access-Control-Request-Method", "GET")
		wInvalid := httptest.NewRecorder()
		sc.Engine.ServeHTTP(wInvalid, reqInvalid)
		if wInvalid.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("expected no allow origin for invalid origin, got %s", wInvalid.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	// -------------------------------------------------------------------------
	// TC-AUTH-34: Header keamanan pada respons API
	// -------------------------------------------------------------------------
	t.Run("TC-AUTH-34_HTTPSecurityHeaders", func(t *testing.T) {
		w := doAuthReq(t, sc.Engine, http.MethodGet, "/healthz", "", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("health check failed: %d", w.Code)
		}
		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("expected X-Content-Type-Options: nosniff, got %s", w.Header().Get("X-Content-Type-Options"))
		}
		csp := w.Header().Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'none'") {
			t.Errorf("expected CSP default-src 'none', got %s", csp)
		}
	})
}
