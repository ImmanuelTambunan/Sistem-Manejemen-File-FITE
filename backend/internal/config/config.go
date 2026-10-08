package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env, HTTPAddr, DBDSN        string
	JWTSecret                   []byte
	JWTIssuer, JWTAudience      string
	AccessTTL, RefreshTTL       time.Duration
	SessionMaxAge               time.Duration
	AllowedOrigin, CookieDomain string
	CookieSecure                bool
	TrustedProxies              []string
	GCSBucket, GCSCredentials   string
	RetentionYears, GraceDays   int
	Timezone                    string
	MaxFileBytes                int64 // 10 MB
	MaxUploadBody, MaxJSONBody  int64 // 11 MB, 1 MB
}

func (c Config) IsProd() bool { return c.Env == "production" }

func Load() (Config, error) {
	var err error
	c := Config{
		Env:            env("APP_ENV", "development"),
		HTTPAddr:       env("HTTP_ADDR", ":8080"),
		DBDSN:          os.Getenv("DB_DSN"),
		JWTSecret:      []byte(os.Getenv("JWT_SECRET")),
		JWTIssuer:      env("JWT_ISSUER", "fite-arsip-api"),
		JWTAudience:    env("JWT_AUDIENCE", "fite-arsip-web"),
		AllowedOrigin:  os.Getenv("ALLOWED_ORIGIN"),
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),
		CookieSecure:   env("COOKIE_SECURE", "false") == "true",
		GCSBucket:      os.Getenv("GCS_BUCKET"),
		GCSCredentials: os.Getenv("GCS_CREDENTIALS_FILE"),
		Timezone:       env("TIMEZONE", "Asia/Jakarta"),
		MaxFileBytes:   10 << 20,
		MaxUploadBody:  11 << 20,
		MaxJSONBody:    1 << 20,
	}
	for _, p := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}
	if c.AccessTTL, err = duration("ACCESS_TTL", 15*time.Minute); err != nil {
		return c, err
	}
	if c.RefreshTTL, err = duration("REFRESH_TTL", 7*24*time.Hour); err != nil {
		return c, err
	}
	if c.SessionMaxAge, err = duration("SESSION_MAX_AGE", 30*24*time.Hour); err != nil {
		return c, err
	}
	if c.RetentionYears, err = integer("RETENTION_YEARS", 8); err != nil {
		return c, err
	}
	if c.GraceDays, err = integer("RETENTION_GRACE_DAYS", 90); err != nil {
		return c, err
	}

	var problems []string
	if c.DBDSN == "" {
		problems = append(problems, "DB_DSN wajib")
	}
	if len(c.JWTSecret) < 32 {
		problems = append(problems, "JWT_SECRET minimal 32 byte")
	}
	if c.AllowedOrigin == "" {
		problems = append(problems, "ALLOWED_ORIGIN wajib")
	}
	// GCS_BUCKET mulai diwajibkan pada fase penyimpanan berkas (fase 5).
	if c.IsProd() && !c.CookieSecure {
		problems = append(problems, "COOKIE_SECURE wajib true di production")
	}
	if len(problems) > 0 {
		return c, errors.New("konfigurasi tidak valid: " + strings.Join(problems, "; "))
	}
	return c, nil
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func duration(k string, def time.Duration) (time.Duration, error) {
	if v := os.Getenv(k); v != "" {
		return time.ParseDuration(v)
	}
	return def, nil
}

func integer(k string, def int) (int, error) {
	if v := os.Getenv(k); v != "" {
		return strconv.Atoi(v)
	}
	return def, nil
}
