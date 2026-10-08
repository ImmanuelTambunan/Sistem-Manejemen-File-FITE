package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"fite-arsip-api/internal/apperr"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

const RequestIDKey = "request_id"

type Meta struct {
	CurrentPage int   `json:"current_page"`
	PerPage     int   `json:"per_page"`
	Total       int64 `json:"total"`
	LastPage    int   `json:"last_page"`
}

func PageMeta(page, per int, total int64) Meta {
	last := int(math.Ceil(float64(total) / float64(per)))
	if last < 1 {
		last = 1
	}
	return Meta{CurrentPage: page, PerPage: per, Total: total, LastPage: last}
}

func RequestID(c *gin.Context) string { return c.GetString(RequestIDKey) }

func OK(c *gin.Context, status int, message string, data any) {
	c.JSON(status, gin.H{"success": true, "message": message, "data": data})
}

func Paged(c *gin.Context, message string, data any, meta Meta) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": message, "data": data, "meta": meta})
}

func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }

func Abort(c *gin.Context, err error) {
	Fail(c, err)
	c.Abort()
}

func Fail(c *gin.Context, err error) {
	var ae *apperr.Error
	if !errors.As(err, &ae) {
		ae = mapError(err)
	}
	if ae.Status >= 500 {
		logError(c, err)
	}
	if ae.RetryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(int(math.Ceil(ae.RetryAfter.Seconds()))))
	}
	body := gin.H{"success": false, "code": ae.Code, "message": ae.Message}
	if len(ae.Fields) > 0 {
		body["errors"] = ae.Fields
	}
	c.JSON(ae.Status, body)
}

func logError(c *gin.Context, err error) {
	slog.ErrorContext(c.Request.Context(), "request gagal",
		"request_id", RequestID(c), "method", c.Request.Method, "path", c.FullPath(), "error", err)
}

func mapError(err error) *apperr.Error {
	var mbe *http.MaxBytesError
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperr.NotFound("")
	case errors.As(err, &mbe):
		return apperr.TooLarge(mbe.Limit)
	case strings.Contains(err.Error(), "request body too large"):
		return apperr.TooLarge(11 << 20)
	}
	return apperr.Internal(err)
}

type HandlerFunc func(*gin.Context) error

// Wrap mengubah handler yang mengembalikan error menjadi gin.HandlerFunc.
func Wrap(h HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := h(c); err != nil {
			Fail(c, err)
		}
	}
}

// ---------------------------------------------------------------- binding

var phoneRe = regexp.MustCompile(`^\+?[0-9]{8,15}$`)

func Init() {
	binding.EnableDecoderDisallowUnknownFields = true
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		for _, tag := range []string{"json", "form"} {
			if name := strings.Split(f.Tag.Get(tag), ",")[0]; name != "" && name != "-" {
				return name
			}
		}
		return f.Name
	})
	_ = v.RegisterValidation("phone", func(fl validator.FieldLevel) bool { return phoneRe.MatchString(fl.Field().String()) })
	_ = v.RegisterValidation("strongpw", func(fl validator.FieldLevel) bool { return StrongPassword(fl.Field().String()) })
}

// StrongPassword: 8-72 byte (batas bcrypt), memuat huruf besar, kecil, dan angka.
func StrongPassword(s string) bool {
	if len(s) < 8 || len(s) > 72 {
		return false
	}
	var up, lo, di bool
	for _, r := range s {
		switch {
		case unicode.IsUpper(r):
			up = true
		case unicode.IsLower(r):
			lo = true
		case unicode.IsDigit(r):
			di = true
		}
	}
	return up && lo && di
}

func BindJSON(c *gin.Context, dst any) error {
	if c.ContentType() != "application/json" {
		return apperr.UnsupportedMedia("Content-Type harus application/json.")
	}
	return bindErr(c.ShouldBindJSON(dst))
}

func BindQuery(c *gin.Context, dst any) error { return bindErr(c.ShouldBindQuery(dst)) }

func BindForm(c *gin.Context, dst any) error {
	return bindErr(c.ShouldBindWith(dst, binding.FormMultipart))
}

func bindErr(err error) error {
	if err == nil {
		return nil
	}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		fields := map[string][]string{}
		for _, fe := range ve {
			fields[fe.Field()] = append(fields[fe.Field()], fieldMessage(fe))
		}
		return apperr.Validation(fields)
	}
	var mbe *http.MaxBytesError
	if errors.As(err, &mbe) {
		return apperr.TooLarge(mbe.Limit)
	}
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	if errors.As(err, &syn) || errors.As(err, &typ) || strings.Contains(err.Error(), "unknown field") {
		return apperr.FieldError("body", "Isi permintaan tidak valid atau memuat field yang tidak dikenal.")
	}
	return apperr.FieldError("body", "Isi permintaan tidak dapat dibaca.")
}

func fieldMessage(fe validator.FieldError) string {
	name := fe.Field()
	isStr := fe.Kind() == reflect.String
	switch fe.Tag() {
	case "required":
		return name + " wajib diisi."
	case "required_if":
		return name + " wajib diisi pada kondisi ini."
	case "email":
		return "Format email tidak valid."
	case "max":
		if isStr {
			return fmt.Sprintf("%s maksimal %s karakter.", name, fe.Param())
		}
		return fmt.Sprintf("%s maksimal %s.", name, fe.Param())
	case "min":
		if isStr {
			return fmt.Sprintf("%s minimal %s karakter.", name, fe.Param())
		}
		return fmt.Sprintf("%s minimal %s.", name, fe.Param())
	case "gt":
		return fmt.Sprintf("%s harus lebih dari %s.", name, fe.Param())
	case "oneof":
		return name + " harus salah satu dari: " + strings.ReplaceAll(fe.Param(), " ", ", ") + "."
	case "eqfield":
		return name + " tidak cocok."
	case "phone":
		return "Nomor telepon harus 8 sampai 15 digit (boleh diawali +)."
	case "strongpw":
		return "Kata sandi minimal 8 karakter dan memuat huruf besar, huruf kecil, dan angka."
	}
	return name + " tidak valid."
}
