package buem

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/config"
)

// fakeUpstream returns a stub BuEM /api/process server. Its response carries
// just enough of a real BuEM response — one heating value — to exercise CSV
// writing and the merge-back path.
func fakeUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req FeatureCollection
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("upstream: decode request: %v", err)
		}
		var feature struct {
			ID string `json:"id"`
		}
		json.Unmarshal(req.Features[0], &feature)

		resp := ResponseFeatureCollection{
			Type:     "FeatureCollection",
			Metadata: CollectionMetadata{TotalFeatures: 1, SuccessfulFeatures: 1},
			Features: []ResponseFeature{{
				Type: "Feature",
				ID:   feature.ID,
				Properties: ResponseProperties{BUEM: ResponseBlock{
					ThermalLoadProfile: ThermalLoadProfile{
						StartTime:  "2018-01-01T00:00:00Z",
						EndTime:    "2018-12-31T23:00:00Z",
						Resolution: "60",
						Summary: ThermalSummary{
							Heating: LoadStats{Total: Quantity{Value: 1000, Unit: "kWh"}},
						},
						Timeseries: &Timeseries{
							Unit:    "kW",
							Heating: []float64{0.114, 0.223},
						},
					},
					ModelMetadata: ModelMetadata{ProcessingTime: Quantity{Value: 1.2, Unit: "s"}},
				}},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestConnectorRunBatch_EnrichesBuildingsAndWritesCSV(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()

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
	conn := NewConnector(cfg)

	inputs := []BuildingInput{testBuildingInput("building-1")}
	results := conn.RunBatch(inputs, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	assertBuemBlockPresent(t, results[0])
	assertHeatingCSVWritten(t, dataDir)
}

// TestConnectorRunBatch_PartialFailureDoesNotAffectOtherBuildings confirms
// RunBatch's core property: one building with no envelope gets its own
// error entry, and every other building in the same request still runs and
// resolves normally — no request-wide failure from one bad building.
func TestConnectorRunBatch_PartialFailureDoesNotAffectOtherBuildings(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()

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
	conn := NewConnector(cfg)

	broken := testBuildingInput("building-broken")
	broken.BUEM = json.RawMessage(`{"building":{"building_type":"SFH","country":"DE"}}`) // no envelope

	inputs := []BuildingInput{testBuildingInput("building-good"), broken}
	results := conn.RunBatch(inputs, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != "building-good" || results[0].Error != "" || results[0].BUEM == nil {
		t.Errorf("expected building-good to resolve cleanly, got %+v", results[0])
	}
	if results[1].ID != "building-broken" || results[1].Error == "" || results[1].BUEM != nil {
		t.Errorf("expected building-broken to carry its own error, got %+v", results[1])
	}
}

func TestConnectorRunSingle_EnrichesOneBuildingNoTopology(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()

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
	conn := NewConnector(cfg)

	geometry := json.RawMessage(`{"type":"Point","coordinates":[12.5,48.5]}`)
	buemBlock := json.RawMessage(`{"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
		{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
	]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}`)

	enriched, err := conn.RunSingle("solo-building", geometry, buemBlock, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if err != nil {
		t.Fatalf("RunSingle() error: %v", err)
	}

	var block map[string]interface{}
	if err := json.Unmarshal(enriched, &block); err != nil {
		t.Fatalf("unmarshal enriched block: %v", err)
	}
	tlp, ok := block["thermal_load_profile"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected thermal_load_profile in enriched block, got %v", block)
	}
	// RunSingle callers (e.g. a browser client) have no access to the shared
	// volume CSVs land on — the timeseries must survive in the response.
	ts, ok := tlp["timeseries"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected timeseries to be present in RunSingle's response, got %v", tlp)
	}
	if _, ok := ts["heating"]; !ok {
		t.Fatalf("expected timeseries.heating to be present, got %v", ts)
	}
	assertHeatingCSVWritten(t, dataDir)
}

func TestConnectorRunSingle_ReturnsErrorOnFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream rejected the request", http.StatusBadRequest)
	}))
	defer upstream.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	cfg := &config.Config{MaxConcurrentSims: 4, BuEM: config.UpstreamService{Host: host, Port: port}}
	conn := NewConnector(cfg)

	geometry := json.RawMessage(`{"type":"Point","coordinates":[12.5,48.5]}`)
	buemBlock := json.RawMessage(`{"building":{"envelope":{"elements":[{"id":"Wall_1"}]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}`)

	_, err := conn.RunSingle("solo-building", geometry, buemBlock, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if err == nil {
		t.Fatal("expected RunSingle to return an error when BuEM rejects the request")
	}
	if errors.Is(err, ErrMissingEnvelope) || errors.Is(err, ErrMissingWeather) {
		t.Fatalf("expected the upstream's rejection to propagate, got a pre-flight validation error instead: %v", err)
	}
}

// TestConnectorRunSingle_RejectsMissingEnvelopeWithoutCallingBuEM confirms
// buem-gateway never resolves a missing envelope itself (no ignis, no other
// external service) and never forwards the incomplete request to BuEM
// either — the upstream server in this test would fail the test if called
// at all.
func TestConnectorRunSingle_RejectsMissingEnvelopeWithoutCallingBuEM(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("BuEM must not be called when envelope is missing")
	}))
	defer upstream.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	cfg := &config.Config{MaxConcurrentSims: 4, BuEM: config.UpstreamService{Host: host, Port: port}}
	conn := NewConnector(cfg)

	geometry := json.RawMessage(`{"type":"Point","coordinates":[12.5,48.5]}`)
	buemBlock := json.RawMessage(`{"building":{"building_type":"SFH","country":"DE"}}`)

	_, err := conn.RunSingle("solo-building", geometry, buemBlock, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if !errors.Is(err, ErrMissingEnvelope) {
		t.Fatalf("RunSingle() error = %v, want ErrMissingEnvelope", err)
	}
}

// TestConnectorRunSingle_RejectsMissingWeatherWithoutCallingBuEM mirrors
// TestConnectorRunSingle_RejectsMissingEnvelopeWithoutCallingBuEM: buem-gateway
// never resolves weather itself (no weather serve call, no fallback) and
// never forwards the incomplete request to BuEM either.
func TestConnectorRunSingle_RejectsMissingWeatherWithoutCallingBuEM(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("BuEM must not be called when weather is missing")
	}))
	defer upstream.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	cfg := &config.Config{MaxConcurrentSims: 4, BuEM: config.UpstreamService{Host: host, Port: port}}
	conn := NewConnector(cfg)

	geometry := json.RawMessage(`{"type":"Point","coordinates":[12.5,48.5]}`)
	buemBlock := json.RawMessage(`{"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
		{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
	]}}}`)

	_, err := conn.RunSingle("solo-building", geometry, buemBlock, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if !errors.Is(err, ErrMissingWeather) {
		t.Fatalf("RunSingle() error = %v, want ErrMissingWeather", err)
	}
}

