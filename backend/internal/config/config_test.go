package config_test

import (
	"os"
	"testing"
	"time"

	"fite-arsip-api/internal/config"
)

func setEnvMap(env map[string]string) func() {
	prev := make(map[string]string)
	for k, v := range env {
		prev[k] = os.Getenv(k)
		_ = os.Setenv(k, v)
	}
	return func() {
		for k := range env {
			if old, ok := prev[k]; ok && old != "" {
				_ = os.Setenv(k, old)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}
}

func TestConfigLoadSuccess(t *testing.T) {
	cleanup := setEnvMap(map[string]string{
		"APP_ENV":              "development",
		"HTTP_ADDR":            ":8080",
		"DB_DSN":               "fite_app:secret@tcp(127.0.0.1:3306)/fite_arsip",
		"JWT_SECRET":           "1234567890123456789012345678901234",
		"ALLOWED_ORIGIN":       "http://localhost:3000",
		"ACCESS_TTL":           "15m",
		"REFRESH_TTL":          "168h",
		"SESSION_MAX_AGE":      "720h",
		"RETENTION_YEARS":      "8",
		"RETENTION_GRACE_DAYS": "90",
		"TIMEZONE":             "Asia/Jakarta",
	})
	defer cleanup()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Env != "development" {
		t.Errorf("expected Env=development, got %s", cfg.Env)
	}
	if cfg.AccessTTL != 15*time.Minute {
		t.Errorf("expected AccessTTL 15m, got %v", cfg.AccessTTL)
	}
	if cfg.RetentionYears != 8 {
		t.Errorf("expected RetentionYears 8, got %d", cfg.RetentionYears)
	}
	if cfg.GraceDays != 90 {
		t.Errorf("expected GraceDays 90, got %d", cfg.GraceDays)
	}
	if cfg.IsProd() {
		t.Errorf("expected IsProd to be false")
	}
	if cfg.StorageDriver != "local" {
		t.Errorf("expected default StorageDriver=local, got %s", cfg.StorageDriver)
	}
}

func TestConfigValidationFailures(t *testing.T) {
	cleanup := setEnvMap(map[string]string{
		"DB_DSN":         "",
		"JWT_SECRET":     "too-short",
		"ALLOWED_ORIGIN": "",
	})
	defer cleanup()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected config.Load to fail due to missing required fields")
	}
}

func TestConfigProductionValidation(t *testing.T) {
	cleanup := setEnvMap(map[string]string{
		"APP_ENV":        "production",
		"DB_DSN":         "fite_app:secret@tcp(127.0.0.1:3306)/fite_arsip",
		"JWT_SECRET":     "1234567890123456789012345678901234",
		"ALLOWED_ORIGIN": "https://arsip.fite.del.ac.id",
		"COOKIE_SECURE":  "false",
		"STORAGE_DRIVER": "gcs",
		"GCS_BUCKET":     "",
	})
	defer cleanup()

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected config.Load to fail in production with missing GCS_BUCKET and insecure cookie")
	}
}

func TestConfigStorageDriver(t *testing.T) {
	baseEnv := map[string]string{
		"APP_ENV":        "development",
		"DB_DSN":         "fite_app:secret@tcp(127.0.0.1:3306)/fite_arsip",
		"JWT_SECRET":     "1234567890123456789012345678901234",
		"ALLOWED_ORIGIN": "http://localhost:3000",
	}

	t.Run("explicit local driver", func(t *testing.T) {
		env := make(map[string]string)
		for k, v := range baseEnv {
			env[k] = v
		}
		env["STORAGE_DRIVER"] = "local"
		env["STORAGE_LOCAL_PATH"] = "./custom_uploads"
		cleanup := setEnvMap(env)
		defer cleanup()

		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.StorageDriver != "local" {
			t.Errorf("expected StorageDriver=local, got %s", cfg.StorageDriver)
		}
		if cfg.StorageLocalPath != "./custom_uploads" {
			t.Errorf("expected StorageLocalPath=./custom_uploads, got %s", cfg.StorageLocalPath)
		}
	})

	t.Run("invalid driver rejected", func(t *testing.T) {
		env := make(map[string]string)
		for k, v := range baseEnv {
			env[k] = v
		}
		env["STORAGE_DRIVER"] = "ftp"
		cleanup := setEnvMap(env)
		defer cleanup()

		_, err := config.Load()
		if err == nil {
			t.Fatal("expected error for invalid STORAGE_DRIVER ftp")
		}
	})
}
