package service_test

import (
	"context"
	"testing"
	"time"

	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/service"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}

	err = db.AutoMigrate(
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
	if err != nil {
		t.Fatalf("failed to auto migrate: %v", err)
	}
	return db
}

func testConfig() config.Config {
	return config.Config{
		JWTSecret:     []byte("super-secret-key-that-is-at-least-32-bytes-long!"),
		JWTIssuer:     "fite-arsip-api",
		JWTAudience:   "fite-arsip-web",
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    7 * 24 * time.Hour,
		SessionMaxAge: 30 * 24 * time.Hour,
		AllowedOrigin: "http://localhost:3000",
	}
}

func seedUser(t *testing.T, db *gorm.DB, roleKey, email, password string, isActive, mustChange bool) *domain.User {
	role := domain.Role{Key: roleKey, Name: roleKey}
	_ = db.FirstOrCreate(&role, domain.Role{Key: roleKey})

	pwHash, _ := bcrypt.GenerateFromPassword([]byte(password), 10)
	user := domain.User{
		Name:               "Test User",
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

func TestAuthService_Login(t *testing.T) {
	db := setupTestDB(t)
	cfg := testConfig()
	svc := service.NewAuthService(db, cfg)
	ctx := context.Background()

	seedUser(t, db, domain.RoleMahasiswa, "mhs@del.ac.id", "Secret123", true, false)
	seedUser(t, db, domain.RoleDosen, "inactive@del.ac.id", "Secret123", false, false)

	t.Run("Login success", func(t *testing.T) {
		pair, err := svc.Login(ctx, "mhs@del.ac.id", "Secret123", "UserAgent", "127.0.0.1")
		if err != nil {
			t.Fatalf("login failed: %v", err)
		}
		if pair.AccessToken == "" || pair.RefreshToken == "" {
			t.Errorf("expected non-empty tokens")
		}
		if pair.User.Email != "mhs@del.ac.id" {
			t.Errorf("expected user email mhs@del.ac.id, got %s", pair.User.Email)
		}
	})

	t.Run("Login invalid password", func(t *testing.T) {
		_, err := svc.Login(ctx, "mhs@del.ac.id", "WrongPass", "UserAgent", "127.0.0.1")
		if err != service.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("Login non-existent email", func(t *testing.T) {
		_, err := svc.Login(ctx, "notfound@del.ac.id", "SomePass", "UserAgent", "127.0.0.1")
		if err != service.ErrInvalidCredentials {
			t.Errorf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("Login inactive user", func(t *testing.T) {
		_, err := svc.Login(ctx, "inactive@del.ac.id", "Secret123", "UserAgent", "127.0.0.1")
		if err == nil {
			t.Fatal("expected error for inactive user")
		}
	})
}

func TestAuthService_RefreshTokenRotationAndReuse(t *testing.T) {
	db := setupTestDB(t)
	cfg := testConfig()
	svc := service.NewAuthService(db, cfg)
	ctx := context.Background()

	seedUser(t, db, domain.RoleBaak, "baak@del.ac.id", "Secret123", true, false)
	pair, err := svc.Login(ctx, "baak@del.ac.id", "Secret123", "Agent1", "127.0.0.1")
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	initialRefresh := pair.RefreshToken

	// 1st Refresh: rotation occurs
	newPair, err := svc.Refresh(ctx, initialRefresh, "Agent2", "127.0.0.1")
	if err != nil {
		t.Fatalf("first refresh failed: %v", err)
	}
	if newPair.RefreshToken == initialRefresh {
		t.Fatalf("expected new refresh token after rotation")
	}

	// Reuse detection: calling refresh with the old initialRefresh token
	_, err = svc.Refresh(ctx, initialRefresh, "AttackerAgent", "10.0.0.1")
	if err == nil {
		t.Fatalf("expected refresh with old token to fail")
	}

	// Now even the new token should be revoked because reuse attack revoked the session!
	_, err = svc.Refresh(ctx, newPair.RefreshToken, "Agent2", "127.0.0.1")
	if err == nil {
		t.Fatalf("expected revoked session to reject new token as well")
	}
}

func TestAuthService_ChangePassword(t *testing.T) {
	db := setupTestDB(t)
	cfg := testConfig()
	svc := service.NewAuthService(db, cfg)
	ctx := context.Background()

	user := seedUser(t, db, domain.RoleSuperAdmin, "admin@del.ac.id", "OldPass123", true, true)
	pair1, _ := svc.Login(ctx, "admin@del.ac.id", "OldPass123", "Device1", "127.0.0.1")
	pair2, _ := svc.Login(ctx, "admin@del.ac.id", "OldPass123", "Device2", "127.0.0.2")

	claims, err := svc.ParseAccess(pair1.AccessToken)
	if err != nil {
		t.Fatalf("parse access token failed: %v", err)
	}

	// Change password using session 1
	err = svc.ChangePassword(ctx, user, claims.SID, "OldPass123", "NewPass456")
	if err != nil {
		t.Fatalf("change password failed: %v", err)
	}

	// Session 1 should still be valid
	_, _, err = svc.Authenticate(ctx, pair1.AccessToken)
	if err != nil {
		t.Errorf("expected session 1 to remain valid, got %v", err)
	}

	// Session 2 should have been revoked!
	_, _, err = svc.Authenticate(ctx, pair2.AccessToken)
	if err == nil {
		t.Errorf("expected session 2 to be revoked after password change")
	}

	// User should have must_change_password set to false
	var updatedUser domain.User
	db.First(&updatedUser, user.ID)
	if updatedUser.MustChangePassword {
		t.Errorf("expected must_change_password=false")
	}
}

func TestFailureLimiter(t *testing.T) {
	limiter := middleware.NewFailureLimiter(5, 1*time.Minute)
	key := "test@del.ac.id|127.0.0.1"

	// 5 failed attempts
	for i := 0; i < 5; i++ {
		ok, _ := limiter.Check(key)
		if !ok {
			t.Fatalf("should not be blocked at attempt %d", i+1)
		}
		limiter.Hit(key)
	}

	// 6th attempt should be blocked
	ok, retry := limiter.Check(key)
	if ok {
		t.Fatalf("expected 6th attempt to be blocked")
	}
	if retry <= 0 {
		t.Fatalf("expected positive retry duration, got %v", retry)
	}

	// Reset clears the limit
	limiter.Reset(key)
	ok, _ = limiter.Check(key)
	if !ok {
		t.Fatalf("expected key to be unblocked after reset")
	}
}

