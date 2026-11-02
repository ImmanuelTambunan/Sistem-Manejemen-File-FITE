package middleware

import (
	"strings"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	ctxUser    = "auth.user"
	ctxSession = "auth.session"
)

func CurrentUser(c *gin.Context) *domain.User {
	v, _ := c.Get(ctxUser)
	u, _ := v.(*domain.User)
	return u
}

func CurrentSession(c *gin.Context) *domain.AuthSession {
	v, _ := c.Get(ctxSession)
	s, _ := v.(*domain.AuthSession)
	return s
}

// Auth memverifikasi JWT, lalu memuat sesi, pengguna, dan peran TERBARU dari basis data
// (peran dan status tidak dipercaya dari klaim token).
func Auth(auth *service.AuthService) gin.HandlerFunc {
	const prefix = "Bearer "
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
			httpx.Abort(c, apperr.Unauthenticated(""))
			return
		}
		user, sess, err := auth.Authenticate(c.Request.Context(), strings.TrimSpace(h[len(prefix):]))
		if err != nil {
			httpx.Abort(c, err)
			return
		}
		c.Set(ctxUser, user)
		c.Set(ctxSession, sess)
		c.Next()
	}
}

// EnforcePasswordChange dipasang pada semua rute kecuali /auth/me dan /auth/password.
func EnforcePasswordChange() gin.HandlerFunc {
	return func(c *gin.Context) {
		if u := CurrentUser(c); u != nil && u.MustChangePassword {
			httpx.Abort(c, apperr.PasswordChangeRequired())
			return
		}
		c.Next()
	}
}

func RequireRoles(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(c *gin.Context) {
		u := CurrentUser(c)
		if u == nil {
			httpx.Abort(c, apperr.Unauthenticated(""))
			return
		}
		if _, ok := allowed[u.Role.Key]; !ok {
			httpx.Abort(c, apperr.Forbidden(""))
			return
		}
		c.Next()
	}
}

