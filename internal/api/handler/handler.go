// Package handler implements buem-gateway's HTTP handlers.
package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/enerplanet/buem-gateway/internal/buem"
)

// Handler holds the dependencies shared by all HTTP handlers.
type Handler struct {
	connector *buem.Connector
}

// New creates a Handler bound to the given Connector.
func New(connector *buem.Connector) *Handler {
	return &Handler{connector: connector}
}

// StartRequest is the topology-JSON body accepted by POST /api/v1/buem/start. It is
// decoded twice on purpose: once into rawFields to read only the handful of
// top-level scalars this handler needs, and the topology itself is kept as
// raw JSON so buem.Connector.Run can parse and re-merge it without this
// handler needing to understand its shape.
type startRequest struct {
	StartDate  string          `json:"start_date"`
	EndDate    string          `json:"end_date"`
	Resolution int             `json:"resolution"`
	ModelID    string          `json:"model_id"`
	Topology   json.RawMessage `json:"topology"`
}

// Start handles POST /api/v1/buem/start: it fans the request topology's buildings
// out to BuEM, writes their load profile CSVs, and returns the topology with
// each building's buem block enriched with the results. Buildings with no
// buem block are returned unchanged.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}

	var rawConfig map[string]json.RawMessage
	if err := json.Unmarshal(body, &rawConfig); err != nil {
		http.Error(w, "can't parse request body", http.StatusBadRequest)
		return
	}
	var req startRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "can't parse request body", http.StatusBadRequest)
		return
	}

	if req.Topology == nil {
		writeJSON(w, rawConfig)
		return
	}

	enriched, err := h.connector.Run(req.Topology, req.StartDate, req.EndDate, req.ModelID, req.Resolution)
	if err != nil {
		http.Error(w, "buem run failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rawConfig["topology"] = enriched
	writeJSON(w, rawConfig)
}

// buildingRequest is the body accepted by POST /api/v1/buem/building — one
// building, no topology/edge-list wrapper. See startRequest for the
// grid-scale multi-building shape.
type buildingRequest struct {
	ID         string          `json:"id"`
	Geometry   json.RawMessage `json:"geometry"`
	StartDate  string          `json:"start_date"`
	EndDate    string          `json:"end_date"`
	Resolution int             `json:"resolution"`
	ModelID    string          `json:"model_id"`
	BUEM       json.RawMessage `json:"buem"`
}

// Building handles POST /api/v1/buem/building: runs BuEM for exactly one building
// and returns its enriched buem block (thermal_load_profile + model_metadata).
// Unlike Start, a failed run is reported as an HTTP error, not echoed back
// unchanged — with only one building there's no partial-success case to
// preserve caller data for.
func (h *Handler) Building(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}
	var req buildingRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "can't parse request body", http.StatusBadRequest)
		return
	}
	if req.BUEM == nil {
		http.Error(w, "buem block is required", http.StatusBadRequest)
		return
	}

	id := req.ID
	if id == "" {
		id = "building"
	}

	enriched, err := h.connector.RunSingle(id, req.Geometry, req.BUEM, req.StartDate, req.EndDate, req.ModelID, req.Resolution)
	if errors.Is(err, buem.ErrMissingEnvelope) {
		// Rejected before ever calling BuEM — an incomplete request, not a
		// run that was attempted and failed. 400, not 422.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "buem run failed: "+err.Error(), http.StatusUnprocessableEntity)
		return
	}

	writeJSON(w, map[string]interface{}{"id": id, "buem": json.RawMessage(enriched)})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "service": "buem-gateway"})
}
