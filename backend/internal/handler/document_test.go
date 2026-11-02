package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
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
)

func setupDocHandlerEnv(t *testing.T) (*gin.Engine, *gorm.DB, *service.AuthService, config.Config) {
	gin.SetMode(gin.TestMode)
	httpx.Init()

	dbName := fmt.Sprintf("file:docmem_%s_%d?mode=memory&cache=shared", t.Name(), time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
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
		JWTSecret:      []byte("super-secret-key-that-is-at-least-32-bytes-long!"),
		JWTIssuer:      "fite-arsip-api",
		JWTAudience:    "fite-arsip-web",
		AccessTTL:      15 * time.Minute,
		RefreshTTL:     7 * 24 * time.Hour,
		SessionMaxAge:  30 * 24 * time.Hour,
		AllowedOrigin:  "http://localhost:3000",
		MaxFileBytes:   10 << 20,
		MaxUploadBody:  11 << 20,
		MaxJSONBody:    1 << 20,
		RetentionYears: 8,
		GraceDays:      90,
	}

	store := storage.NewMemoryStorage()
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

	engine := router.New(cfg, authSvc, h, func(_ context.Context) error { return nil })
	return engine, db, authSvc, cfg
}

func loginAndGetToken(t *testing.T, engine *gin.Engine, email, password string) string {
	payload := fmt.Sprintf(`{"email":"%s","password":"%s"}`, email, password)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("login failed for %s, code %d: %s", email, w.Code, w.Body.String())
	}

	var resp struct {
		Data struct {
			AccessToken string `json:"access_token"`
		} `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Data.AccessToken
}

func seedUserForDoc(t *testing.T, db *gorm.DB, roleKey, email, password string, prodiID *uint) *domain.User {
	role := domain.Role{Key: roleKey, Name: roleKey}
	_ = db.FirstOrCreate(&role, domain.Role{Key: roleKey})

	pwHash, _ := bcrypt.GenerateFromPassword([]byte(password), 10)
	user := domain.User{
		Name:               "User " + roleKey,
		Email:              email,
		PasswordHash:       string(pwHash),
		RoleID:             role.ID,
		Role:               role,
		ProdiID:            prodiID,
		IsActive:           true,
		MustChangePassword: false,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("failed to seed user: %v", err)
	}
	return &user
}

func TestDocumentHandler_FullLifecycle(t *testing.T) {
	engine, db, _, _ := setupDocHandlerEnv(t)

	// Seed reference data
	prodiIF := domain.Prodi{Code: "IF", Name: "S1 Informatika", IsActive: true}
	db.Create(&prodiIF)

	cat := domain.Category{Name: "Tugas Akhir", AllowedRoles: domain.StringList{domain.RoleMahasiswa, domain.RoleSuperAdmin}}
	db.Create(&cat)

	// Seed users
	mhs := seedUserForDoc(t, db, domain.RoleMahasiswa, "mhs_flow@del.ac.id", "Password123", &prodiIF.ID)
	_ = seedUserForDoc(t, db, domain.RoleBaak, "baak_flow@del.ac.id", "Password123", nil)

	// Get tokens
	mhsToken := loginAndGetToken(t, engine, mhs.Email, "Password123")
	baakToken := loginAndGetToken(t, engine, "baak_flow@del.ac.id", "Password123")

	// 1. Upload document (POST /api/v1/documents)
	body := new(bytes.Buffer)
	mw := multipart.NewWriter(body)
	_ = mw.WriteField("title", "Proposal Skripsi TA")
	_ = mw.WriteField("category_id", fmt.Sprintf("%d", cat.ID))
	_ = mw.WriteField("visibility", "private")

	fw, err := mw.CreateFormFile("file", "proposal.pdf")
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	_, _ = fw.Write([]byte("%PDF-1.4 sample proposal body"))
	_ = mw.Close()

	uploadReq := httptest.NewRequest("POST", "/api/v1/documents", body)
	uploadReq.Header.Set("Content-Type", mw.FormDataContentType())
	uploadReq.Header.Set("Authorization", "Bearer "+mhsToken)
	wUpload := httptest.NewRecorder()
	engine.ServeHTTP(wUpload, uploadReq)

	if wUpload.Code != http.StatusCreated {
		t.Fatalf("expected 201 on upload, got %d: %s", wUpload.Code, wUpload.Body.String())
	}

	var uploadResp struct {
		Data struct {
			ID         uint   `json:"id"`
			Title      string `json:"title"`
			Visibility string `json:"visibility"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wUpload.Body.Bytes(), &uploadResp)
	docID := uploadResp.Data.ID

	if docID == 0 || uploadResp.Data.Visibility != "private" {
		t.Fatalf("unexpected upload response: %+v", uploadResp)
	}

	// 2. Download document (GET /api/v1/documents/:id/download)
	dlReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/documents/%d/download", docID), nil)
	dlReq.Header.Set("Authorization", "Bearer "+mhsToken)
	wDl := httptest.NewRecorder()
	engine.ServeHTTP(wDl, dlReq)

	if wDl.Code != http.StatusOK {
		t.Fatalf("expected 200 on download, got %d: %s", wDl.Code, wDl.Body.String())
	}
	if cd := wDl.Header().Get("Content-Disposition"); cd == "" {
		t.Errorf("expected Content-Disposition header")
	}
	if nosniff := wDl.Header().Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", nosniff)
	}

	// 3. Change Visibility to Public (PATCH /api/v1/documents/:id/visibility)
	visPayload := `{"visibility":"public"}`
	visReq := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/documents/%d/visibility", docID), bytes.NewBufferString(visPayload))
	visReq.Header.Set("Content-Type", "application/json")
	visReq.Header.Set("Authorization", "Bearer "+mhsToken)
	wVis := httptest.NewRecorder()
	engine.ServeHTTP(wVis, visReq)

	if wVis.Code != http.StatusOK {
		t.Fatalf("expected 200 on update visibility, got %d: %s", wVis.Code, wVis.Body.String())
	}

	// 4. BAAK lists verifications (GET /api/v1/verifications)
	verListReq := httptest.NewRequest("GET", "/api/v1/verifications?status=pending", nil)
	verListReq.Header.Set("Authorization", "Bearer "+baakToken)
	wVerList := httptest.NewRecorder()
	engine.ServeHTTP(wVerList, verListReq)

	if wVerList.Code != http.StatusOK {
		t.Fatalf("expected 200 on verifications list, got %d: %s", wVerList.Code, wVerList.Body.String())
	}

	var verListResp struct {
		Data []struct {
			ID         uint `json:"id"`
			DocumentID uint `json:"document_id"`
			Status     string `json:"status"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wVerList.Body.Bytes(), &verListResp)
	if len(verListResp.Data) == 0 {
		t.Fatalf("expected at least 1 pending verification, got %d", len(verListResp.Data))
	}
	verID := verListResp.Data[0].ID

	// 5. BAAK decides approval (PATCH /api/v1/verifications/:id)
	decidePayload := `{"status":"approved"}`
	decideReq := httptest.NewRequest("PATCH", fmt.Sprintf("/api/v1/verifications/%d", verID), bytes.NewBufferString(decidePayload))
	decideReq.Header.Set("Content-Type", "application/json")
	decideReq.Header.Set("Authorization", "Bearer "+baakToken)
	wDecide := httptest.NewRecorder()
	engine.ServeHTTP(wDecide, decideReq)

	if wDecide.Code != http.StatusOK {
		t.Fatalf("expected 200 on decide verification, got %d: %s", wDecide.Code, wDecide.Body.String())
	}

	// 6. Public Index explorer (GET /api/v1/documents/public) - no auth required or authed
	pubReq := httptest.NewRequest("GET", "/api/v1/documents/public", nil)
	pubReq.Header.Set("Authorization", "Bearer "+mhsToken)
	wPub := httptest.NewRecorder()
	engine.ServeHTTP(wPub, pubReq)

	if wPub.Code != http.StatusOK {
		t.Fatalf("expected 200 on public index, got %d: %s", wPub.Code, wPub.Body.String())
	}

	var pubResp struct {
		Data []struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	_ = json.Unmarshal(wPub.Body.Bytes(), &pubResp)
	if len(pubResp.Data) == 0 {
		t.Errorf("expected approved document in public index, got 0")
	}

	// 7. Mahasiswa deletes document (DELETE /api/v1/documents/:id)
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/v1/documents/%d", docID), nil)
	delReq.Header.Set("Authorization", "Bearer "+mhsToken)
	wDel := httptest.NewRecorder()
	engine.ServeHTTP(wDel, delReq)

	if wDel.Code != http.StatusNoContent {
		t.Fatalf("expected 204 on delete, got %d: %s", wDel.Code, wDel.Body.String())
	}
}
