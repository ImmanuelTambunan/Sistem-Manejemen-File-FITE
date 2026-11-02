package handler

import (
	"net/http"
	"strings"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type UserHandler struct {
	db *gorm.DB
}

func NewUserHandler(db *gorm.DB) *UserHandler {
	return &UserHandler{db: db}
}

type createUserRequest struct {
	Name    string  `json:"name" binding:"required,max=100"`
	Email   string  `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,strongpw"`
	Role    string  `json:"role" binding:"required,oneof=super_admin dekan baak kaprodi dosen mahasiswa"`
	ProdiID *uint   `json:"prodi_id"`
	NIM     *string `json:"nim" binding:"omitempty,max=30"`
	NIP     *string `json:"nip" binding:"omitempty,max=30"`
	Phone   *string `json:"phone" binding:"omitempty,phone"`
}

type updateUserRequest struct {
	Name    string  `json:"name" binding:"required,max=100"`
	Email   string  `json:"email" binding:"required,email,max=255"`
	ProdiID *uint   `json:"prodi_id"`
	NIM     *string `json:"nim" binding:"omitempty,max=30"`
	NIP     *string `json:"nip" binding:"omitempty,max=30"`
	Phone   *string `json:"phone" binding:"omitempty,phone"`
}

type updateRoleRequest struct {
	Role    string `json:"role" binding:"required,oneof=super_admin dekan baak kaprodi dosen mahasiswa"`
	ProdiID *uint  `json:"prodi_id"`
}

type updateStatusRequest struct {
	IsActive bool `json:"is_active"`
}

type resetPasswordRequest struct {
	Password             string `json:"password" binding:"required,strongpw"`
	PasswordConfirmation string `json:"password_confirmation" binding:"required,eqfield=Password"`
}

type userFilter struct {
	Q        string `form:"q"`
	Role     string `form:"role"`
	Prodi    string `form:"prodi"`
	IsActive *bool  `form:"is_active"`
	Page     *int   `form:"page" binding:"omitempty,min=1"`
	PerPage  *int   `form:"per_page" binding:"omitempty,min=1,max=100"`
}

type prodiUpdateRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	IsActive bool   `json:"is_active"`
}

func (h *UserHandler) ProdiIndex(c *gin.Context) error {
	var prodis []domain.Prodi
	if err := h.db.WithContext(c.Request.Context()).Order("id ASC").Find(&prodis).Error; err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "OK", prodis)
	return nil
}

func (h *UserHandler) ProdiUpdate(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var req prodiUpdateRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	var p domain.Prodi
	if err := h.db.WithContext(c.Request.Context()).First(&p, id).Error; err != nil {
		return err
	}
	p.Name = req.Name
	p.IsActive = req.IsActive
	if err := h.db.WithContext(c.Request.Context()).Save(&p).Error; err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "Program studi diperbarui", p)
	return nil
}

func (h *UserHandler) Roles(c *gin.Context) error {
	type roleStat struct {
		Key        string `json:"key"`
		Label      string `json:"label"`
		UsersCount int64  `json:"users_count"`
	}
	var roles []domain.Role
	if err := h.db.WithContext(c.Request.Context()).Order("id ASC").Find(&roles).Error; err != nil {
		return err
	}
	out := make([]roleStat, len(roles))
	for i, r := range roles {
		var cnt int64
		h.db.WithContext(c.Request.Context()).Model(&domain.User{}).Where("role_id = ?", r.ID).Count(&cnt)
		out[i] = roleStat{
			Key:        r.Key,
			Label:      r.Name,
			UsersCount: cnt,
		}
	}
	httpx.OK(c, http.StatusOK, "OK", out)
	return nil
}

func (h *UserHandler) Index(c *gin.Context) error {
	var f userFilter
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
	}

	q := h.db.WithContext(c.Request.Context()).Model(&domain.User{}).
		Preload("Role").Preload("Prodi")

	if f.Q != "" {
		p := "%" + strings.TrimSpace(f.Q) + "%"
		q = q.Where("users.name LIKE ? OR users.email LIKE ? OR users.nim LIKE ? OR users.nip LIKE ?", p, p, p, p)
	}
	if f.Role != "" {
		q = q.Joins("JOIN roles ON roles.id = users.role_id").Where("roles.`key` = ?", f.Role)
	}
	if f.Prodi != "" {
		q = q.Joins("JOIN prodi ON prodi.id = users.prodi_id").Where("prodi.code = ?", f.Prodi)
	}
	if f.IsActive != nil {
		q = q.Where("users.is_active = ?", *f.IsActive)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return err
	}

	var users []domain.User
	if err := q.Order("users.created_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&users).Error; err != nil {
		return err
	}

	out := make([]UserResponse, len(users))
	for i := range users {
		out[i] = toUserResponse(&users[i])
	}
	httpx.Paged(c, "OK", out, httpx.PageMeta(page, perPage, total))
	return nil
}

