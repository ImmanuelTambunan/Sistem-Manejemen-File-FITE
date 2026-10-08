package apperr

import (
	"fmt"
	"net/http"
	"time"
)

type Error struct {
	Status     int
	Code       string
	Message    string
	Fields     map[string][]string
	RetryAfter time.Duration
	Err        error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}
func (e *Error) Unwrap() error { return e.Err }

func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

func or(msg, def string) string {
	if msg == "" {
		return def
	}
	return msg
}

func Unauthenticated(msg string) *Error {
	return New(http.StatusUnauthorized, "unauthenticated",
		or(msg, "Tidak terautentikasi. Token tidak ada, tidak valid, atau kedaluwarsa."))
}
func Forbidden(msg string) *Error {
	return New(http.StatusForbidden, "forbidden", or(msg, "Anda tidak memiliki hak akses untuk tindakan ini."))
}
func AccountDisabled() *Error {
	return New(http.StatusForbidden, "account_disabled", "Akun Anda dinonaktifkan. Hubungi administrator.")
}
func PasswordChangeRequired() *Error {
	return New(http.StatusForbidden, "password_change_required", "Anda wajib mengganti kata sandi sebelum melanjutkan.")
}
func NotFound(msg string) *Error {
	return New(http.StatusNotFound, "not_found", or(msg, "Sumber daya tidak ditemukan."))
}
func Conflict(msg string) *Error {
	return New(http.StatusConflict, "conflict", or(msg, "Konflik status data."))
}
func TooLarge(limit int64) *Error {
	return New(http.StatusRequestEntityTooLarge, "payload_too_large",
		fmt.Sprintf("Ukuran permintaan melebihi batas %d MB.", limit>>20))
}
func UnsupportedMedia(msg string) *Error {
	return New(http.StatusUnsupportedMediaType, "unsupported_media_type", or(msg, "Tipe konten tidak didukung."))
}
func Validation(fields map[string][]string) *Error {
	e := New(http.StatusUnprocessableEntity, "validation_error", "Data yang dikirim tidak valid")
	e.Fields = fields
	return e
}
func FieldError(field, msg string) *Error { return Validation(map[string][]string{field: {msg}}) }
func RateLimited(after time.Duration) *Error {
	e := New(http.StatusTooManyRequests, "rate_limited", "Terlalu banyak permintaan. Coba lagi beberapa saat.")
	e.RetryAfter = after
	return e
}
func Internal(err error) *Error {
	e := New(http.StatusInternalServerError, "server_error", "Terjadi kesalahan pada server.")
	e.Err = err
	return e
}
