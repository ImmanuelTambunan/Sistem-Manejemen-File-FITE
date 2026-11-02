package router

import (
	"context"
	"net/http"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/handler"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/service"

	"github.com/gin-gonic/gin"
)

type Handlers struct {
	Auth         *handler.AuthHandler
	Document     *handler.DocumentHandler
	Verification *handler.VerificationHandler
	Category     *handler.CategoryHandler
	Notification *handler.NotificationHandler
	User         *handler.UserHandler
	Dashboard    *handler.DashboardHandler
}

func New(cfg config.Config, auth *service.AuthService, h Handlers, ready func(context.Context) error) *gin.Engine {
	r := gin.New()
	r.HandleMethodNotAllowed = true
	r.MaxMultipartMemory = 1 << 20 // bagian multipart > 1 MB ditulis ke berkas sementara, bukan RAM
	_ = r.SetTrustedProxies(cfg.TrustedProxies)

	r.Use(
		middleware.RequestID(),
		middleware.Recover(),
		middleware.Logger(),
		middleware.CORS(cfg.AllowedOrigin),
		middleware.SecurityHeaders(cfg.IsProd()),
		middleware.LimitBody(cfg.MaxJSONBody, cfg.MaxUploadBody, http.MethodPost, "/api/v1/documents"),
	)

	r.NoRoute(func(c *gin.Context) { httpx.Fail(c, apperr.NotFound("")) })
	r.NoMethod(func(c *gin.Context) {
		httpx.Fail(c, apperr.New(http.StatusMethodNotAllowed, "method_not_allowed", "Metode HTTP tidak diizinkan."))
	})

	r.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	r.GET("/readyz", func(c *gin.Context) {
		if ready != nil {
			if err := ready(c.Request.Context()); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready"})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	w := httpx.Wrap
	api := r.Group("/api/v1")

	// ---- publik (dilindungi cookie/pembatas, tanpa Bearer) ----
	pub := api.Group("/auth")
	pub.POST("/login", w(h.Auth.Login))
	pub.POST("/refresh", w(h.Auth.Refresh))
	pub.POST("/logout", w(h.Auth.Logout))

	// ---- terautentikasi: Bearer + sesi aktif + 60 permintaan/menit/pengguna ----
	userLimiter := middleware.NewUserLimiter(60)
	authed := api.Group("", middleware.Auth(auth), userLimiter.Middleware())
	authed.GET("/auth/me", w(h.Auth.Me)) // boleh walau wajib ganti sandi
	authed.PUT("/auth/password", w(h.Auth.ChangePassword))

	// ---- semua rute lain: wajib sudah mengganti sandi awal ----
	g := authed.Group("", middleware.EnforcePasswordChange())
	g.PUT("/auth/profile", w(h.Auth.UpdateProfile))

	sa := middleware.RequireRoles(domain.RoleSuperAdmin)
	verifier := middleware.RequireRoles(domain.RoleBaak, domain.RoleSuperAdmin)

	// 2. Document Management
	docs := g.Group("/documents")
	docs.GET("", w(h.Document.Index))
	docs.POST("", w(h.Document.Store))
	docs.GET("/public", w(h.Document.PublicIndex))
	docs.GET("/categories", w(h.Category.Index))
	docs.POST("/categories", sa, w(h.Category.Store))
	docs.PUT("/categories/:id", sa, w(h.Category.Update))
	docs.DELETE("/categories/:id", sa, w(h.Category.Destroy))
	docs.GET("/:id", w(h.Document.Show))
	docs.GET("/:id/download", w(h.Document.Download))
	docs.PATCH("/:id", w(h.Document.Update))
	docs.PATCH("/:id/visibility", w(h.Document.UpdateVisibility))
	docs.DELETE("/:id", w(h.Document.Destroy))

	// 3. Verification & Approval
	ver := g.Group("/verifications", verifier)
	ver.GET("", w(h.Verification.Index))
	ver.GET("/:id", w(h.Verification.Show))
	ver.PATCH("/:id", w(h.Verification.Update))

	// 4. Notifications
	n := g.Group("/notifications")
	n.GET("", w(h.Notification.Index))
	n.GET("/unread-count", w(h.Notification.UnreadCount))
	n.PATCH("/read-all", w(h.Notification.ReadAll))
	n.PATCH("/:id/read", w(h.Notification.MarkRead))
	n.DELETE("/:id", w(h.Notification.Destroy))

	// 5. User & Role Management
	u := g.Group("/users")
	u.GET("/prodi", w(h.User.ProdiIndex)) // semua peran
	u.GET("/roles", sa, w(h.User.Roles))
	u.PUT("/prodi/:id", sa, w(h.User.ProdiUpdate))
	u.GET("", sa, w(h.User.Index))
	u.POST("", sa, w(h.User.Store))
	u.GET("/:id", sa, w(h.User.Show))
	u.PUT("/:id", sa, w(h.User.Update))
	u.PATCH("/:id/role", sa, w(h.User.UpdateRole))
	u.PATCH("/:id/status", sa, w(h.User.UpdateStatus))
	u.PATCH("/:id/password", sa, w(h.User.ResetPassword))
	u.DELETE("/:id", sa, w(h.User.Destroy))

	// 6. Dashboard Statistics
	d := g.Group("/dashboard")
	d.GET("/summary", w(h.Dashboard.Summary))
	d.GET("/documents-by-category", w(h.Dashboard.DocumentsByCategory))
	d.GET("/uploads-trend", w(h.Dashboard.UploadsTrend))
	d.GET("/documents-by-prodi", middleware.RequireRoles(domain.RoleSuperAdmin, domain.RoleDekan, domain.RoleBaak),
		w(h.Dashboard.DocumentsByProdi))
	d.GET("/verification-stats", verifier, w(h.Dashboard.VerificationStats))
	d.GET("/storage", sa, w(h.Dashboard.Storage))
	d.GET("/storage-logs", sa, w(h.Dashboard.StorageLogs))

	return r
}
