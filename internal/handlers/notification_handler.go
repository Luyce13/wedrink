package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"wedrink/internal/middleware"
	"wedrink/internal/render"
	"wedrink/internal/repository"
	"wedrink/internal/services"
)

type NotificationHandler struct {
	notifService *services.NotificationService
	renderer     *render.Renderer
}

func NewNotificationHandler(notifService *services.NotificationService, renderer *render.Renderer) *NotificationHandler {
	return &NotificationHandler{
		notifService: notifService,
		renderer:     renderer,
	}
}

// GetUnread (GET /notifications/unread) — backward compatible redirect to GetList
func (h *NotificationHandler) GetUnread(w http.ResponseWriter, r *http.Request) {
	h.GetList(w, r)
}

// GetList (GET /notifications/list) — returns paginated notifications with filter (unread/all)
func (h *NotificationHandler) GetList(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	filter := strings.TrimSpace(r.URL.Query().Get("filter"))
	if filter == "" {
		filter = "unread"
	}
	cursor := strings.TrimSpace(r.URL.Query().Get("cursor"))
	isAppend := r.URL.Query().Get("append") == "true"

	unreadCount, _ := h.notifService.GetUnreadCount(r.Context())

	result, err := h.notifService.GetNotificationsWithParams(r.Context(), repository.NotificationQueryParams{
		Filter: filter,
		Limit:  10,
		Cursor: cursor,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch notifications: %v", err), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Notifications": result.Notifications,
		"UnreadCount":   unreadCount,
		"Filter":        filter,
		"NextCursor":    result.NextCursor,
		"HasMore":       result.HasMore,
		"TriggerIdx":    result.TriggerIdx,
	}

	if isAppend {
		_ = h.renderer.RenderPartial(w, "notification_items.html", data)
		return
	}

	_ = h.renderer.RenderPartial(w, "notification_dropdown.html", data)
}

// GetBadge (GET /notifications/badge) — returns only the header bell badge HTML snippet
func (h *NotificationHandler) GetBadge(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		w.WriteHeader(http.StatusOK)
		return
	}

	count, err := h.notifService.GetUnreadCount(r.Context())
	if err != nil {
		count = 0
	}

	data := map[string]any{
		"UnreadCount": count,
	}

	_ = h.renderer.RenderPartial(w, "notification_badge.html", data)
}

// MarkAsRead (POST /notifications/mark-read) — marks notification as read via HTMX
func (h *NotificationHandler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	id := strings.TrimSpace(r.FormValue("id"))
	if id == "" {
		id = strings.TrimSpace(r.URL.Query().Get("id"))
	}

	filter := strings.TrimSpace(r.FormValue("filter"))
	if filter == "" {
		filter = strings.TrimSpace(r.URL.Query().Get("filter"))
	}
	if filter == "" {
		filter = "unread"
	}

	if id != "" {
		_ = h.notifService.MarkAsRead(r.Context(), id)
	}

	w.Header().Set("HX-Trigger", "notificationUpdated")

	r.URL.RawQuery = fmt.Sprintf("filter=%s", filter)
	h.GetList(w, r)
}
