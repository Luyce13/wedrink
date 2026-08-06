package utils

import (
	"net"
	"net/http"
	"strings"
)

// GetClientIP extracts the real client IP address from the HTTP request.
// It prioritizes Cloudflare's CF-Connecting-IP header, followed by X-Forwarded-For,
// and falls back to r.RemoteAddr.
func GetClientIP(r *http.Request) string {
	if r == nil {
		return "127.0.0.1"
	}

	// 1. Cloudflare Tunnel primary header
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		return cfIP
	}

	// 2. Standard X-Forwarded-For header (comma-separated list, first IP is client)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	// 3. Fallback to direct TCP connection RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If RemoteAddr doesn't contain a port, return as is
		if r.RemoteAddr != "" {
			return strings.TrimSpace(r.RemoteAddr)
		}
		return "127.0.0.1"
	}

	return strings.TrimSpace(host)
}
