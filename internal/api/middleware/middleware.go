// Package middleware provides HTTP middleware for buem-gateway's API server.
package middleware

import (
	"log"
	"net/http"
	"os"
	"strings"
)

// CORS sets CORS headers for browser clients, mirroring ignis's identical
// middleware (github.com/thd-spatial-ai/ignis, internal/api/middleware).
//
// This is the only CORS implementation in the stack. The optional TLS proxy
// (environment/https/caddy/Caddyfile) forwards every request, preflight
// included, and sets no CORS headers of its own, and the HTTP environment runs
// with no proxy at all. Without this middleware a real request succeeds
// server-side and is then rejected by the browser as a failed fetch.
//
// Allowed origins come from the ALLOWED_ORIGINS environment variable as a
// comma-separated list (e.g. "http://localhost:5173,https://app.example.com").
// If unset, all cross-origin requests are rejected.
func CORS(next http.Handler) http.Handler {
	allowed := parseAllowedOrigins()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && isAllowed(origin, allowed) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseAllowedOrigins() []string {
	var allowed []string
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		if t := strings.TrimSpace(o); t != "" {
			allowed = append(allowed, t)
		}
	}
	if len(allowed) == 0 {
		log.Println("warning: ALLOWED_ORIGINS is not set — all cross-origin requests will be rejected")
	}
	return allowed
}

func isAllowed(origin string, allowed []string) bool {
	for _, o := range allowed {
		if o == origin {
			return true
		}
	}
	return false
}
