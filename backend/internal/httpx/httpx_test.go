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
		{"Short1!", false},        // < 8 chars
		{"alllowercase1", false},  // no uppercase
		{"ALLUPPERCASE1", false},  // no lowercase
		{"NoDigitsHere!", false},  // no digits
		{"ValidPass123", true},    // valid
		{"Another#Strong99", true}, // valid
		{"P@ssw0rd2026", true},    // valid
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

	t.Run("Paged Envelope with Dual Compatibility", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		meta := httpx.PageMeta(1, 15, 42)
		if meta.LastPage != 3 {
			t.Errorf("expected LastPage=3, got %d", meta.LastPage)
		}
		if meta.TotalPages != 3 {
			t.Errorf("expected TotalPages=3, got %d", meta.TotalPages)
		}
		if meta.CurrentPage != 1 {
			t.Errorf("expected CurrentPage=1, got %d", meta.CurrentPage)
		}
		if meta.Page != 1 {
			t.Errorf("expected Page=1, got %d", meta.Page)
		}
		if meta.PerPage != 15 {
			t.Errorf("expected PerPage=15, got %d", meta.PerPage)
		}
		if meta.Limit != 15 {
			t.Errorf("expected Limit=15, got %d", meta.Limit)
		}

		httpx.Paged(c, "OK", []string{"doc1", "doc2"}, meta)
		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want 200", w.Code)
		}

		var resp struct {
			Success bool `json:"success"`
			Meta    struct {
				CurrentPage int   `json:"current_page"`
				Page        int   `json:"page"`
				PerPage     int   `json:"per_page"`
				Limit       int   `json:"limit"`
				Total       int64 `json:"total"`
				LastPage    int   `json:"last_page"`
				TotalPages  int   `json:"total_pages"`
			} `json:"meta"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("json parse error: %v", err)
		}

		if resp.Meta.CurrentPage != 1 || resp.Meta.Page != 1 {
			t.Errorf("unexpected page values: current_page=%d, page=%d", resp.Meta.CurrentPage, resp.Meta.Page)
		}
		if resp.Meta.LastPage != 3 || resp.Meta.TotalPages != 3 {
			t.Errorf("unexpected last page values: last_page=%d, total_pages=%d", resp.Meta.LastPage, resp.Meta.TotalPages)
		}
		if resp.Meta.PerPage != 15 || resp.Meta.Limit != 15 {
			t.Errorf("unexpected per_page values: per_page=%d, limit=%d", resp.Meta.PerPage, resp.Meta.Limit)
		}
	})
}
