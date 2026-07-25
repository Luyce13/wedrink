package middleware

import (
	"context"
	"net/http"
	"strings"

	"wedrink/internal/models"
)

type contextKey string

const UserContextKey contextKey = "user"

type SessionManager struct {
	secretKey string
}

func NewSessionManager(secretKey string) *SessionManager {
	return &SessionManager{secretKey: secretKey}
}

func (sm *SessionManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("wedrink_session")
		if err == nil && cookie.Value != "" {
			// Format: username:role:fullname
			parts := strings.Split(cookie.Value, "|")
			if len(parts) >= 3 {
				user := &models.User{
					Username: parts[0],
					Role:     models.Role(parts[1]),
					FullName: parts[2],
				}
				ctx := context.WithValue(r.Context(), UserContextKey, user)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

func GetUser(r *http.Request) *models.User {
	if user, ok := r.Context().Value(UserContextKey).(*models.User); ok {
		return user
	}
	return nil
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r)
		if user == nil {
			if r.Header.Get("HX-Request") == "true" {
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func RequireRole(roles ...models.Role) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			user := GetUser(r)
			if user == nil {
				if r.Header.Get("HX-Request") == "true" {
					w.Header().Set("HX-Redirect", "/login")
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			allowed := false
			for _, role := range roles {
				if user.Role == role {
					allowed = true
					break
				}
			}

			if !allowed {
				if r.Header.Get("HX-Request") == "true" {
					w.Header().Set("HX-Reswap", "outerHTML")
					http.Error(w, `<div class="p-4 bg-red-900/50 border border-red-500/50 text-red-200 rounded-lg">Forbidden: You do not have permission to perform this action (Manager role required).</div>`, http.StatusForbidden)
					return
				}
				http.Error(w, "Forbidden: You do not have permission to access this resource.", http.StatusForbidden)
				return
			}

			next(w, r)
		}
	}
}
