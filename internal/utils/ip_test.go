package utils_test

import (
	"net/http/httptest"
	"testing"

	"wedrink/internal/utils"
)

func TestGetClientIP(t *testing.T) {
	t.Run("CF-Connecting-IP takes precedence", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("CF-Connecting-IP", "203.0.113.195")
		req.Header.Set("X-Forwarded-For", "198.51.100.1")
		req.RemoteAddr = "192.0.2.1:12345"

		ip := utils.GetClientIP(req)
		if ip != "203.0.113.195" {
			t.Errorf("expected 203.0.113.195, got %s", ip)
		}
	})

	t.Run("X-Forwarded-For fallback when no CF header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.1, 70.41.3.18")
		req.RemoteAddr = "192.0.2.1:12345"

		ip := utils.GetClientIP(req)
		if ip != "198.51.100.1" {
			t.Errorf("expected 198.51.100.1, got %s", ip)
		}
	})

	t.Run("RemoteAddr fallback when no headers present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.0.2.55:54321"

		ip := utils.GetClientIP(req)
		if ip != "192.0.2.55" {
			t.Errorf("expected 192.0.2.55, got %s", ip)
		}
	})
}
