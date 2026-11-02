package handler

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	refreshCookie = "refresh_token"
	cookiePath    = "/api/v1/auth"
)

type AuthHandler struct {
	svc     *service.AuthService
	cfg     config.Config
	limiter *middleware.FailureLimiter
}

func NewAuthHandler(svc *service.AuthService, cfg config.Config, l *middleware.FailureLimiter) *AuthHandler {
	return &AuthHandler{svc: svc, cfg: cfg, limiter: l}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,max=255"`
}

type profileRequest struct {
	Name  string  `json:"name" binding:"required,max=100"`
	Phone *string `json:"phone" binding:"omitempty,phone"`
}

type passwordRequest struct {
	CurrentPassword      string `json:"current_password" binding:"required,max=255"`
	Password             string `json:"password" binding:"required,strongpw"`
	PasswordConfirmation string `json:"password_confirmation" binding:"required,eqfield=Password"`
}

func (h *AuthHandler) tokenBody(p *service.TokenPair) gin.H {
	return gin.H{
		"access_token": p.AccessToken,
		"token_type":   "Bearer",
		"expires_in":   p.ExpiresIn,
		"user":         toUserResponse(p.User),
	}
}

func (h *AuthHandler) setRefreshCookie(c *gin.Context, raw string, exp time.Time) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookie,
		Value:    raw,
		Path:     cookiePath,
		Domain:   h.cfg.CookieDomain,
		Expires:  exp,
		MaxAge:   int(time.Until(exp).Seconds()),
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     refreshCookie,
		Value:    "",
		Path:     cookiePath,
		Domain:   h.cfg.CookieDomain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: http.SameSiteStrictMode,
	})
}

// checkOrigin: pertahanan CSRF tambahan untuk rute yang memakai cookie. Di production header Origin wajib ada.
func (h *AuthHandler) checkOrigin(c *gin.Context) error {
	o := c.GetHeader("Origin")
	if o == "" && !h.cfg.IsProd() {
		return nil // memudahkan pengujian dengan Postman
	}
	if o != h.cfg.AllowedOrigin {
		return apperr.Forbidden("Origin permintaan tidak sah.")
	}
	return nil
}

func (h *AuthHandler) Login(c *gin.Context) error {
	var req loginRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	c.Header("Cache-Control", "no-store")
	key := strings.ToLower(strings.TrimSpace(req.Email)) + "|" + c.ClientIP()
	if ok, retry := h.limiter.Check(key); !ok {
		return apperr.RateLimited(retry)
	}

	pair, err := h.svc.Login(c.Request.Context(), req.Email, req.Password, c.Request.UserAgent(), c.ClientIP())
	if errors.Is(err, service.ErrInvalidCredentials) {
		h.limiter.Hit(key) // hanya percobaan gagal yang dihitung
		return apperr.Unauthenticated("Email atau kata sandi salah.")
	}
	if err != nil {
		return err
	}

	h.limiter.Reset(key)
	h.setRefreshCookie(c, pair.RefreshToken, pair.RefreshExpiresAt)
	httpx.OK(c, http.StatusOK, "Login berhasil", h.tokenBody(pair))
	return nil
}

func (h *AuthHandler) Refresh(c *gin.Context) error {
	c.Header("Cache-Control", "no-store")
	if err := h.checkOrigin(c); err != nil {
		return err
	}
	raw, err := c.Cookie(refreshCookie)
	if err != nil || raw == "" {
		return apperr.Unauthenticated("")
	}

	pair, err := h.svc.Refresh(c.Request.Context(), raw, c.Request.UserAgent(), c.ClientIP())
	if err != nil {
		h.clearRefreshCookie(c)
		return err
	}

	h.setRefreshCookie(c, pair.RefreshToken, pair.RefreshExpiresAt)
	httpx.OK(c, http.StatusOK, "Token diperbarui", h.tokenBody(pair))
	return nil
}

// Logout tidak memakai middleware Auth agar tetap bekerja saat access token sudah kedaluwarsa
// (sesi ditentukan dari cookie refresh).
func (h *AuthHandler) Logout(c *gin.Context) error {
	if err := h.checkOrigin(c); err != nil {
		return err
	}
	ctx := c.Request.Context()
	revoked := false

	if hdr := c.GetHeader("Authorization"); len(hdr) > 7 && strings.EqualFold(hdr[:7], "Bearer ") {
		if claims, err := h.svc.ParseAccess(strings.TrimSpace(hdr[7:])); err == nil {
			if err := h.svc.RevokeSession(ctx, claims.SID); err != nil {
				return err
			}
			revoked = true
		}
	}
	if raw, err := c.Cookie(refreshCookie); err == nil && raw != "" {
		ok, err := h.svc.RevokeByRefreshToken(ctx, raw)
		if err != nil {
			return err
		}
		revoked = revoked || ok
	}

	h.clearRefreshCookie(c)
	if !revoked {
		return apperr.Unauthenticated("")
	}
	httpx.OK(c, http.StatusOK, "Logout berhasil", nil)
	return nil
}

func (h *AuthHandler) Me(c *gin.Context) error {
	u := middleware.CurrentUser(c)
	httpx.OK(c, http.StatusOK, "OK", toUserResponse(u))
	return nil
}

func (h *AuthHandler) UpdateProfile(c *gin.Context) error {
	var req profileRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	u := middleware.CurrentUser(c)
	phone := ""
	if req.Phone != nil {
		phone = *req.Phone
	}
	if err := h.svc.UpdateProfile(c.Request.Context(), u.ID, req.Name, phone); err != nil {
		return err
	}
	u.Name = req.Name
	u.Phone = req.Phone

	httpx.OK(c, http.StatusOK, "Profil diperbarui", toUserResponse(u))
	return nil
}

func (h *AuthHandler) ChangePassword(c *gin.Context) error {
	var req passwordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	u, sess := middleware.CurrentUser(c), middleware.CurrentSession(c)
	if err := h.svc.ChangePassword(c.Request.Context(), u, sess.ID, req.CurrentPassword, req.Password); err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "Kata sandi diperbarui", nil)
	return nil
}

