package handler

import (
	"net/http"

	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"
	"fite-arsip-api/internal/service"

	"github.com/gin-gonic/gin"
)

type VerificationHandler struct {
	svc *service.VerificationService
}

func NewVerificationHandler(svc *service.VerificationService) *VerificationHandler {
	return &VerificationHandler{svc: svc}
}

type verificationQuery struct {
	Status     string `form:"status" binding:"omitempty,oneof=pending approved rejected"`
	Prodi      string `form:"prodi" binding:"omitempty,oneof=IF SI TE"`
	DocumentID *uint  `form:"document_id" binding:"omitempty,gt=0"`
	Page       *int   `form:"page" binding:"omitempty,min=1"`
	PerPage    *int   `form:"per_page" binding:"omitempty,min=1,max=100"`
}

type decideRequest struct {
	Status string `json:"status" binding:"required,oneof=approved rejected"`
	Reason string `json:"reason" binding:"required_if=Status rejected,omitempty,min=5,max=500"`
}

func (h *VerificationHandler) Index(c *gin.Context) error {
	var q verificationQuery
	if err := httpx.BindQuery(c, &q); err != nil {
		return err
	}
	f := service.VerificationFilter{Status: q.Status, Prodi: q.Prodi, DocumentID: q.DocumentID, Page: 1, PerPage: 15}
	if q.Page != nil {
		f.Page = *q.Page
	}
	if q.PerPage != nil {
		f.PerPage = *q.PerPage
	}
	items, total, err := h.svc.List(c.Request.Context(), f)
	if err != nil {
		return err
	}
	out := make([]VerificationResponse, 0, len(items))
	for i := range items {
		out = append(out, toVerificationResponse(&items[i]))
	}
	httpx.Paged(c, "OK", out, httpx.PageMeta(f.Page, f.PerPage, total))
	return nil
}

func (h *VerificationHandler) Show(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	v, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "OK", toVerificationResponse(v))
	return nil
}

func (h *VerificationHandler) Update(c *gin.Context) error {
	id, err := idParam(c)
	if err != nil {
		return err
	}
	var req decideRequest
	if err := httpx.BindJSON(c, &req); err != nil {
		return err
	}
	v, err := h.svc.Decide(c.Request.Context(), middleware.CurrentUser(c), id, req.Status, req.Reason)
	if err != nil {
		return err
	}
	msg := "Berkas disetujui"
	if req.Status == "rejected" {
		msg = "Berkas ditolak"
	}
	httpx.OK(c, http.StatusOK, msg, toVerificationResponse(v))
	return nil
}
