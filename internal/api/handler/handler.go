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

// buildingsRequest is the body accepted by POST /api/v1/buem/buildings —
// several buildings, no topology/edge-list wrapper. start_date/end_date/
// resolution/model_id/weather are shared across the whole batch rather than
// repeated per building — weather in particular is normally the same
// timeseries for every building in one model run (one point resolved for
// the model's whole area), so repeating it per building would mean sending
// the same hourly-for-a-year arrays once per building for no reason.
type buildingsRequest struct {
	StartDate  string             `json:"start_date"`
	EndDate    string             `json:"end_date"`
	Resolution int                `json:"resolution"`
	ModelID    string             `json:"model_id"`
	Weather    json.RawMessage    `json:"weather"`
	Buildings  []buildingListItem `json:"buildings"`
}

// buildingListItem is one building's own data — geometry and its building
// block (envelope, building_type, country, ...). No weather here; see
// buildingsRequest.Weather.
type buildingListItem struct {
	ID       string          `json:"id"`
	Geometry json.RawMessage `json:"geometry"`
	Building json.RawMessage `json:"building"`
}

type buildingResultItem struct {
	ID    string          `json:"id"`
	BUEM  json.RawMessage `json:"buem,omitempty"`
	Error string          `json:"error,omitempty"`
}

// Buildings handles POST /api/v1/buem/buildings: runs BuEM for each building
// in the request concurrently and returns one result per building, in the
// same order as the request. A building with an incomplete buem block, or
// one BuEM itself rejects, gets its own error entry — it never affects any
// other building's result. The caller owns whatever grid/topology concept
// ties these buildings together, if any; buem-gateway only sees the flat
// list. req.Weather is re-attached to each building here, reconstructing
// the same per-building {building, weather} shape /api/v1/buem/building
// takes, so every check and code path past this point (envelope/weather
// validation, task building) is shared with the single-building endpoint.
func (h *Handler) Buildings(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}
	var req buildingsRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "can't parse request body", http.StatusBadRequest)
		return
	}

	inputs := make([]buem.BuildingInput, len(req.Buildings))
	for i, b := range req.Buildings {
		buemBlock, err := json.Marshal(map[string]json.RawMessage{"building": b.Building, "weather": req.Weather})
		if err != nil {
			http.Error(w, "can't build request for building "+b.ID+": "+err.Error(), http.StatusBadRequest)
			return
		}
		inputs[i] = buem.BuildingInput{ID: b.ID, Geometry: b.Geometry, BUEM: buemBlock}
	}

	results := h.connector.RunBatch(inputs, req.StartDate, req.EndDate, req.ModelID, req.Resolution)

	items := make([]buildingResultItem, len(results))
	for i, r := range results {
		items[i] = buildingResultItem{ID: r.ID, BUEM: r.BUEM, Error: r.Error}
	}
	writeJSON(w, items)
}

// buildingRequest is the body accepted by POST /api/v1/buem/building — one
// building, no topology/edge-list wrapper. See buildingsRequest for the
// multi-building shape.
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
// Unlike Topology, a failed run is reported as an HTTP error, not echoed back
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
	if errors.Is(err, buem.ErrMissingEnvelope) || errors.Is(err, buem.ErrMissingWeather) {
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

// Validate handles POST /api/v1/buem/validate: checks that a single-building
// request (same body shape as /api/v1/buem/building) has a complete
// envelope and weather block, without ever calling BuEM. Lets a caller
// (e.g. the Orchestrator) confirm a request is well-formed before paying
// for the real run.
func (h *Handler) Validate(w http.ResponseWriter, r *http.Request) {
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

	if err := buem.ValidateSingle(req.BUEM); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, map[string]interface{}{"valid": true})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Health handles GET /buem/health. contract_version is the BUEM-EnerPlanET
// API contract this instance enforces (schemas/<contract_version>/), separate
// from buem-gateway's own software version.
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "ok", "contract_version": buem.APIVersion})
}