func (h *UserHandler) Store(c *gin.Context) error {
	var req createUserRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}

	fields := map[string][]string{}
	// Validate role specific rules
	switch req.Role {
	case domain.RoleMahasiswa:
		if req.NIM == nil || *req.NIM == "" {
			fields["nim"] = []string{"NIM wajib diisi untuk mahasiswa."}
		}
		if req.ProdiID == nil {
			fields["prodi_id"] = []string{"Program studi wajib dipilih untuk mahasiswa."}
		}
	case domain.RoleDosen, domain.RoleKaprodi:
		if req.NIP == nil || *req.NIP == "" {
			fields["nip"] = []string{"NIP wajib diisi untuk " + req.Role + "."}
		}
		if req.ProdiID == nil {
			fields["prodi_id"] = []string{"Program studi wajib dipilih untuk " + req.Role + "."}
		}
	case domain.RoleBaak:
		if req.NIP == nil || *req.NIP == "" {
			fields["nip"] = []string{"NIP wajib diisi untuk BAAK."}
		}
	}

	// Check duplicates
	email := strings.ToLower(strings.TrimSpace(req.Email))
	var cnt int64
	h.db.WithContext(c.Request.Context()).Model(&domain.User{}).Where("email = ?", email).Count(&cnt)
	if cnt > 0 {
		fields["email"] = []string{"Email sudah terdaftar."}
	}
	if req.NIM != nil && *req.NIM != "" {
		h.db.WithContext(c.Request.Context()).Model(&domain.User{}).Where("nim = ?", *req.NIM).Count(&cnt)
		if cnt > 0 {
			fields["nim"] = []string{"NIM sudah terdaftar."}
		}
	}
	if req.NIP != nil && *req.NIP != "" {
		h.db.WithContext(c.Request.Context()).Model(&domain.User{}).Where("nip = ?", *req.NIP).Count(&cnt)
		if cnt > 0 {
			fields["nip"] = []string{"NIP sudah terdaftar."}
		}
	}

	if len(fields) > 0 {
		return apperr.Validation(fields)
	}

	var role domain.Role
	if err := h.db.WithContext(c.Request.Context()).Where("`key` = ?", req.Role).First(&role).Error; err != nil {
		return apperr.FieldError("role", "Peran tidak valid.")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return err
	}

	user := domain.User{
		Name:               req.Name,
		Email:              email,
		PasswordHash:       string(hashed),
		RoleID:             role.ID,
		Role:               role,
		ProdiID:            req.ProdiID,
		NIM:                req.NIM,
		NIP:                req.NIP,
		Phone:              req.Phone,
		IsActive:           true,
		MustChangePassword: true,
	}

	if err := h.db.WithContext(c.Request.Context()).Create(&user).Error; err != nil {
		return err
	}

	_ = h.db.WithContext(c.Request.Context()).Preload("Role").Preload("Prodi").First(&user, user.ID)
	httpx.OK(c, http.StatusCreated, "Pengguna dibuat", toUserResponse(&user))
	return nil
}

func (h *UserHandler) Show(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var u domain.User
	if err := h.db.WithContext(c.Request.Context()).Preload("Role").Preload("Prodi").First(&u, id).Error; err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "OK", toUserResponse(&u))
	return nil
}

func (h *UserHandler) Update(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var u domain.User
	if err := h.db.WithContext(c.Request.Context()).Preload("Role").First(&u, id).Error; err != nil {
		return err
	}

	var req updateUserRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}

	fields := map[string][]string{}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	var cnt int64
	h.db.WithContext(c.Request.Context()).Model(&domain.User{}).Where("email = ? AND id <> ?", email, id).Count(&cnt)
	if cnt > 0 {
		fields["email"] = []string{"Email sudah terdaftar."}
	}
	if req.NIM != nil && *req.NIM != "" {
		h.db.WithContext(c.Request.Context()).Model(&domain.User{}).Where("nim = ? AND id <> ?", *req.NIM, id).Count(&cnt)
		if cnt > 0 {
			fields["nim"] = []string{"NIM sudah terdaftar."}
		}
	}
	if req.NIP != nil && *req.NIP != "" {
		h.db.WithContext(c.Request.Context()).Model(&domain.User{}).Where("nip = ? AND id <> ?", *req.NIP, id).Count(&cnt)
		if cnt > 0 {
			fields["nip"] = []string{"NIP sudah terdaftar."}
		}
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}

	u.Name = req.Name
	u.Email = email
	u.ProdiID = req.ProdiID
	u.NIM = req.NIM
	u.NIP = req.NIP
	u.Phone = req.Phone

	if err := h.db.WithContext(c.Request.Context()).Save(&u).Error; err != nil {
		return err
	}
	_ = h.db.WithContext(c.Request.Context()).Preload("Role").Preload("Prodi").First(&u, u.ID)
	httpx.OK(c, http.StatusOK, "Pengguna diperbarui", toUserResponse(&u))
	return nil
}

