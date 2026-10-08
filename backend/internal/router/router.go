// Package router merakit middleware dan rute HTTP.
package router

import (
	"net/http"
	"time"

	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/httpx"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// New membuat engine Gin. Pada fase 1 hanya tersedia /api/v1/health.
func New(cfg config.Config, db *gorm.DB) *gin.Engine {
	if cfg.IsProd() {
		gin.SetMode(gin.ReleaseMode)
	}
	httpx.Init()
	r := gin.New()
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")
	v1.GET("/health", func(c *gin.Context) {
		status, dbState := http.StatusOK, "up"
		if db != nil {
			sqlDB, err := db.DB()
			if err != nil || sqlDB.Ping() != nil {
				status, dbState = http.StatusServiceUnavailable, "down"
			}
		}
		httpx.OK(c, status, "Layanan berjalan.", gin.H{
			"status":   map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK],
			"database": dbState,
			"time":     time.Now().UTC().Format(time.RFC3339),
		})
	})
	return r
}
