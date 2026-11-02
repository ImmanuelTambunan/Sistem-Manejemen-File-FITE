package middleware

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/httpx"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := uuid.NewString() // sengaja tidak memercayai ID dari klien
		c.Set(httpx.RequestIDKey, id)
		c.Header("X-Request-ID", id)
		c.Next()
	}
}

func Recover() gin.HandlerFunc {
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, rec any) {
		httpx.Abort(c, apperr.Internal(fmt.Errorf("panic: %v", rec)))
	})
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		attrs := []any{
			"request_id", httpx.RequestID(c), "method", c.Request.Method, "route", c.FullPath(),
			"status", c.Writer.Status(), "ms", time.Since(start).Milliseconds(), "ip", c.ClientIP(),
		}
		if u := CurrentUser(c); u != nil {
			attrs = append(attrs, "user_id", u.ID)
		}
		slog.Info("http", attrs...)
	}
}

func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if o := c.GetHeader("Origin"); o != "" && o == origin {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Add("Vary", "Origin")
			h.Set("Access-Control-Expose-Headers", "X-Request-ID, Retry-After, Content-Disposition")
			if c.Request.Method == http.MethodOptions {
				h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
				h.Set("Access-Control-Max-Age", "600")
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
		}
		c.Next()
	}
}

func SecurityHeaders(prod bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if prod {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// LimitBody: batas keras isi permintaan. Rute unggah memakai batas lebih besar.
func LimitBody(jsonMax, uploadMax int64, uploadMethod, uploadRoute string) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := jsonMax
		if c.Request.Method == uploadMethod && c.FullPath() == uploadRoute {
			limit = uploadMax
		}
		if c.Request.ContentLength > limit {
			httpx.Abort(c, apperr.TooLarge(limit))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}

