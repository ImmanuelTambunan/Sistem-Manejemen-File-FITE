package httpx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/httpx"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
	httpx.Init()
}

func TestStrongPassword(t *testing.T) {
	tests := []struct {
		pw   string
		want bool
	}{
		{"Short1!", false},               // < 8 chars
		{"alllowercase1", false},          // no uppercase
		{"ALLUPPERCASE1", false},          // no lowercase
		{"NoDigitsHere!", false},          // no digits
		{"ValidPass123", true},            // valid
		{"Another#Strong99", true},        // valid
		{"P@ssw0rd2026", true},            // valid
	}

	for _, tt := range tests {
		t.Run(tt.pw, func(t *testing.T) {
			got := httpx.StrongPassword(tt.pw)
			if got != tt.want {
				t.Errorf("StrongPassword(%q) = %v, want %v", tt.pw, got, tt.want)
			}
		})
	}
}

func TestResponseEnvelopes(t *testing.T) {
	t.Run("OK Envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		httpx.OK(c, http.StatusOK, "Berhasil", gin.H{"id": 1})

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json parse error: %v", err)
		}

		if resp["success"] != true || resp["message"] != "Berhasil" {
			t.Errorf("unexpected body: %v", resp)
		}
	})

	t.Run("Fail 422 Envelope with errors", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		err := apperr.Validation(map[string][]string{
			"email": {"Format email tidak valid."},
		})
		httpx.Fail(c, err)

		if w.Code != http.StatusUnprocessableEntity {
			t.Errorf("status = %d, want 422", w.Code)
		}

		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json parse error: %v", err)
		}

		if resp["success"] != false || resp["code"] != "validation_error" {
			t.Errorf("unexpected response: %v", resp)
		}
		errorsField, ok := resp["errors"].(map[string]any)
		if !ok || len(errorsField) == 0 {
			t.Errorf("expected errors map in response, got %v", resp["errors"])
		}
	})

	t.Run("Fail 429 Envelope with Retry-After header", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		err := apperr.RateLimited(45 * time.Second)
		httpx.Fail(c, err)

		if w.Code != http.StatusTooManyRequests {
			t.Errorf("status = %d, want 429", w.Code)
		}
		if retry := w.Header().Get("Retry-After"); retry != "45" {
			t.Errorf("expected Retry-After 45, got %q", retry)
		}
	})

	t.Run("Paged Envelope", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		meta := httpx.PageMeta(1, 15, 42)
		if meta.LastPage != 3 {
			t.Errorf("expected LastPage=3, got %d", meta.LastPage)
		}

		httpx.Paged(c, "OK", []string{"doc1", "doc2"}, meta)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}
	})
}
