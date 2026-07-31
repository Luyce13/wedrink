package handlers

import (
	"fmt"
	"html"
	"net/http"
	"net/url"

	"wedrink/internal/middleware"
	"wedrink/internal/models"
	"wedrink/internal/render"
	"wedrink/internal/services"
)

type UserHandler struct {
	userService *services.UserService
	renderer    *render.Renderer
}

func NewUserHandler(userService *services.UserService, renderer *render.Renderer) *UserHandler {
	return &UserHandler{
		userService: userService,
		renderer:    renderer,
	}
}

// RenderUserList (GET /admin/users)
func (h *UserHandler) RenderUserList(w http.ResponseWriter, r *http.Request) {
	ctxUser := middleware.GetUser(r)
	users, err := h.userService.GetAllUsers(r.Context())
	if err != nil {
		users = []models.User{}
	}

	data := map[string]interface{}{
		"Title":     "User Management — Wedrink EOD Portal",
		"ActiveTab": "users",
		"User":      ctxUser,
		"Users":     users,
		"Success":   r.URL.Query().Get("success"),
		"Error":     r.URL.Query().Get("error"),
	}

	_ = h.renderer.RenderPage(w, "admin_users.html", data)
}

// HandleCreateUser (POST /admin/users/create)
func (h *UserHandler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirmPassword")
	fullName := r.FormValue("fullName")
	role := r.FormValue("role")

	_, err := h.userService.CreateUser(r.Context(), username, password, confirmPassword, fullName, role)
	if err != nil {
		safeErr := html.EscapeString(err.Error())
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, `<div class="alert-banner p-3 mb-4 text-xs font-semibold text-rose-300 bg-rose-950/90 border border-rose-800 rounded flex items-center justify-between"><span>⚠ %s</span><button type="button" onclick="dismissAlert(this.parentElement)" class="text-rose-400 hover:text-white px-1">✕</button></div>`, safeErr)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/users?error=%s", url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/users?success=User+created+successfully")
		return
	}

	http.Redirect(w, r, "/admin/users?success=User+created+successfully", http.StatusSeeOther)
}

// RenderEditUserModal (GET /admin/users/edit)
func (h *UserHandler) RenderEditUserModal(w http.ResponseWriter, r *http.Request) {
	ctxUser := middleware.GetUser(r)
	targetUsername := r.URL.Query().Get("username")
	user, err := h.userService.GetUserByUsername(r.Context(), targetUsername)
	if err != nil || user == nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	data := map[string]interface{}{
		"CurrentUser": ctxUser,
		"TargetUser":  user,
	}

	_ = h.renderer.RenderPartial(w, "user_edit_modal.html", data)
}

// HandleEditUser (POST /admin/users/edit)
func (h *UserHandler) HandleEditUser(w http.ResponseWriter, r *http.Request) {
	ctxUser := middleware.GetUser(r)
	if ctxUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	targetUsername := r.FormValue("username")
	fullName := r.FormValue("fullName")
	role := r.FormValue("role")
	newPassword := r.FormValue("newPassword")
	confirmPassword := r.FormValue("confirmPassword")
	adminPassword := r.FormValue("adminPassword")

	updatedUser, err := h.userService.UpdateUser(r.Context(), ctxUser.Username, targetUsername, fullName, role, newPassword, confirmPassword, adminPassword)
	if err != nil {
		safeErr := html.EscapeString(err.Error())
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, `<div class="alert-banner p-3 mb-4 text-xs font-semibold text-rose-300 bg-rose-950/90 border border-rose-800 rounded flex items-center justify-between"><span>⚠ %s</span><button type="button" onclick="dismissAlert(this.parentElement)" class="text-rose-400 hover:text-white px-1">✕</button></div>`, safeErr)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/users?error=%s", url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	// Update session fullname if editing self
	if ctxUser.Username == updatedUser.Username {
		ctxUser.FullName = updatedUser.FullName
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/users?success=User+updated+successfully")
		return
	}

	http.Redirect(w, r, "/admin/users?success=User+updated+successfully", http.StatusSeeOther)
}

// HandleDeleteUser (POST /admin/users/delete)
func (h *UserHandler) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	ctxUser := middleware.GetUser(r)
	if ctxUser == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	targetUsername := r.FormValue("username")
	adminPassword := r.FormValue("adminPassword")

	err := h.userService.DeleteUser(r.Context(), targetUsername, ctxUser.Username, adminPassword)
	if err != nil {
		safeErr := html.EscapeString(err.Error())
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			_, _ = fmt.Fprintf(w, `<div class="alert-banner p-3 mb-4 text-xs font-semibold text-rose-300 bg-rose-950/90 border border-rose-800 rounded flex items-center justify-between"><span>⚠ %s</span><button type="button" onclick="dismissAlert(this.parentElement)" class="text-rose-400 hover:text-white px-1">✕</button></div>`, safeErr)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/users?error=%s", url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/admin/users?success=User+deleted+successfully")
		return
	}

	http.Redirect(w, r, "/admin/users?success=User+deleted+successfully", http.StatusSeeOther)
}
