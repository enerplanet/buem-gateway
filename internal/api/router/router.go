// Package router wires buem-gateway's HTTP routes to their handlers.
package router

import (
	"net/http"

	"github.com/enerplanet/buem-gateway/internal/api/handler"
	"github.com/enerplanet/buem-gateway/internal/api/middleware"
)

// New builds the HTTP handler for buem-gateway: GET /buem/health (unversioned
// -- liveness only, no request/response contract to break -- but nested
// under /buem/ so it doesn't collide with sibling services' own health
// checks once this sits behind the shared Orchestrator, matching weather's
// /v1/weather/health), POST /api/v1/buem/buildings (a flat list of
// buildings), POST /api/v1/buem/building (a single building, same shape as
// one buildings entry), and POST /api/v1/buem/validate (pre-flight-checks a
// /building request without calling BuEM). Each name matches its own
// request shape rather than a vague lifecycle verb — an earlier version
// called the multi-building endpoint /start, which said nothing about what
// it accepted or returned; a later one accepted a grid-topology graph
// (`{from, to}` edges), a shape that was never buem-gateway's own concept —
// it only ever needed a building's id and buem block — and pushed
// topology-graph parsing onto every caller, EnerPlanET being the only one.
// The /api/v1 prefix matches ignis's convention for the same reason: the
// request/response shape can change without breaking every caller at once.
func New(h *handler.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /buem/health", h.Health)
	mux.HandleFunc("POST /api/v1/buem/buildings", h.Buildings)
	mux.HandleFunc("POST /api/v1/buem/building", h.Building)
	mux.HandleFunc("POST /api/v1/buem/validate", h.Validate)
	return middleware.CORS(mux)
}
