package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/buem"
	"github.com/enerplanet/buem-gateway/internal/config"
)

// fakeUpstream returns a stub BuEM /api/process server, just enough to
// exercise the handler → connector → CSV-write path end to end. Mirrors
// internal/buem/connector_test.go's helper of the same name.
func fakeUpstream(t *testing.T, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if statusCode != http.StatusOK {
			http.Error(w, "upstream rejected the request", statusCode)
			return
		}

		var req struct {
			Features []json.RawMessage `json:"features"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("upstream: decode request: %v", err)
		}
		var feature struct {
			ID string `json:"id"`
		}
		json.Unmarshal(req.Features[0], &feature)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"type":     "FeatureCollection",
			"metadata": map[string]int{"total_features": 1, "successful_features": 1},
			"features": []map[string]interface{}{{
				"type": "Feature",
				"id":   feature.ID,
				"properties": map[string]interface{}{
					"buem": map[string]interface{}{
						"thermal_load_profile": map[string]interface{}{
							"start_time": "2018-01-01T00:00:00Z",
							"end_time":   "2018-12-31T23:00:00Z",
							"resolution": "60",
							"summary": map[string]interface{}{
								"heating": map[string]interface{}{
									"total": map[string]interface{}{"value": 1000, "unit": "kWh"},
								},
							},
							"timeseries": map[string]interface{}{
								"unit":    "kW",
								"heating": []float64{0.1, 0.2},
							},
						},
						"model_metadata": map[string]interface{}{
							"processing_time": map[string]interface{}{"value": 1.2, "unit": "s"},
						},
					},
				},
			}},
		})
	}))
}

func newTestHandler(t *testing.T, upstream *httptest.Server) *Handler {
	t.Helper()
	host, portStr, ok := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	if !ok {
		t.Fatalf("unexpected upstream URL %q", upstream.URL)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse upstream port: %v", err)
	}

	dataDir := t.TempDir()
	cfg := &config.Config{
		MaxConcurrentSims: 4,
		BuEM:              config.UpstreamService{Host: host, Port: port},
		BuemDataDir:       dataDir,
		BuemResultsDir:    dataDir,
	}
	return New(buem.NewConnector(cfg))
}

func decodeBody(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, w.Body.String())
	}
	return body
}

func TestHealth(t *testing.T) {
	h := New(nil)
	req := httptest.NewRequest(http.MethodGet, "/buem/health", nil)
	w := httptest.NewRecorder()
	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	body := decodeBody(t, w)
	if body["status"] != "ok" {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestBuilding_MissingBuemBlock(t *testing.T) {
	h := New(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/building", strings.NewReader(`{"id":"b1"}`))
	w := httptest.NewRecorder()
	h.Building(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBuilding_BadJSON(t *testing.T) {
	h := New(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/building", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	h.Building(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestBuilding_MissingEnvelope confirms a request with no building.envelope
// is rejected with 400 (an incomplete request), not 422 (a run that reached
// BuEM and failed) — buem-gateway resolves envelope from nowhere else.
func TestBuilding_MissingEnvelope(t *testing.T) {
	h := New(nil)
	reqBody := `{
		"geometry": {"type":"Point","coordinates":[12.5,48.5]},
		"buem": {"building":{"building_type":"SFH","country":"DE"}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/building", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Building(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "envelope") {
		t.Errorf("body = %q, want it to mention envelope", w.Body.String())
	}
}

// TestBuilding_MissingWeather mirrors TestBuilding_MissingEnvelope: a
// request with envelope but no buem.weather is rejected with 400, not 422
// — buem-gateway resolves weather from nowhere else either.
func TestBuilding_MissingWeather(t *testing.T) {
	h := New(nil)
	reqBody := `{
		"geometry": {"type":"Point","coordinates":[12.5,48.5]},
		"buem": {"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
			{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
		]}}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/building", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Building(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "weather") {
		t.Errorf("body = %q, want it to mention weather", w.Body.String())
	}
}

func TestValidate_MissingBuemBlock(t *testing.T) {
	h := New(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/validate", strings.NewReader(`{"id":"b1"}`))
	w := httptest.NewRecorder()
	h.Validate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestValidate_BadJSON(t *testing.T) {
	h := New(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/validate", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	h.Validate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestValidate_MissingEnvelope(t *testing.T) {
	h := New(nil)
	reqBody := `{"buem": {"building":{"building_type":"SFH","country":"DE"}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/validate", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Validate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "envelope") {
		t.Errorf("body = %q, want it to mention envelope", w.Body.String())
	}
}

func TestValidate_MissingWeather(t *testing.T) {
	h := New(nil)
	reqBody := `{
		"buem": {"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
			{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
		]}}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/validate", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Validate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusBadRequest, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "weather") {
		t.Errorf("body = %q, want it to mention weather", w.Body.String())
	}
}

// TestValidate_ValidRequestNeverCallsBuEM confirms /validate reports a
// well-formed request without ever reaching the upstream BuEM service --
// h is built with New(nil), so any attempt to call a connector method
// would nil-panic and fail the test.
func TestValidate_ValidRequestNeverCallsBuEM(t *testing.T) {
	h := New(nil)
	reqBody := `{
		"geometry": {"type":"Point","coordinates":[12.5,48.5]},
		"buem": {"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
			{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
		]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/validate", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Validate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["valid"] != true {
		t.Errorf("body = %v, want valid=true", body)
	}
}

func TestBuilding_DefaultsIDWhenOmitted(t *testing.T) {
	upstream := fakeUpstream(t, http.StatusOK)
	defer upstream.Close()
	h := newTestHandler(t, upstream)

	reqBody := `{
		"geometry": {"type":"Point","coordinates":[12.5,48.5]},
		"start_date": "2018-01-01T00:00:00Z",
		"end_date": "2018-12-31T23:00:00Z",
		"resolution": 60,
		"model_id": "demo",
		"buem": {"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
			{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
		]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/building", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Building(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["id"] != "building" {
		t.Errorf("id = %v, want default %q", body["id"], "building")
	}
	buemBlock, ok := body["buem"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected buem object in response, got %v", body["buem"])
	}
	if _, ok := buemBlock["thermal_load_profile"]; !ok {
		t.Errorf("expected thermal_load_profile in buem block, got %v", buemBlock)
	}
}

func TestBuilding_UsesProvidedID(t *testing.T) {
	upstream := fakeUpstream(t, http.StatusOK)
	defer upstream.Close()
	h := newTestHandler(t, upstream)

	reqBody := `{
		"id": "solo-building",
		"geometry": {"type":"Point","coordinates":[12.5,48.5]},
		"start_date": "2018-01-01T00:00:00Z",
		"end_date": "2018-12-31T23:00:00Z",
		"resolution": 60,
		"model_id": "demo",
		"buem": {"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
			{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
		]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/building", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Building(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
	body := decodeBody(t, w)
	if body["id"] != "solo-building" {
		t.Errorf("id = %v, want %q", body["id"], "solo-building")
	}
}

func TestBuilding_ConnectorErrorReturnsUnprocessableEntity(t *testing.T) {
	upstream := fakeUpstream(t, http.StatusBadRequest)
	defer upstream.Close()
	h := newTestHandler(t, upstream)

	reqBody := `{
		"id": "solo-building",
		"geometry": {"type":"Point","coordinates":[12.5,48.5]},
		"start_date": "2018-01-01T00:00:00Z",
		"end_date": "2018-12-31T23:00:00Z",
		"resolution": 60,
		"model_id": "demo",
		"buem": {"building":{"envelope":{"elements":[{"id":"Wall_1"}]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/building", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Building(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusUnprocessableEntity, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "upstream rejected the request") {
		t.Errorf("expected the upstream's own error message to surface, got %q", w.Body.String())
	}
}

func TestBuildings_BadJSON(t *testing.T) {
	h := New(nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/buildings", strings.NewReader(`not json`))
	w := httptest.NewRecorder()
	h.Buildings(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBuildings_EmptyListReturnsEmptyResults(t *testing.T) {
	h := New(nil)
	reqBody := `{"start_date":"2018-01-01T00:00:00Z","model_id":"demo","buildings":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/buildings", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Buildings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	var results []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, w.Body.String())
	}
	if len(results) != 0 {
		t.Errorf("expected an empty results array, got %v", results)
	}
}

func TestBuildings_RunsEachBuildingIndependently(t *testing.T) {
	upstream := fakeUpstream(t, http.StatusOK)
	defer upstream.Close()
	h := newTestHandler(t, upstream)

	reqBody := `{
		"start_date": "2018-01-01T00:00:00Z",
		"end_date": "2018-12-31T23:00:00Z",
		"resolution": 60,
		"model_id": "demo",
		"weather": {"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}},
		"buildings": [
			{
				"id": "building-1",
				"geometry": {"type":"Point","coordinates":[12.5,48.5]},
				"building": {"building_type":"SFH","country":"DE","envelope":{"elements":[
					{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
				]}}
			},
			{
				"id": "building-2",
				"geometry": {"type":"Point","coordinates":[12.6,48.6]},
				"building": {"building_type":"SFH","country":"DE"}
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/buildings", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Buildings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
	var results []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, w.Body.String())
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %v", results)
	}

	if results[0]["id"] != "building-1" {
		t.Fatalf("expected results[0].id = building-1, got %v", results[0])
	}
	buemBlock, ok := results[0]["buem"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected building-1 to be enriched with a buem block, got %v", results[0])
	}
	if _, present := buemBlock["thermal_load_profile"]; !present {
		t.Fatalf("expected building-1 to carry thermal_load_profile, got %v", buemBlock)
	}

	if results[1]["id"] != "building-2" {
		t.Fatalf("expected results[1].id = building-2, got %v", results[1])
	}
	if _, present := results[1]["buem"]; present {
		t.Fatalf("expected building-2 (no envelope) to have no buem block, got %v", results[1])
	}
	errMsg, ok := results[1]["error"].(string)
	if !ok || !strings.Contains(errMsg, "envelope") {
		t.Fatalf("expected building-2's error to mention envelope, got %v", results[1])
	}
}

// TestBuildings_WeatherIsSharedAcrossBuildings confirms the top-level
// weather field (sent once) reaches every building's run, not just one —
// the whole point of hoisting it out of the per-building block.
func TestBuildings_WeatherIsSharedAcrossBuildings(t *testing.T) {
	upstream := fakeUpstream(t, http.StatusOK)
	defer upstream.Close()
	h := newTestHandler(t, upstream)

	reqBody := `{
		"start_date": "2018-01-01T00:00:00Z",
		"end_date": "2018-12-31T23:00:00Z",
		"resolution": 60,
		"model_id": "demo",
		"weather": {"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}},
		"buildings": [
			{
				"id": "building-1",
				"geometry": {"type":"Point","coordinates":[12.5,48.5]},
				"building": {"envelope":{"elements":[{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}]}}
			},
			{
				"id": "building-2",
				"geometry": {"type":"Point","coordinates":[12.6,48.6]},
				"building": {"envelope":{"elements":[{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}]}}
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/buildings", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Buildings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
	var results []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, w.Body.String())
	}
	for _, result := range results {
		if _, present := result["error"]; present {
			t.Fatalf("expected every building to resolve using the shared weather, got %v", result)
		}
	}
}

// TestBuildings_MissingTopLevelWeatherFailsEveryBuilding confirms every
// building needs the shared weather field — omitting it isn't "no weather
// for one building", it's "no weather for the whole batch".
func TestBuildings_MissingTopLevelWeatherFailsEveryBuilding(t *testing.T) {
	h := New(nil)
	reqBody := `{
		"start_date": "2018-01-01T00:00:00Z",
		"model_id": "demo",
		"buildings": [
			{
				"id": "building-1",
				"geometry": {"type":"Point","coordinates":[12.5,48.5]},
				"building": {"envelope":{"elements":[{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}]}}
			}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/buem/buildings", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	h.Buildings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (a batch-level problem is still a 200 with per-building errors, body=%s)", w.Code, http.StatusOK, w.Body.String())
	}
	var results []map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode response body: %v (body=%s)", err, w.Body.String())
	}
	errMsg, ok := results[0]["error"].(string)
	if !ok || !strings.Contains(errMsg, "weather") {
		t.Fatalf("expected building-1's error to mention weather, got %v", results[0])
	}
}
