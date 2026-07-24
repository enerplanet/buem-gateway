// Package handler implements buem-gateway's HTTP handlers.
package handler

import (
	"encoding/json"
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

// StartRequest is the topology-JSON body accepted by POST /buem/start. It is
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

// Start handles POST /buem/start: it fans the request topology's buildings
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

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Health handles GET /health.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "service": "buem-gateway"})
}
