package handler

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/config"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db  *gorm.DB
	cfg config.Config
}

func NewDashboardHandler(db *gorm.DB, cfg config.Config) *DashboardHandler {
	return &DashboardHandler{db: db, cfg: cfg}
}

func (h *DashboardHandler) Summary(c *gin.Context) error {
	user := middleware.CurrentUser(c)
	ctx := c.Request.Context()

	base := h.db.WithContext(ctx).Model(&domain.Document{}).Scopes(service.VisibleTo(user))
	cards := gin.H{}

	todayStart := time.Now().UTC().Truncate(24 * time.Hour)

	switch user.Role.Key {
	case domain.RoleSuperAdmin:
		var totalDocs, totalUsers, pendingVerifications int64
		var totalKB uint
		h.db.WithContext(ctx).Model(&domain.Document{}).Count(&totalDocs)
		h.db.WithContext(ctx).Model(&domain.User{}).Where("is_active = ?", true).Count(&totalUsers)
		h.db.WithContext(ctx).Model(&domain.Verification{}).Where("status = ?", "pending").Count(&pendingVerifications)
		type sumResult struct{ Total uint }
		var s sumResult
		h.db.WithContext(ctx).Model(&domain.Document{}).Select("COALESCE(SUM(size_kb), 0) as total").Scan(&s)
		totalKB = s.Total

		cards["total_documents"] = totalDocs
		cards["total_users"] = totalUsers
		cards["pending_verifications"] = pendingVerifications
		cards["storage_used_mb"] = math.Round(float64(totalKB)/1024*10) / 10

	case domain.RoleDekan:
		var totalDocs, myDocs, akreditasiDocs int64
		h.db.WithContext(ctx).Model(&domain.Document{}).Count(&totalDocs)
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ?", user.ID).Count(&myDocs)
		h.db.WithContext(ctx).Model(&domain.Document{}).
			Joins("JOIN categories ON categories.id = documents.category_id").
			Where("categories.name LIKE ?", "%Akreditasi%").Count(&akreditasiDocs)

		cards["total_faculty_documents"] = totalDocs
		cards["my_documents"] = myDocs
		cards["accreditation_documents"] = akreditasiDocs

	case domain.RoleBaak:
		var pendingVerifications, totalDocs, uploadsToday, myDocs int64
		h.db.WithContext(ctx).Model(&domain.Verification{}).Where("status = ?", "pending").Count(&pendingVerifications)
		h.db.WithContext(ctx).Model(&domain.Document{}).Count(&totalDocs)
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("created_at >= ?", todayStart).Count(&uploadsToday)
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("uploader_id = ?", user.ID).Count(&myDocs)

		cards["pending_verifications"] = pendingVerifications
		cards["total_documents"] = totalDocs
		cards["uploads_today"] = uploadsToday
		cards["my_documents"] = myDocs

	case domain.RoleKaprodi:
		var prodiDocs, dosenDocs, mhsDocs, myDocs int64
		if user.ProdiID != nil {
			h.db.WithContext(ctx).Model(&domain.Document{}).Where("prodi_id = ?", *user.ProdiID).Count(&prodiDocs)
			h.db.WithContext(ctx).Model(&domain.Document{}).Where("prodi_id = ? AND owner_role = ?", *user.ProdiID, domain.RoleMahasiswa).Count(&mhsDocs)
		}
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_role = ?", domain.RoleDosen).Count(&dosenDocs)
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ?", user.ID).Count(&myDocs)

		cards["prodi_documents"] = prodiDocs
		cards["faculty_lecturer_documents"] = dosenDocs
		cards["prodi_student_documents"] = mhsDocs
		cards["my_documents"] = myDocs

	case domain.RoleDosen:
		var myDocs, mhsDocs, pendingDocs int64
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ?", user.ID).Count(&myDocs)
		if user.ProdiID != nil {
			h.db.WithContext(ctx).Model(&domain.Document{}).Where("prodi_id = ? AND owner_role = ?", *user.ProdiID, domain.RoleMahasiswa).Count(&mhsDocs)
		}
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ? AND (verification_status = 'pending' OR verification_status = 'rejected')", user.ID).Count(&pendingDocs)

		cards["my_documents"] = myDocs
		cards["student_documents"] = mhsDocs
		cards["pending_or_rejected_documents"] = pendingDocs

	case domain.RoleMahasiswa:
		var myDocs, pendingDocs, approvedDocs, rejectedDocs int64
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ?", user.ID).Count(&myDocs)
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ? AND verification_status = ?", user.ID, "pending").Count(&pendingDocs)
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ? AND verification_status = ?", user.ID, "approved").Count(&approvedDocs)
		h.db.WithContext(ctx).Model(&domain.Document{}).Where("owner_id = ? AND verification_status = ?", user.ID, "rejected").Count(&rejectedDocs)

		cards["my_documents"] = myDocs
		cards["pending_documents"] = pendingDocs
		cards["approved_documents"] = approvedDocs
		cards["rejected_documents"] = rejectedDocs
	}

	data := gin.H{
		"role":  user.Role.Key,
		"cards": cards,
	}

	if user.Role.Key == domain.RoleSuperAdmin || user.Role.Key == domain.RoleDekan || user.Role.Key == domain.RoleBaak {
		type prodiStat struct {
			Prodi string `json:"prodi"`
			Total int64  `json:"total"`
		}
		var list []prodiStat
		var prodis []domain.Prodi
		h.db.WithContext(ctx).Find(&prodis)
		for _, p := range prodis {
			var cnt int64
			base.Session(&gorm.Session{}).Where("documents.prodi_id = ?", p.ID).Count(&cnt)
			list = append(list, prodiStat{Prodi: p.Code, Total: cnt})
		}
		data["documents_by_prodi"] = list
	}

	httpx.OK(c, http.StatusOK, "OK", data)
	return nil
}

