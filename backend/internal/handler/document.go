package handler

import (
	"errors"
	"mime"
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
)

type DocumentHandler struct {
	svc *service.DocumentService
	cfg config.Config
}

func NewDocumentHandler(svc *service.DocumentService, cfg config.Config) *DocumentHandler {
	return &DocumentHandler{svc: svc, cfg: cfg}
}

type createDocumentForm struct {
	Title         string `form:"title" binding:"required,max=255"`
	Description   string `form:"description" binding:"omitempty,max=2000"`
	CategoryID    uint   `form:"category_id" binding:"required,gt=0"`
	Visibility    string `form:"visibility" binding:"omitempty,oneof=private public"`
	ProdiID       *uint  `form:"prodi_id" binding:"omitempty,gt=0"`
	SubjectUserID *uint  `form:"subject_user_id" binding:"omitempty,gt=0"`
	ArchivedAt    string `form:"archived_at"`
}

type updateDocumentRequest struct {
	Title       *string `json:"title" binding:"omitempty,min=1,max=255"`
	Description *string `json:"description" binding:"omitempty,max=2000"`
	CategoryID  *uint   `json:"category_id" binding:"omitempty,gt=0"`
}

type visibilityRequest struct {
	Visibility string `json:"visibility" binding:"required,oneof=private public"`
}

func (h *DocumentHandler) loadViewable(c *gin.Context) (*domain.Document, *domain.User, error) {
	id, err := idParam(c)
	if err != nil {
		return nil, nil, err
	}
	doc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		return nil, nil, err
	}
	user := middleware.CurrentUser(c)
	ok, err := h.svc.CanView(c.Request.Context(), user, doc)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, apperr.Forbidden("")
	}
	return doc, user, nil
}

func (h *DocumentHandler) Index(c *gin.Context) error {
	return h.list(c, false)
}

func (h *DocumentHandler) PublicIndex(c *gin.Context) error {
	return h.list(c, true)
}

func (h *DocumentHandler) list(c *gin.Context, public bool) error {
	var f service.DocumentFilter
	if err := httpx.BindQuery(c, &f); err != nil {
		return err
	}
	docs, total, err := h.svc.List(c.Request.Context(), middleware.CurrentUser(c), f, public)
	if err != nil {
		return err
	}
	httpx.Paged(c, "OK", toDocumentResponses(docs), httpx.PageMeta(f.PageNum(), f.PerPageNum(), total))
	return nil
}

func (h *DocumentHandler) Store(c *gin.Context) error {
	if c.ContentType() != "multipart/form-data" {
		return apperr.UnsupportedMedia("Gunakan multipart/form-data untuk mengunggah berkas.")
	}
	var form createDocumentForm
	if err := httpx.BindForm(c, &form); err != nil {
		return err
	}
	fh, err := c.FormFile("file")
	if errors.Is(err, http.ErrMissingFile) {
		return apperr.FieldError("file", "Berkas wajib dilampirkan.")
	}
	if err != nil {
		return err // termasuk MaxBytesError -> 413
	}

	var archived *time.Time
	if form.ArchivedAt != "" {
		t, perr := time.Parse(time.RFC3339, form.ArchivedAt)
		if perr != nil {
			if t, perr = time.Parse("2006-01-02", form.ArchivedAt); perr != nil {
				return apperr.FieldError("archived_at", "Format tanggal harus RFC3339 atau YYYY-MM-DD.")
			}
		}
		if t.After(time.Now()) {
			return apperr.FieldError("archived_at", "Tanggal arsip tidak boleh di masa depan.")
		}
		archived = &t
	}

	up, err := service.InspectUpload(fh, h.cfg.MaxFileBytes)
	if err != nil {
		return err
	}
	defer up.File.Close()

	var desc *string
	if form.Description != "" {
		desc = &form.Description
	}

	doc, err := h.svc.Create(c.Request.Context(), service.CreateInput{
		Actor:         middleware.CurrentUser(c),
		Title:         form.Title,
		Description:   desc,
		CategoryID:    form.CategoryID,
		Public:        form.Visibility == "public",
		ProdiID:       form.ProdiID,
		SubjectUserID: form.SubjectUserID,
		ArchivedAt:    archived,
		Upload:        up,
	})
	if err != nil {
		return err
	}

	msg := "Berkas berhasil diunggah"
	if doc.VerificationStatus != nil && *doc.VerificationStatus == "pending" {
		msg = "Berkas berhasil diunggah dan menunggu verifikasi"
	}
	httpx.OK(c, http.StatusCreated, msg, toDocumentResponse(doc))
	return nil
}

func (h *DocumentHandler) Show(c *gin.Context) error {
	doc, _, err := h.loadViewable(c)
	if err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "OK", toDocumentResponse(doc))
	return nil
}

func (h *DocumentHandler) Download(c *gin.Context) error {
	doc, user, err := h.loadViewable(c)
	if err != nil {
		return err
	}
	rc, size, err := h.svc.Download(c.Request.Context(), user, doc)
	if err != nil {
		return err
	}
	defer rc.Close()

	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": doc.OriginalName}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.DataFromReader(http.StatusOK, size, doc.MimeType, rc, nil)
	return nil
}

func (h *DocumentHandler) Update(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var req updateDocumentRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	doc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		return err
	}
	user := middleware.CurrentUser(c)
	if !service.CanUpdateDocument(user, doc) {
		return apperr.Forbidden("")
	}
	updated, err := h.svc.UpdateMeta(c.Request.Context(), user, doc,
		service.UpdateMetaInput{Title: req.Title, Description: req.Description, CategoryID: req.CategoryID})
	if err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "Berkas diperbarui", toDocumentResponse(updated))
	return nil
}

func (h *DocumentHandler) UpdateVisibility(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var req visibilityRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	doc, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		return err
	}
	user := middleware.CurrentUser(c)
	if !service.CanChangeVisibility(user, doc) {
		return apperr.Forbidden("")
	}
	updated, err := h.svc.ChangeVisibility(c.Request.Context(), user, id, req.Visibility)
	if err != nil {
		return err
	}
	msg := "Visibilitas berkas diperbarui"
	if updated.VerificationStatus != nil && *updated.VerificationStatus == "pending" {
		msg = "Visibilitas diubah ke publik dan menunggu verifikasi"
	}
	httpx.OK(c, http.StatusOK, msg, toDocumentResponse(updated))
	return nil
}

func (h *DocumentHandler) Destroy(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	force := false
	if v := c.Query("force"); v != "" {
		if force, err = strconv.ParseBool(v); err != nil {
			return apperr.FieldError("force", "Nilai force harus true atau false.")
		}
	}
	user := middleware.CurrentUser(c)
	isSA := user.Role.Key == domain.RoleSuperAdmin

	doc, err := h.svc.GetUnscoped(c.Request.Context(), id)
	if err != nil {
		return err
	}
	// berkas yang sudah di-soft-delete hanya dapat ditangani Super Admin dengan force=true
	if doc.DeletedAt.Valid && !(force && isSA) {
		return apperr.NotFound("")
	}
	if force {
		if !isSA {
			return apperr.Forbidden("Hapus permanen hanya untuk Super Admin.")
		}
	} else if !service.CanDeleteDocument(user, doc) {
		return apperr.Forbidden("")
	}

	if err := h.svc.Delete(c.Request.Context(), user, doc, force); err != nil {
		return err
	}
	httpx.NoContent(c)
	return nil
}
