package handler

import (
	"net/http"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type CategoryHandler struct {
	db *gorm.DB
}

func NewCategoryHandler(db *gorm.DB) *CategoryHandler {
	return &CategoryHandler{db: db}
}

type categoryRequest struct {
	Name          string            `json:"name" binding:"required,max=100"`
	AllowedRoles  domain.StringList `json:"allowed_roles" binding:"required"`
	IsOperational *bool             `json:"is_operational"`
}

func (h *CategoryHandler) Index(c *gin.Context) error {
	user := middleware.CurrentUser(c)
	var cats []domain.Category
	if err := h.db.WithContext(c.Request.Context()).Order("name ASC").Find(&cats).Error; err != nil {
		return err
	}

	filtered := make([]domain.Category, 0, len(cats))
	for _, cat := range cats {
		if user.Role.Key == domain.RoleSuperAdmin || cat.AllowedRoles.Contains(user.Role.Key) {
			filtered = append(filtered, cat)
		}
	}
	httpx.OK(c, http.StatusOK, "OK", filtered)
	return nil
}

func (h *CategoryHandler) Store(c *gin.Context) error {
	var req categoryRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}

	for _, r := range req.AllowedRoles {
		valid := false
		for _, ar := range domain.AllRoles {
			if r == ar {
				valid = true
				break
			}
		}
		if !valid {
			return apperr.FieldError("allowed_roles", "Peran tidak dikenal: "+r)
		}
	}

	var count int64
	h.db.WithContext(c.Request.Context()).Model(&domain.Category{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		return apperr.FieldError("name", "Nama kategori sudah digunakan.")
	}

	cat := domain.Category{
		Name:         req.Name,
		AllowedRoles: req.AllowedRoles,
	}
	if req.IsOperational != nil {
		cat.IsOperational = *req.IsOperational
	}

	if err := h.db.WithContext(c.Request.Context()).Create(&cat).Error; err != nil {
		return err
	}
	httpx.OK(c, http.StatusCreated, "Kategori dibuat", cat)
	return nil
}

func (h *CategoryHandler) Update(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var cat domain.Category
	if err := h.db.WithContext(c.Request.Context()).First(&cat, id).Error; err != nil {
		return err
	}

	var req categoryRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}

	for _, r := range req.AllowedRoles {
		valid := false
		for _, ar := range domain.AllRoles {
			if r == ar {
				valid = true
				break
			}
		}
		if !valid {
			return apperr.FieldError("allowed_roles", "Peran tidak dikenal: "+r)
		}
	}

	var dup int64
	h.db.WithContext(c.Request.Context()).Model(&domain.Category{}).
		Where("name = ? AND id <> ?", req.Name, id).Count(&dup)
	if dup > 0 {
		return apperr.FieldError("name", "Nama kategori sudah digunakan.")
	}

	cat.Name = req.Name
	cat.AllowedRoles = req.AllowedRoles
	if req.IsOperational != nil {
		cat.IsOperational = *req.IsOperational
	}

	if err := h.db.WithContext(c.Request.Context()).Save(&cat).Error; err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "Kategori diperbarui", cat)
	return nil
}

func (h *CategoryHandler) Destroy(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var count int64
	h.db.WithContext(c.Request.Context()).Model(&domain.Document{}).Where("category_id = ?", id).Count(&count)
	if count > 0 {
		return apperr.Conflict("Kategori masih dipakai berkas.")
	}

	res := h.db.WithContext(c.Request.Context()).Delete(&domain.Category{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperr.NotFound("")
	}
	httpx.NoContent(c)
	return nil
}
