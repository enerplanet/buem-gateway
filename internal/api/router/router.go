// Package router wires buem-gateway's HTTP routes to their handlers.
package router

import (
	"net/http"

	"github.com/enerplanet/buem-gateway/internal/api/handler"
	"github.com/enerplanet/buem-gateway/internal/api/middleware"
)

// New builds the HTTP handler for buem-gateway: GET /health (unversioned,
// liveness only — no request/response contract to break), POST
// /api/v1/buem/start (topology, multi-building), and POST
// /api/v1/buem/building (single building, no topology wrapper). The
// /api/v1 prefix matches ignis's convention for the same reason: the
// request/response shape can change without breaking every caller at once.
func New(h *handler.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("POST /api/v1/buem/start", h.Start)
	mux.HandleFunc("POST /api/v1/buem/building", h.Building)
	return middleware.CORS(mux)
}