func (h *DashboardHandler) DocumentsByProdi(c *gin.Context) error {
	yearStr := c.Query("year")
	var prodis []domain.Prodi
	if err := h.db.WithContext(c.Request.Context()).Find(&prodis).Error; err != nil {
		return err
	}

	type prodiTotal struct {
		Prodi string `json:"prodi"`
		Total int64  `json:"total"`
	}
	out := make([]prodiTotal, 0, len(prodis))

	for _, p := range prodis {
		q := h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).Where("prodi_id = ?", p.ID)
		if yearStr != "" {
			if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
				start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
				q = q.Where("created_at >= ? AND created_at < ?", start, end)
			}
		}
		var cnt int64
		_ = q.Count(&cnt).Error
		out = append(out, prodiTotal{Prodi: p.Code, Total: cnt})
	}

	httpx.OK(c, http.StatusOK, "OK", out)
	return nil
}

func (h *DashboardHandler) DocumentsByCategory(c *gin.Context) error {
	user := middleware.CurrentUser(c)
	prodi := c.Query("prodi")
	yearStr := c.Query("year")

	var cats []domain.Category
	if err := h.db.WithContext(c.Request.Context()).Find(&cats).Error; err != nil {
		return err
	}

	type catTotal struct {
		Category string `json:"category"`
		Total    int64  `json:"total"`
	}
	out := make([]catTotal, 0, len(cats))

	for _, cat := range cats {
		q := h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).
			Scopes(service.VisibleTo(user)).
			Where("documents.category_id = ?", cat.ID)
		if prodi != "" {
			q = q.Where("documents.prodi_id = (SELECT id FROM prodi WHERE code = ?)", prodi)
		}
		if yearStr != "" {
			if y, err := strconv.Atoi(yearStr); err == nil && y > 0 {
				start := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC)
				end := time.Date(y+1, 1, 1, 0, 0, 0, 0, time.UTC)
				q = q.Where("documents.created_at >= ? AND documents.created_at < ?", start, end)
			}
		}
		var cnt int64
		_ = q.Count(&cnt).Error
		out = append(out, catTotal{Category: cat.Name, Total: cnt})
	}

	httpx.OK(c, http.StatusOK, "OK", out)
	return nil
}

func (h *DashboardHandler) UploadsTrend(c *gin.Context) error {
	user := middleware.CurrentUser(c)
	fromStr := c.Query("from")
	toStr := c.Query("to")

	now := time.Now().UTC()
	to := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	from := to.AddDate(0, -12, 0)

	if fromStr != "" {
		t, err := time.Parse("2006-01", fromStr)
		if err != nil {
			return apperr.FieldError("from", "Format rentang harus YYYY-MM")
		}
		from = t
	}
	if toStr != "" {
		t, err := time.Parse("2006-01", toStr)
		if err != nil {
			return apperr.FieldError("to", "Format rentang harus YYYY-MM")
		}
		to = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	}

	monthsDiff := (to.Year()-from.Year())*12 + int(to.Month()-from.Month())
	if monthsDiff <= 0 || monthsDiff > 36 {
		return apperr.FieldError("range", "Rentang waktu tidak valid atau lebih dari 36 bulan.")
	}

	type trendItem struct {
		Month string `json:"month"`
		Total int64  `json:"total"`
	}
	out := make([]trendItem, 0, monthsDiff)

	cur := from
	for cur.Before(to) {
		next := cur.AddDate(0, 1, 0)
		var cnt int64
		h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).
			Scopes(service.VisibleTo(user)).
			Where("documents.created_at >= ? AND documents.created_at < ?", cur, next).
			Count(&cnt)

		out = append(out, trendItem{
			Month: cur.Format("2006-01"),
			Total: cnt,
		})
		cur = next
	}

	httpx.OK(c, http.StatusOK, "OK", out)
	return nil
}

