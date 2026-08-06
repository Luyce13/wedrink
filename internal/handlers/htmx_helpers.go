// Package handlers provides shared utilities for HTTP handler methods.
package handlers

import (
	"log/slog"
	"net/http"

	"wedrink/internal/render"
)

// isHTMX returns true if the request originated from an HTMX client.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// renderPage renders a full page template, returning a 500 error page on failure.
// Replaces the pattern: _ = h.renderer.RenderPage(w, ...)
func renderPage(w http.ResponseWriter, renderer *render.Renderer, page string, data any) {
	if err := renderer.RenderPage(w, page, data); err != nil {
		slog.Error("Failed to render page", "page", page, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// renderPartial renders an HTMX partial fragment, returning a 500 error on failure.
// Replaces the pattern: _ = h.renderer.RenderPartial(w, ...)
func renderPartial(w http.ResponseWriter, renderer *render.Renderer, partial string, data any) {
	if err := renderer.RenderPartial(w, partial, data); err != nil {
		slog.Error("Failed to render partial", "partial", partial, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// renderHTMXError renders a consistent HTMX error fragment using the alert_error.html template.
// This replaces all inline HTML error strings scattered across handlers.
func renderHTMXError(w http.ResponseWriter, renderer *render.Renderer, msg string) {
	w.Header().Set("Content-Type", "text/html")
	data := map[string]any{"Error": msg}
	renderPartial(w, renderer, "alert_error.html", data)
}
