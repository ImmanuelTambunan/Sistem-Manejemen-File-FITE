package handler

import (
	"errors"
	"net/http"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/domain"
	"fite-arsip-api/internal/httpx"
	"fite-arsip-api/internal/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type NotificationHandler struct {
	db *gorm.DB
}

func NewNotificationHandler(db *gorm.DB) *NotificationHandler {
	return &NotificationHandler{db: db}
}

type notificationFilter struct {
	IsRead  *bool  `form:"is_read"`
	Type    string `form:"type" binding:"omitempty,max=50"`
	Page    *int   `form:"page" binding:"omitempty,min=1"`
	PerPage *int   `form:"per_page" binding:"omitempty,min=1,max=100"`
	Limit   *int   `form:"limit" binding:"omitempty,min=1,max=100"`
}

type NotificationItemResponse struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Data      domain.JSONMap `json:"data"`
	IsRead    bool           `json:"is_read"`
	CreatedAt string         `json:"created_at"`
}

func toNotificationResponse(n *domain.Notification) NotificationItemResponse {
	return NotificationItemResponse{
		ID:        n.ID,
		Type:      n.Type,
		Title:     n.Title,
		Message:   n.Message,
		Data:      n.Data,
		IsRead:    n.IsRead,
		CreatedAt: ts(n.CreatedAt),
	}
}

func (h *NotificationHandler) Index(c *gin.Context) error {
	user := middleware.CurrentUser(c)
	var f notificationFilter
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

	q := h.db.WithContext(c.Request.Context()).Model(&domain.Notification{}).Where("user_id = ?", user.ID)
	if f.IsRead != nil {
		q = q.Where("is_read = ?", *f.IsRead)
	}
	if f.Type != "" {
		q = q.Where("type = ?", f.Type)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return err
	}

	var items []domain.Notification
	err := q.Order("created_at DESC").Offset((page - 1) * perPage).Limit(perPage).Find(&items).Error
	if err != nil {
		return err
	}

	out := make([]NotificationItemResponse, 0, len(items))
	for i := range items {
		out = append(out, toNotificationResponse(&items[i]))
	}
	httpx.Paged(c, "OK", out, httpx.PageMeta(page, perPage, total))
	return nil
}

func (h *NotificationHandler) UnreadCount(c *gin.Context) error {
	user := middleware.CurrentUser(c)
	var count int64
	err := h.db.WithContext(c.Request.Context()).Model(&domain.Notification{}).
		Where("user_id = ? AND is_read = ?", user.ID, false).Count(&count).Error
	if err != nil {
		return err
	}
	httpx.OK(c, http.StatusOK, "OK", gin.H{"unread": count})
	return nil
}

func (h *NotificationHandler) ReadAll(c *gin.Context) error {
	user := middleware.CurrentUser(c)
	res := h.db.WithContext(c.Request.Context()).Model(&domain.Notification{}).
		Where("user_id = ? AND is_read = ?", user.ID, false).
		Update("is_read", true)
	if res.Error != nil {
		return res.Error
	}
	httpx.OK(c, http.StatusOK, "OK", gin.H{"updated": res.RowsAffected})
	return nil
}

func (h *NotificationHandler) MarkRead(c *gin.Context) error {
	id := c.Param("id")
	user := middleware.CurrentUser(c)

	var notif domain.Notification
	err := h.db.WithContext(c.Request.Context()).First(&notif, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("")
	}
	if err != nil {
		return err
	}

	if notif.UserID != user.ID {
		return apperr.Forbidden("")
	}

	if !notif.IsRead {
		notif.IsRead = true
		if err := h.db.WithContext(c.Request.Context()).Save(&notif).Error; err != nil {
			return err
		}
	}

	httpx.OK(c, http.StatusOK, "OK", toNotificationResponse(&notif))
	return nil
}

func (h *NotificationHandler) Destroy(c *gin.Context) error {
	id := c.Param("id")
	user := middleware.CurrentUser(c)

	var notif domain.Notification
	err := h.db.WithContext(c.Request.Context()).First(&notif, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return apperr.NotFound("")
	}
	if err != nil {
		return err
	}

	if notif.UserID != user.ID {
		return apperr.Forbidden("")
	}

	if err := h.db.WithContext(c.Request.Context()).Delete(&notif).Error; err != nil {
		return err
	}
	httpx.NoContent(c)
	return nil
}