// TestConnectorRunSingle_RejectsWeatherWithOnlyUnusableVariables confirms a
// weather block with variables BuEM never reads (e.g. wind, not solar) is
// treated the same as no weather at all -- matching
// geojson_processor.py::_weather_from_payload's own column check.
func TestConnectorRunSingle_RejectsWeatherWithOnlyUnusableVariables(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("BuEM must not be called when weather has no usable columns")
	}))
	defer upstream.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	cfg := &config.Config{MaxConcurrentSims: 4, BuEM: config.UpstreamService{Host: host, Port: port}}
	conn := NewConnector(cfg)

	geometry := json.RawMessage(`{"type":"Point","coordinates":[12.5,48.5]}`)
	buemBlock := json.RawMessage(`{"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[
		{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
	]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"WS_10M":[3.0]}}}`)

	_, err := conn.RunSingle("solo-building", geometry, buemBlock, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if !errors.Is(err, ErrMissingWeather) {
		t.Fatalf("RunSingle() error = %v, want ErrMissingWeather", err)
	}
}

// testWeatherBlock returns a minimal but valid buem.weather block —
// shape matches weather serve's GET /v1/weather/point?format=json
// response, required by BuEM since enerplanet/buem#10.
func testWeatherBlock() map[string]interface{} {
	return map[string]interface{}{
		"index":     []string{"2018-01-01T00:30:00Z"},
		"variables": map[string]interface{}{"T": []float64{1.0}},
	}
}

// testBuildingInput returns a complete BuildingInput (valid envelope and
// weather) for id, ready to run.
func testBuildingInput(id string) BuildingInput {
	buemBlock, _ := json.Marshal(map[string]interface{}{
		"building": map[string]interface{}{
			"building_type": "SFH", "country": "DE",
			"envelope": map[string]interface{}{"elements": []interface{}{
				map[string]interface{}{"id": "Wall_1", "type": "wall", "area": 10.0, "azimuth": 0.0, "tilt": 90.0, "U": 1.5},
			}},
		},
		"weather": testWeatherBlock(),
	})
	geometry, _ := json.Marshal(map[string]interface{}{"type": "Point", "coordinates": []float64{12.5, 48.5}})
	return BuildingInput{ID: id, Geometry: geometry, BUEM: buemBlock}
}

func assertBuemBlockPresent(t *testing.T, result BuildingResult) {
	t.Helper()
	if result.Error != "" {
		t.Fatalf("expected no error, got %q", result.Error)
	}
	var block map[string]interface{}
	if err := json.Unmarshal(result.BUEM, &block); err != nil {
		t.Fatalf("unmarshal result buem block: %v", err)
	}
	tlp, ok := block["thermal_load_profile"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected thermal_load_profile in result buem block, got %v", block)
	}
	// RunBatch callers read results from the shared volume CSVs — the inline
	// timeseries should be stripped, unlike RunSingle's response.
	if _, present := tlp["timeseries"]; present {
		t.Fatalf("expected timeseries to be stripped from RunBatch's response, got %v", tlp["timeseries"])
	}
}

func assertHeatingCSVWritten(t *testing.T, dataDir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dataDir, "demo-model", "heating_*.csv"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("expected exactly one heating CSV under %s/demo-model, got %v (err=%v)", dataDir, matches, err)
	}
	content, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read heating CSV: %v", err)
	}
	if !strings.HasPrefix(string(content), "demand\n0.114\n0.223\n") {
		t.Fatalf("unexpected heating CSV content: %q", content)
	}
}