func (h *DashboardHandler) VerificationStats(c *gin.Context) error {
	var pending, approved, rejected int64
	ctx := c.Request.Context()

	q := h.db.WithContext(ctx).Model(&domain.Verification{})
	if prodi := c.Query("prodi"); prodi != "" {
		q = q.Where("document_id IN (SELECT d.id FROM documents d JOIN prodi p ON p.id = d.prodi_id WHERE p.code = ?)", prodi)
	}
	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			q = q.Where("requested_at >= ?", t)
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			q = q.Where("requested_at <= ?", t)
		}
	}

	q.Session(&gorm.Session{}).Where("status = ?", "pending").Count(&pending)
	q.Session(&gorm.Session{}).Where("status = ?", "approved").Count(&approved)
	q.Session(&gorm.Session{}).Where("status = ?", "rejected").Count(&rejected)

	type avgResult struct {
		AvgSec float64
	}
	var avgRes avgResult
	h.db.WithContext(ctx).Raw(`
		SELECT COALESCE(AVG(TIMESTAMPDIFF(SECOND, requested_at, verified_at)), 0) as avg_sec 
		FROM document_verifications 
		WHERE status IN ('approved', 'rejected') AND verified_at IS NOT NULL
	`).Scan(&avgRes)

	avgHours := math.Round((avgRes.AvgSec/3600)*10) / 10

	httpx.OK(c, http.StatusOK, "OK", gin.H{
		"pending":              pending,
		"approved":             approved,
		"rejected":             rejected,
		"avg_processing_hours": avgHours,
	})
	return nil
}

func (h *DashboardHandler) Storage(c *gin.Context) error {
	var totalDocs int64
	var totalKB uint
	h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).Count(&totalDocs)
	type sumRes struct{ Total uint }
	var sr sumRes
	h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).Select("COALESCE(SUM(size_kb), 0) as total").Scan(&sr)
	totalKB = sr.Total

	now := time.Now().UTC()
	cutoff8Years := now.AddDate(-h.cfg.RetentionYears, 0, 0)
	soonCutoff := cutoff8Years.AddDate(0, 0, 90)

	var expiringWithin90Days int64
	h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).
		Where("archived_at IS NOT NULL AND archived_at >= ? AND archived_at <= ?", cutoff8Years, soonCutoff).
		Count(&expiringWithin90Days)

	var softDeletedInGrace int64
	graceCutoff := now.AddDate(0, 0, -h.cfg.GraceDays)
	h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).Unscoped().
		Where("deleted_at IS NOT NULL AND deleted_at > ?", graceCutoff).
		Count(&softDeletedInGrace)

	httpx.OK(c, http.StatusOK, "OK", gin.H{
		"total_documents":             totalDocs,
		"total_size_mb":               math.Round(float64(totalKB)/1024*10) / 10,
		"expiring_within_90_days":     expiringWithin90Days,
		"soft_deleted_in_grace_period": softDeletedInGrace,
	})
	return nil
}

type storageLogFilter struct {
	Action     string `form:"action"`
	DocumentID *uint  `form:"document_id"`
	UserID     *uint  `form:"user_id"`
	From       string `form:"from"`
	To         string `form:"to"`
	Page       *int   `form:"page" binding:"omitempty,min=1"`
	PerPage    *int   `form:"per_page" binding:"omitempty,min=1,max=100"`
	Limit      *int   `form:"limit" binding:"omitempty,min=1,max=100"`
}

type StorageLogResponse struct {
	ID         uint           `json:"id"`
	Action     string         `json:"action"`
	DocumentID *uint          `json:"document_id"`
	User       *UserRef       `json:"user"`
	Meta       domain.JSONMap `json:"meta"`
	CreatedAt  string         `json:"created_at"`
}

func (h *DashboardHandler) StorageLogs(c *gin.Context) error {
	var f storageLogFilter
	if err := httpx.BindQuery(c, &f); err != nil {
		return err
	}
	page := 1
	if f.Page != nil {
		page = *f.Page
	}
	perPage := 15
	if f.PerPage != nil {
		perPage = *f.PerPage
	} else if f.Limit != nil {
		perPage = *f.Limit
	}

	q := h.db.WithContext(c.Request.Context()).Model(&domain.StorageLog{})
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.DocumentID != nil {
		q = q.Where("document_id = ?", *f.DocumentID)
	}
	if f.UserID != nil {
		q = q.Where("user_id = ?", *f.UserID)
	}
	if f.From != "" {
		if t, err := time.Parse("2006-01-02", f.From); err == nil {
			q = q.Where("created_at >= ?", t)
		}
	}
	if f.To != "" {
		if t, err := time.Parse("2006-01-02", f.To); err == nil {
			q = q.Where("created_at <= ?", t.AddDate(0, 0, 1))
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return err
	}

	var logs []domain.StorageLog
	if err := q.Order("created_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&logs).Error; err != nil {
		return err
	}

	out := make([]StorageLogResponse, len(logs))
	for i, l := range logs {
		var userRef *UserRef
		if l.UserID != nil {
			var u domain.User
			if err := h.db.WithContext(c.Request.Context()).Preload("Role").First(&u, *l.UserID).Error; err == nil {
				ur := toUserRef(u)
				userRef = &ur
			}
		}
		out[i] = StorageLogResponse{
			ID:         l.ID,
			Action:     l.Action,
			DocumentID: l.DocumentID,
			User:       userRef,
			Meta:       l.Meta,
			CreatedAt:  ts(l.CreatedAt),
		}
	}

	httpx.Paged(c, "OK", out, httpx.PageMeta(page, perPage, total))
	return nil
}