func (h *UserHandler) UpdateRole(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	currentUser := middleware.CurrentUser(c)
	if currentUser.ID == id {
		return apperr.Conflict("Tidak dapat mengubah peran diri sendiri.")
	}

	var req updateRoleRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}

	var targetRole domain.Role
	if err := h.db.WithContext(c.Request.Context()).Where("`key` = ?", req.Role).First(&targetRole).Error; err != nil {
		return apperr.FieldError("role", "Peran tidak valid.")
	}

	now := time.Now().UTC()
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var targetUser domain.User
		if err := tx.Preload("Role").First(&targetUser, id).Error; err != nil {
			return err
		}

		// Check if demoting the last active super_admin
		if targetUser.Role.Key == domain.RoleSuperAdmin && req.Role != domain.RoleSuperAdmin {
			var saCount int64
			tx.Model(&domain.User{}).
				Joins("JOIN roles ON roles.id = users.role_id").
				Where("roles.`key` = ? AND users.is_active = ?", domain.RoleSuperAdmin, true).
				Count(&saCount)
			if saCount <= 1 {
				return apperr.Conflict("Tidak dapat menurunkan Super Admin aktif terakhir.")
			}
		}

		updates := map[string]any{
			"role_id": targetRole.ID,
		}
		if req.ProdiID != nil {
			updates["prodi_id"] = req.ProdiID
		}

		if err := tx.Model(&domain.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}

		// Revoke all sessions for this user
		if err := tx.Model(&domain.AuthSession{}).Where("user_id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error; err != nil {
			return err
		}

		// Sync documents.owner_role for all documents owned by this user
		return tx.Model(&domain.Document{}).Where("owner_id = ?", id).
			Update("owner_role", targetRole.Key).Error
	})
	if err != nil {
		return err
	}

	var updatedUser domain.User
	_ = h.db.WithContext(c.Request.Context()).Preload("Role").Preload("Prodi").First(&updatedUser, id)
	httpx.OK(c, http.StatusOK, "Peran pengguna diperbarui", toUserResponse(&updatedUser))
	return nil
}

func (h *UserHandler) UpdateStatus(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	currentUser := middleware.CurrentUser(c)
	if currentUser.ID == id {
		return apperr.Conflict("Tidak dapat menonaktifkan akun sendiri.")
	}

	var req updateStatusRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}

	now := time.Now().UTC()
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var targetUser domain.User
		if err := tx.Preload("Role").First(&targetUser, id).Error; err != nil {
			return err
		}

		if !req.IsActive && targetUser.Role.Key == domain.RoleSuperAdmin {
			var saCount int64
			tx.Model(&domain.User{}).
				Joins("JOIN roles ON roles.id = users.role_id").
				Where("roles.`key` = ? AND users.is_active = ?", domain.RoleSuperAdmin, true).
				Count(&saCount)
			if saCount <= 1 {
				return apperr.Conflict("Tidak dapat menonaktifkan Super Admin aktif terakhir.")
			}
		}

		if err := tx.Model(&domain.User{}).Where("id = ?", id).Update("is_active", req.IsActive).Error; err != nil {
			return err
		}

		if !req.IsActive {
			// Revoke all sessions
			if err := tx.Model(&domain.AuthSession{}).Where("user_id = ? AND revoked_at IS NULL", id).
				Update("revoked_at", now).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	var updatedUser domain.User
	_ = h.db.WithContext(c.Request.Context()).Preload("Role").Preload("Prodi").First(&updatedUser, id)
	httpx.OK(c, http.StatusOK, "Status pengguna diperbarui", toUserResponse(&updatedUser))
	return nil
}

func (h *UserHandler) ResetPassword(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var req resetPasswordRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&domain.User{}).Where("id = ?", id).
			Updates(map[string]any{"password": string(hashed), "must_change_password": true}).Error; err != nil {
			return err
		}
		// Revoke all user's sessions
		return tx.Model(&domain.AuthSession{}).Where("user_id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error
	})
	if err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "Kata sandi diatur ulang", nil)
	return nil
}

func (h *UserHandler) Destroy(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	currentUser := middleware.CurrentUser(c)
	if currentUser.ID == id {
		return apperr.Conflict("Tidak dapat menghapus diri sendiri.")
	}

	now := time.Now().UTC()
	err = h.db.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		var targetUser domain.User
		if err := tx.Preload("Role").First(&targetUser, id).Error; err != nil {
			return err
		}

		if targetUser.Role.Key == domain.RoleSuperAdmin {
			var saCount int64
			tx.Model(&domain.User{}).
				Joins("JOIN roles ON roles.id = users.role_id").
				Where("roles.`key` = ? AND users.is_active = ?", domain.RoleSuperAdmin, true).
				Count(&saCount)
			if saCount <= 1 {
				return apperr.Conflict("Tidak dapat menghapus Super Admin terakhir.")
			}
		}

		if err := tx.Delete(&domain.User{}, id).Error; err != nil {
			return err
		}

		return tx.Model(&domain.AuthSession{}).Where("user_id = ? AND revoked_at IS NULL", id).
			Update("revoked_at", now).Error
	})
	if err != nil {
		return err
	}
	httpx.NoContent(c)
	return nil
}
