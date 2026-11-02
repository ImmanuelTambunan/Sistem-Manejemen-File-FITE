package app_test

import (
	"testing"

	"fite-arsip-api/internal/app"
	"fite-arsip-api/internal/config"
)

func TestOpenDB_InvalidDSN(t *testing.T) {
	cfg := config.Config{
		DBDSN: "invalid-user:invalid-pass@tcp(127.0.0.1:9999)/nonexistent",
	}

	// OpenDB sets pool parameters and tries to open connection
	db, err := app.OpenDB(cfg)
	// GORM mysql driver configures connection pool, check that function does not panic
	if err == nil && db != nil {
		sqlDB, err := db.DB()
		if err == nil {
			defer sqlDB.Close()
		}
	}
}
