package apperr_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"fite-arsip-api/internal/apperr"
)

func TestAppErrorConstructors(t *testing.T) {
	tests := []struct {
		name       string
		err        *apperr.Error
		wantStatus int
		wantCode   string
	}{
		{"Unauthenticated default", apperr.Unauthenticated(""), http.StatusUnauthorized, "unauthenticated"},
		{"Unauthenticated custom", apperr.Unauthenticated("Token expired"), http.StatusUnauthorized, "unauthenticated"},
		{"Forbidden default", apperr.Forbidden(""), http.StatusForbidden, "forbidden"},
		{"AccountDisabled", apperr.AccountDisabled(), http.StatusForbidden, "account_disabled"},
		{"PasswordChangeRequired", apperr.PasswordChangeRequired(), http.StatusForbidden, "password_change_required"},
		{"NotFound default", apperr.NotFound(""), http.StatusNotFound, "not_found"},
		{"Conflict default", apperr.Conflict(""), http.StatusConflict, "conflict"},
		{"TooLarge", apperr.TooLarge(11 << 20), http.StatusRequestEntityTooLarge, "payload_too_large"},
		{"UnsupportedMedia", apperr.UnsupportedMedia(""), http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"Validation", apperr.Validation(map[string][]string{"email": {"wajib diisi"}}), http.StatusUnprocessableEntity, "validation_error"},
		{"FieldError", apperr.FieldError("title", "wajib diisi"), http.StatusUnprocessableEntity, "validation_error"},
		{"RateLimited", apperr.RateLimited(60 * time.Second), http.StatusTooManyRequests, "rate_limited"},
		{"Internal", apperr.Internal(errors.New("db error")), http.StatusInternalServerError, "server_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Status != tt.wantStatus {
				t.Errorf("got status %d, want %d", tt.err.Status, tt.wantStatus)
			}
			if tt.err.Code != tt.wantCode {
				t.Errorf("got code %q, want %q", tt.err.Code, tt.wantCode)
			}
		})
	}
}

func TestAppErrorUnwrap(t *testing.T) {
	orig := errors.New("underlying cause")
	e := apperr.Internal(orig)
	if !errors.Is(e, orig) {
		t.Errorf("errors.Is should match unwrapped error")
	}
	if e.Error() != "Terjadi kesalahan pada server.: underlying cause" {
		t.Errorf("unexpected error string: %s", e.Error())
	}
}
