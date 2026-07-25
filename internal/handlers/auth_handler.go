package handlers

import (
	"fmt"
	"net/http"
	"time"

	"wedrink/internal/middleware"
	"wedrink/internal/render"
	"wedrink/internal/services"
)

type AuthHandler struct {
	authService *services.AuthService
	renderer    *render.Renderer
}

func NewAuthHandler(authService *services.AuthService, renderer *render.Renderer) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		renderer:    renderer,
	}
}

func (h *AuthHandler) RenderLogin(w http.ResponseWriter, r *http.Request) {
	if user := middleware.GetUser(r); user != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	data := map[string]any{
		"Title": "Login - Wedrink EOD System",
		"Error": "",
	}
	_ = h.renderer.RenderPage(w, "login.html", data)
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form submission", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.authService.Authenticate(r.Context(), username, password)
	if err != nil {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Reswap", "outerHTML")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(fmt.Sprintf(`<div id="login-error" class="p-3 bg-red-950/80 border border-red-500/50 text-red-300 text-sm rounded-lg flex items-center gap-2 mb-4"><svg class="w-5 h-5 text-red-400 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>%s</div>`, err.Error())))
			return
		}

		data := map[string]any{
			"Title": "Login - Wedrink EOD System",
			"Error": err.Error(),
		}
		_ = h.renderer.RenderPage(w, "login.html", data)
		return
	}

	// Set session cookie (username|role|fullname)
	cookieValue := fmt.Sprintf("%s|%s|%s", user.Username, string(user.Role), user.FullName)
	http.SetCookie(w, &http.Cookie{
		Name:     "wedrink_session",
		Value:    cookieValue,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "wedrink_session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
	})

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
