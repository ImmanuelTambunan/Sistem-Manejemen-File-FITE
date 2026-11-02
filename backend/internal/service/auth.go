package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const bcryptCost = 12

var ErrInvalidCredentials = errors.New("kredensial tidak valid")

// dummyHash dipakai agar waktu respons sama untuk email tidak terdaftar (anti-enumerasi).
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("dummy-password-timing"), bcryptCost)

type Claims struct {
	SID string `json:"sid"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken      string
	ExpiresIn        int
	RefreshToken     string
	RefreshExpiresAt time.Time
	User             *domain.User
}

type AuthService struct {
	db  *gorm.DB
	cfg config.Config
	now func() time.Time
}

func NewAuthService(db *gorm.DB, cfg config.Config) *AuthService {
	return &AuthService{db: db, cfg: cfg, now: time.Now}
}

func (s *AuthService) SetNowFunc(f func() time.Time) {
	s.now = f
}

func (s *AuthService) Login(ctx context.Context, email, password, ua, ip string) (*TokenPair, error) {
	var user domain.User
	err := s.db.WithContext(ctx).Preload("Role").Preload("Prodi").
		Where("email = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error
	found := err == nil
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hash := dummyHash
	if found {
		hash = []byte(user.PasswordHash)
	}

	pwOK := bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil
	if !found || !pwOK {
		return nil, ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, apperr.AccountDisabled()
	}

	return s.startSession(ctx, &user, ua, ip)
}

func (s *AuthService) startSession(ctx context.Context, user *domain.User, ua, ip string) (*TokenPair, error) {
	now := s.now().UTC()
	raw, hash, err := newRefreshToken()
	if err != nil {
		return nil, err
	}

	absolute := now.Add(s.cfg.SessionMaxAge)
	exp := minTime(now.Add(s.cfg.RefreshTTL), absolute)
	sess := domain.AuthSession{
		ID:                  uuid.NewString(),
		UserID:              user.ID,
		RefreshTokenHash:    hash,
		UserAgent:           truncate(ua, 255),
		IP:                  ip,
		ExpiresAt:           exp,
		AbsoluteExpiresAt:   absolute,
		LastUsedAt:          &now,
	}
	if err := s.db.WithContext(ctx).Create(&sess).Error; err != nil {
		return nil, err
	}

	access, err := s.signAccess(user.ID, sess.ID, now)
	if err != nil {
		return nil, err
	}

	return &TokenPair{
		AccessToken:      access,
		ExpiresIn:        int(s.cfg.AccessTTL.Seconds()),
		RefreshToken:     raw,
		RefreshExpiresAt: exp,
		User:             user,
	}, nil
}

func (s *AuthService) signAccess(uid uint, sid string, now time.Time) (string, error) {
	claims := Claims{
		SID: sid,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(uint64(uid), 10),
			Issuer:    s.cfg.JWTIssuer,
			Audience:  jwt.ClaimStrings{s.cfg.JWTAudience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.AccessTTL)),
			ID:        uuid.NewString(),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.cfg.JWTSecret)
}

// ParseAccess memverifikasi tanda tangan (algoritma dikunci HS256), exp, iss, dan aud.
func (s *AuthService) ParseAccess(raw string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(raw, claims,
		func(*jwt.Token) (any, error) { return s.cfg.JWTSecret, nil },
		jwt.WithValidMethods([]string{"HS256"}),
		jwt.WithIssuer(s.cfg.JWTIssuer),
		jwt.WithAudience(s.cfg.JWTAudience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(5*time.Second),
	)
	if err != nil || claims.SID == "" {
		return nil, apperr.Unauthenticated("")
	}
	return claims, nil
}

// Authenticate: token sah + sesi aktif + akun aktif; peran dibaca dari DB pada setiap permintaan.
func (s *AuthService) Authenticate(ctx context.Context, raw string) (*domain.User, *domain.AuthSession, error) {
	claims, err := s.ParseAccess(raw)
	if err != nil {
		return nil, nil, err
	}
	uid, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return nil, nil, apperr.Unauthenticated("")
	}

	now := s.now().UTC()
	var sess domain.AuthSession
	err = s.db.WithContext(ctx).
		Where("id = ? AND user_id = ? AND revoked_at IS NULL AND expires_at > ? AND absolute_expires_at > ?",
			claims.SID, uid, now, now).First(&sess).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, apperr.Unauthenticated("")
	}
	if err != nil {
		return nil, nil, err
	}

	var user domain.User
	err = s.db.WithContext(ctx).Preload("Role").Preload("Prodi").First(&user, uid).Error
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !user.IsActive) {
		return nil, nil, apperr.Unauthenticated("")
	}
	if err != nil {
		return nil, nil, err
	}

	return &user, &sess, nil
}

// Refresh memutar refresh token. Pemakaian ulang token lama mencabut sesi (deteksi pencurian).
func (s *AuthService) Refresh(ctx context.Context, rawToken, ua, ip string) (*TokenPair, error) {
	hash := hashToken(rawToken)
	now := s.now().UTC()
	var pair *TokenPair

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var sess domain.AuthSession
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("refresh_token_hash = ?", hash).First(&sess).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			res := tx.Model(&domain.AuthSession{}).
				Where("previous_refresh_hash = ? AND revoked_at IS NULL", hash).Update("revoked_at", now)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected > 0 {
				slog.Warn("pemakaian ulang refresh token terdeteksi; sesi dicabut")
			}
			return nil // pair tetap nil -> 401 (pencabutan di-commit)
		}
		if err != nil {
			return err
		}

		if sess.RevokedAt != nil || !now.Before(sess.ExpiresAt) || !now.Before(sess.AbsoluteExpiresAt) {
			return nil
		}

		var user domain.User
		err = tx.Preload("Role").Preload("Prodi").First(&user, sess.UserID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && !user.IsActive) {
			return tx.Model(&sess).Update("revoked_at", now).Error
		}
		if err != nil {
			return err
		}

		raw, newHash, err := newRefreshToken()
		if err != nil {
			return err
		}
		exp := minTime(now.Add(s.cfg.RefreshTTL), sess.AbsoluteExpiresAt)
		if err := tx.Model(&sess).Updates(map[string]any{
			"previous_refresh_hash": sess.RefreshTokenHash,
			"refresh_token_hash":    newHash,
			"expires_at":           exp,
			"last_used_at":         now,
			"user_agent":           truncate(ua, 255),
			"ip":                   ip,
		}).Error; err != nil {
			return err
		}

		access, err := s.signAccess(user.ID, sess.ID, now)
		if err != nil {
			return err
		}

		pair = &TokenPair{
			AccessToken:      access,
			ExpiresIn:        int(s.cfg.AccessTTL.Seconds()),
			RefreshToken:     raw,
			RefreshExpiresAt: exp,
			User:             &user,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, apperr.Unauthenticated("")
	}
	return pair, nil
}

func (s *AuthService) RevokeSession(ctx context.Context, sid string) error {
	return s.db.WithContext(ctx).Model(&domain.AuthSession{}).
		Where("id = ? AND revoked_at IS NULL", sid).Update("revoked_at", s.now().UTC()).Error
}

func (s *AuthService) RevokeByRefreshToken(ctx context.Context, raw string) (bool, error) {
	h := hashToken(raw)
	res := s.db.WithContext(ctx).Model(&domain.AuthSession{}).
		Where("(refresh_token_hash = ? OR previous_refresh_hash = ?) AND revoked_at IS NULL", h, h).
		Update("revoked_at", s.now().UTC())
	return res.RowsAffected > 0, res.Error
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID uint, name, phone string) error {
	var p any
	if phone != "" {
		p = phone
	}
	return s.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", userID).
		Updates(map[string]any{"name": name, "phone": p}).Error
}

func (s *AuthService) ChangePassword(ctx context.Context, user *domain.User, currentSID, current, next string) error {
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(current)) != nil {
		return apperr.FieldError("current_password", "Kata sandi lama tidak sesuai.")
	}
	if current == next {
		return apperr.FieldError("password", "Kata sandi baru harus berbeda dari kata sandi lama.")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(next), bcryptCost)
	if err != nil {
		return err
	}

	now := s.now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.User{}).Where("id = ?", user.ID).
			Updates(map[string]any{"password": string(h), "must_change_password": false}).Error; err != nil {
			return err
		}
		// Sesi lain dicabut; sesi yang sedang dipakai tetap berlaku
		return tx.Model(&domain.AuthSession{}).
			Where("user_id = ? AND id <> ? AND revoked_at IS NULL", user.ID, currentSID).
			Update("revoked_at", now).Error
	})
}

// Helpers
func newRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

