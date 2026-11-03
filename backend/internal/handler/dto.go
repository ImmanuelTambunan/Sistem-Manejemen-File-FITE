package handler

import (
	"strconv"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/domain"

	"github.com/gin-gonic/gin"
)

func ts(t time.Time) string { return t.UTC().Format("2006-01-02T15:04:05Z") }

func tsp(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := ts(*t)
	return &s
}

func idParam(c *gin.Context) (uint, error) {
	n, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || n == 0 {
		return 0, apperr.NotFound("")
	}
	return uint(n), nil
}

type ProdiRef struct {
	ID   uint   `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

func toProdiRef(p *domain.Prodi) *ProdiRef {
	if p == nil || p.ID == 0 {
		return nil
	}
	return &ProdiRef{ID: p.ID, Code: p.Code, Name: p.Name}
}

type UserResponse struct {
	ID                 uint      `json:"id"`
	Name               string    `json:"name"`
	Email              string    `json:"email"`
	Role               string    `json:"role"`
	RoleLabel          string    `json:"role_label"`
	NIM                *string   `json:"nim"`
	NIP                *string   `json:"nip"`
	Phone              *string   `json:"phone"`
	Prodi              *ProdiRef `json:"prodi"`
	IsActive           bool      `json:"is_active"`
	MustChangePassword bool      `json:"must_change_password"`
}

func toUserResponse(u *domain.User) UserResponse {
	return UserResponse{
		ID:                 u.ID,
		Name:               u.Name,
		Email:              u.Email,
		Role:               u.Role.Key,
		RoleLabel:          u.Role.Name,
		NIM:                u.NIM,
		NIP:                u.NIP,
		Phone:              u.Phone,
		Prodi:              toProdiRef(u.Prodi),
		IsActive:           u.IsActive,
		MustChangePassword: u.MustChangePassword,
	}
}

type UserRef struct {
	ID   uint    `json:"id"`
	Name string  `json:"name"`
	Role string  `json:"role"`
	NIM  *string `json:"nim"`
	NIP  *string `json:"nip"`
}

func toUserRef(u domain.User) UserRef {
	return UserRef{ID: u.ID, Name: u.Name, Role: u.Role.Key, NIM: u.NIM, NIP: u.NIP}
}

type DocumentResponse struct {
	ID                 uint      `json:"id"`
	Title              string    `json:"title"`
	Description        *string   `json:"description"`
	OriginalName       string    `json:"original_name"`
	FileName           string    `json:"file_name"`
	Extension          string    `json:"extension"`
	MimeType           string    `json:"mime_type"`
	SizeKB             uint      `json:"size_kb"`
	FileSizeBytes      int64     `json:"file_size_bytes"`
	Category           struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"category"`
	Prodi              *ProdiRef `json:"prodi"`
	Uploader           UserRef   `json:"uploader"`
	Owner              UserRef   `json:"owner"`
	Visibility         string    `json:"visibility"`
	VerificationStatus *string   `json:"verification_status"`
	ArchivedAt         *string   `json:"archived_at"`
	CreatedAt          string    `json:"created_at"`
	Links              struct {
		Self     string `json:"self"`
		Download string `json:"download"`
	} `json:"links"`
}

func toDocumentResponse(d *domain.Document) DocumentResponse {
	r := DocumentResponse{
		ID:                 d.ID,
		Title:              d.Title,
		Description:        d.Description,
		OriginalName:       d.OriginalName,
		FileName:           d.OriginalName,
		Extension:          d.Extension,
		MimeType:           d.MimeType,
		SizeKB:             d.SizeKB,
		FileSizeBytes:      int64(d.SizeKB) * 1024,
		Prodi:              toProdiRef(d.Prodi),
		Uploader:           toUserRef(d.Uploader),
		Owner:              toUserRef(d.Owner),
		Visibility:         d.Visibility,
		VerificationStatus: d.VerificationStatus,
		ArchivedAt:         tsp(d.ArchivedAt),
		CreatedAt:          ts(d.CreatedAt),
	}
	r.Category.ID, r.Category.Name = d.Category.ID, d.Category.Name
	base := "/api/v1/documents/" + strconv.FormatUint(uint64(d.ID), 10)
	r.Links.Self, r.Links.Download = base, base+"/download"
	return r
}

func toDocumentResponses(ds []domain.Document) []DocumentResponse {
	out := make([]DocumentResponse, 0, len(ds))
	for i := range ds {
		out = append(out, toDocumentResponse(&ds[i]))
	}
	return out
}

type VerificationResponse struct {
	ID       uint `json:"id"`
	Document struct {
		ID    uint    `json:"id"`
		Title string  `json:"title"`
		Prodi *string `json:"prodi"`
	} `json:"document"`
	Status      string  `json:"status"`
	Reason      *string `json:"reason"`
	RequestedBy struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"requested_by"`
	VerifiedBy *struct {
		ID   uint   `json:"id"`
		Name string `json:"name"`
	} `json:"verified_by"`
	RequestedAt string  `json:"requested_at"`
	VerifiedAt  *string `json:"verified_at"`
}

func toVerificationResponse(v *domain.Verification) VerificationResponse {
	r := VerificationResponse{
		ID:          v.ID,
		Status:      v.Status,
		Reason:      v.Reason,
		RequestedAt: ts(v.RequestedAt),
		VerifiedAt:  tsp(v.VerifiedAt),
	}
	r.Document.ID, r.Document.Title = v.Document.ID, v.Document.Title
	if v.Document.Prodi != nil {
		code := v.Document.Prodi.Code
		r.Document.Prodi = &code
	}
	r.RequestedBy.ID, r.RequestedBy.Name = v.Requester.ID, v.Requester.Name
	if v.Verifier != nil {
		r.VerifiedBy = &struct {
			ID   uint   `json:"id"`
			Name string `json:"name"`
		}{v.Verifier.ID, v.Verifier.Name}
	}
	return r
}
