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

	renderPage(w, h.renderer, "admin_users.html", data)
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

	ctxUser := middleware.GetUser(r)
	_, err := h.userService.CreateUser(r.Context(), ctxUser, username, password, confirmPassword, fullName, role, r)
	if err != nil {
		safeErr := html.EscapeString(err.Error())
		if isHTMX(r) {
			renderHTMXError(w, h.renderer, safeErr)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/users?error=%s", url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	if isHTMX(r) {
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

	renderPartial(w, h.renderer, "user_edit_modal.html", data)
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

	updatedUser, err := h.userService.UpdateUser(r.Context(), ctxUser, targetUsername, fullName, role, newPassword, confirmPassword, adminPassword, r)
	if err != nil {
		safeErr := html.EscapeString(err.Error())
		if isHTMX(r) {
			renderHTMXError(w, h.renderer, safeErr)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/users?error=%s", url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	// Update session fullname if editing self
	if ctxUser.Username == updatedUser.Username {
		ctxUser.FullName = updatedUser.FullName
	}

	if isHTMX(r) {
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

	err := h.userService.DeleteUser(r.Context(), targetUsername, ctxUser, adminPassword, r)
	if err != nil {
		safeErr := html.EscapeString(err.Error())
		if isHTMX(r) {
			renderHTMXError(w, h.renderer, safeErr)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/admin/users?error=%s", url.QueryEscape(err.Error())), http.StatusSeeOther)
		return
	}

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/admin/users?success=User+deleted+successfully")
		return
	}

	http.Redirect(w, r, "/admin/users?success=User+deleted+successfully", http.StatusSeeOther)
}
