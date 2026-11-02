package middleware_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
	httpx.Init()
}

func TestRequestID(t *testing.T) {
	r := gin.New()
	r.Use(middleware.RequestID())
	r.GET("/test", func(c *gin.Context) {
		reqID := httpx.RequestID(c)
		if reqID == "" {
			t.Errorf("expected request_id in context")
		}
		c.String(http.StatusOK, "ok")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.ServeHTTP(w, req)

	headerID := w.Header().Get("X-Request-ID")
	if headerID == "" {
		t.Errorf("expected X-Request-ID header in response")
	}
}

func TestRecover(t *testing.T) {
	r := gin.New()
	r.Use(middleware.Recover())
	r.GET("/panic", func(c *gin.Context) {
		panic("something went wrong")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500 on panic, got %d", w.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json parse error: %v", err)
	}
	if resp["success"] != false || resp["code"] != "server_error" {
		t.Errorf("unexpected response body: %v", resp)
	}
}

func TestCORS(t *testing.T) {
	origin := "http://localhost:3000"
	r := gin.New()
	r.Use(middleware.CORS(origin))
	r.GET("/data", func(c *gin.Context) {
		c.String(http.StatusOK, "data")
	})

	// Matching origin GET
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/data", nil)
	req.Header.Set("Origin", origin)
	r.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != origin {
		t.Errorf("expected Allow-Origin %s, got %s", origin, w.Header().Get("Access-Control-Allow-Origin"))
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Errorf("expected Allow-Credentials true")
	}

	// Preflight OPTIONS
	wOpt := httptest.NewRecorder()
	reqOpt := httptest.NewRequest(http.MethodOptions, "/data", nil)
	reqOpt.Header.Set("Origin", origin)
	r.ServeHTTP(wOpt, reqOpt)

	if wOpt.Code != http.StatusNoContent {
		t.Errorf("expected status 204 on OPTIONS preflight, got %d", wOpt.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	t.Run("Development", func(t *testing.T) {
		r := gin.New()
		r.Use(middleware.SecurityHeaders(false))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))

		if w.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Errorf("expected nosniff")
		}
		if w.Header().Get("Strict-Transport-Security") != "" {
			t.Errorf("HSTS should not be set in dev")
		}
	})

	t.Run("Production", func(t *testing.T) {
		r := gin.New()
		r.Use(middleware.SecurityHeaders(true))
		r.GET("/test", func(c *gin.Context) { c.Status(200) })

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/test", nil))

		if !strings.Contains(w.Header().Get("Strict-Transport-Security"), "max-age=31536000") {
			t.Errorf("HSTS should be set in prod, got: %s", w.Header().Get("Strict-Transport-Security"))
		}
	})
}

func TestLimitBody(t *testing.T) {
	r := gin.New()
	r.Use(middleware.LimitBody(1<<20, 11<<20, http.MethodPost, "/upload"))
	r.POST("/regular", func(c *gin.Context) { c.Status(200) })
	r.POST("/upload", func(c *gin.Context) { c.Status(200) })

	// Large body on regular route (> 1 MB)
	w := httptest.NewRecorder()
	largeData := bytes.Repeat([]byte("A"), 2<<20)
	req := httptest.NewRequest(http.MethodPost, "/regular", bytes.NewReader(largeData))
	req.ContentLength = int64(len(largeData))
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected 413 on regular route with large body, got %d", w.Code)
	}
}

func TestFailureLimiter(t *testing.T) {
	lim := middleware.NewFailureLimiter(3, 500*time.Millisecond)

	// Initially allowed
	ok, _ := lim.Check("user1")
	if !ok {
		t.Errorf("expected check to be ok initially")
	}

	// 3 hits
	lim.Hit("user1")
	lim.Hit("user1")
	lim.Hit("user1")

	// 4th check should be blocked
	ok, wait := lim.Check("user1")
	if ok {
		t.Errorf("expected check to be blocked after 3 hits")
	}
	if wait <= 0 {
		t.Errorf("expected positive wait duration")
	}

	// Reset
	lim.Reset("user1")
	ok, _ = lim.Check("user1")
	if !ok {
		t.Errorf("expected check to be ok after reset")
	}
}
