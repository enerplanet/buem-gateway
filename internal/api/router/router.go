// Package router wires buem-gateway's HTTP routes to their handlers.
package router

import (
	"net/http"

	"github.com/enerplanet/buem-gateway/internal/api/handler"
)

// New builds the HTTP handler for buem-gateway: GET /health and
// POST /buem/start.
func New(h *handler.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /buem/start", h.Start)
	return mux
}
