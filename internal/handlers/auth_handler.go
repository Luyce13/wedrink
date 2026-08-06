package handlers

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
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
	renderPage(w, h.renderer, "login.html", data)
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
		safeErr := html.EscapeString(err.Error())
		if isHTMX(r) {
			renderHTMXError(w, h.renderer, safeErr)
			return
		}

		data := map[string]any{
			"Title": "Login - Wedrink EOD System",
			"Error": err.Error(),
		}
		renderPage(w, h.renderer, "login.html", data)
		return
	}

	// Set session cookie (username|role|fullname)
	rawCookie := fmt.Sprintf("%s|%s|%s", user.Username, string(user.Role), user.FullName)
	cookieValue := url.QueryEscape(rawCookie)
	http.SetCookie(w, &http.Cookie{
		Name:     "wedrink_session",
		Value:    cookieValue,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	if isHTMX(r) {
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

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/login")
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
