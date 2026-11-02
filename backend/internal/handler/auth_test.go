package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/handler"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func setupHandlerTestEnv(t *testing.T) (*gin.Engine, *gorm.DB, *service.AuthService, config.Config) {
	gin.SetMode(gin.TestMode)
	httpx.Init()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
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

	cfg := config.Config{
		JWTSecret:     []byte("super-secret-key-that-is-at-least-32-bytes-long!"),
		JWTIssuer:     "fite-arsip-api",
		JWTAudience:   "fite-arsip-web",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
		SessionMaxAge: 30 * 24 * time.Hour,
		AllowedOrigin: "http://localhost:3000",
		CookieSecure:  false,
	}

	authSvc := service.NewAuthService(db, cfg)
	limiter := middleware.NewFailureLimiter(5, 1*time.Minute)
	authHandler := handler.NewAuthHandler(authSvc, cfg, limiter)

	r := gin.New()
	r.Use(middleware.RequestID(), middleware.Recover())

	api := r.Group("/api/v1")
	pub := api.Group("/auth")
	pub.POST("/login", httpx.Wrap(authHandler.Login))
	pub.POST("/refresh", httpx.Wrap(authHandler.Refresh))
	pub.POST("/logout", httpx.Wrap(authHandler.Logout))

	authed := api.Group("", middleware.Auth(authSvc))
	authed.GET("/auth/me", httpx.Wrap(authHandler.Me))
	authed.PUT("/auth/password", httpx.Wrap(authHandler.ChangePassword))

	g := authed.Group("", middleware.EnforcePasswordChange())
	g.PUT("/auth/profile", httpx.Wrap(authHandler.UpdateProfile))
	g.GET("/test-guarded", func(c *gin.Context) {
		httpx.OK(c, http.StatusOK, "guarded ok", nil)
	})

	return r, db, authSvc, cfg
}

func seedUserForHandler(t *testing.T, db *gorm.DB, roleKey, email, password string, isActive, mustChange bool) *domain.User {
	role := domain.Role{Key: roleKey, Name: roleKey}
	_ = db.FirstOrCreate(&role, domain.Role{Key: roleKey})

	pwHash, _ := bcrypt.GenerateFromPassword([]byte(password), 10)
	user := domain.User{
		Name:               "Budi Mahasiswa",
		Email:              email,
		PasswordHash:       string(pwHash),
		RoleID:             role.ID,
		Role:               role,
		IsActive:           isActive,
		MustChangePassword: mustChange,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return &user
}

func TestAuthHandler_LoginRefreshLogoutMe(t *testing.T) {
	r, db, _, _ := setupHandlerTestEnv(t)
	seedUserForHandler(t, db, domain.RoleMahasiswa, "mhs@fite.id", "Password123", true, false)

	// 1. POST /api/v1/auth/login
	loginPayload := `{"email":"mhs@fite.id","password":"Password123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 on login, got %d: %s", w.Code, w.Body.String())
	}

	var loginResp struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Data    struct {
			AccessToken string `json:"access_token"`
			TokenType   string `json:"token_type"`
			ExpiresIn   int    `json:"expires_in"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)

	if !loginResp.Success || loginResp.Data.AccessToken == "" {
		t.Fatalf("expected access token in response, got %+v", loginResp)
	}

	cookies := w.Result().Cookies()
	var refreshCookie *http.Cookie
	for _, ck := range cookies {
		if ck.Name == "refresh_token" {
			refreshCookie = ck
			break
		}
	}
	if refreshCookie == nil {
		t.Fatalf("expected refresh_token cookie set")
	}
	if !refreshCookie.HttpOnly || refreshCookie.SameSite != http.SameSiteStrictMode {
		t.Errorf("cookie security flags missing: httponly=%v samesite=%v", refreshCookie.HttpOnly, refreshCookie.SameSite)
	}

	// 2. GET /api/v1/auth/me
	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	wMe := httptest.NewRecorder()
	r.ServeHTTP(wMe, meReq)

	if wMe.Code != http.StatusOK {
		t.Fatalf("expected 200 on /auth/me, got %d: %s", wMe.Code, wMe.Body.String())
	}

	// 3. POST /api/v1/auth/refresh
	refreshReq := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	refreshReq.AddCookie(refreshCookie)
	wRefresh := httptest.NewRecorder()
	r.ServeHTTP(wRefresh, refreshReq)

	if wRefresh.Code != http.StatusOK {
		t.Fatalf("expected 200 on /auth/refresh, got %d: %s", wRefresh.Code, wRefresh.Body.String())
	}

	// 4. POST /api/v1/auth/logout
	logoutReq := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginResp.Data.AccessToken)
	logoutReq.AddCookie(refreshCookie)
	wLogout := httptest.NewRecorder()
	r.ServeHTTP(wLogout, logoutReq)

	if wLogout.Code != http.StatusOK {
		t.Fatalf("expected 200 on /auth/logout, got %d: %s", wLogout.Code, wLogout.Body.String())
	}

	// After logout, meReq should fail 401
	wMeAfter := httptest.NewRecorder()
	r.ServeHTTP(wMeAfter, meReq)
	if wMeAfter.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", wMeAfter.Code)
	}
}

func TestAuthHandler_MustChangePasswordEnforcement(t *testing.T) {
	r, db, _, _ := setupHandlerTestEnv(t)
	seedUserForHandler(t, db, domain.RoleDosen, "dosen@fite.id", "TempPass123", true, true)

	// Login with must_change_password = true
	loginPayload := `{"email":"dosen@fite.id","password":"TempPass123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(loginPayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var loginResp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &loginResp)
	token := loginResp.Data.AccessToken

	// Accessing /auth/me should be allowed
	meReq := httptest.NewRequest("GET", "/api/v1/auth/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	wMe := httptest.NewRecorder()
	r.ServeHTTP(wMe, meReq)
	if wMe.Code != http.StatusOK {
		t.Errorf("expected /auth/me to succeed even if must_change_password=true, got %d", wMe.Code)
	}

	// Accessing guarded route should fail with 403 password_change_required
	guardedReq := httptest.NewRequest("GET", "/api/v1/test-guarded", nil)
	guardedReq.Header.Set("Authorization", "Bearer "+token)
	wGuarded := httptest.NewRecorder()
	r.ServeHTTP(wGuarded, guardedReq)

	if wGuarded.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for guarded route, got %d", wGuarded.Code)
	}
	var failResp map[string]any
	_ = json.Unmarshal(wGuarded.Body.Bytes(), &failResp)
	if failResp["code"] != "password_change_required" {
		t.Errorf("expected code=password_change_required, got %v", failResp["code"])
	}
}
