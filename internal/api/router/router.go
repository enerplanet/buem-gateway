// Package router wires buem-gateway's HTTP routes to their handlers.
package router

import (
	"net/http"

	"github.com/enerplanet/buem-gateway/internal/api/handler"
)

// New builds the HTTP handler for buem-gateway: GET /health,
// POST /buem/start (topology, multi-building), and POST /buem/building
// (single building, no topology wrapper).
func New(h *handler.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /buem/start", h.Start)
	mux.HandleFunc("POST /buem/building", h.Building)
	return mux
}
