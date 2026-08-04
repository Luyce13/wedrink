package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"wedrink/internal/middleware"
	"wedrink/internal/render"
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

// GetUnread (GET /notifications/unread) — returns the notification dropdown partial
func (h *NotificationHandler) GetUnread(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUser(r)
	if user == nil || !user.CanEditReports() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	notifs, err := h.notifService.GetUnreadNotifications(r.Context())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch notifications: %v", err), http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Notifications": notifs,
		"UnreadCount":   len(notifs),
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

	if id != "" {
		_ = h.notifService.MarkAsRead(r.Context(), id)
	}

	// Set HTMX trigger header to refresh notification count dynamically
	w.Header().Set("HX-Trigger", "notificationUpdated")

	// Return updated unread dropdown
	h.GetUnread(w, r)
}
