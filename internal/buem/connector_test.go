package buem

import (
	"encoding/json"
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

func TestConnectorRun_EnrichesTopologyAndWritesCSV(t *testing.T) {
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

	topology := buildTestTopology(t, "building-1")
	enriched, err := conn.Run(topology, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	assertBuemBlockMerged(t, enriched)
	assertHeatingCSVWritten(t, dataDir)
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
	]}}}`)

	enriched, err := conn.RunSingle("solo-building", geometry, buemBlock, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if err != nil {
		t.Fatalf("RunSingle() error: %v", err)
	}

	var block map[string]interface{}
	if err := json.Unmarshal(enriched, &block); err != nil {
		t.Fatalf("unmarshal enriched block: %v", err)
	}
	if _, ok := block["thermal_load_profile"]; !ok {
		t.Fatalf("expected thermal_load_profile in enriched block, got %v", block)
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
	buemBlock := json.RawMessage(`{"building":{"envelope":{"elements":[{"id":"Wall_1"}]}}}`)

	_, err := conn.RunSingle("solo-building", geometry, buemBlock, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60)
	if err == nil {
		t.Fatal("expected RunSingle to return an error when BuEM rejects the request")
	}
}

func buildTestTopology(t *testing.T, nodeID string) json.RawMessage {
	t.Helper()
	edge := map[string]interface{}{
		"from": map[string]interface{}{
			"id":       nodeID,
			"geometry": map[string]interface{}{"type": "Point", "coordinates": []float64{12.5, 48.5}},
			"properties": map[string]interface{}{
				"feature_type": "BasePOI",
				"buem": map[string]interface{}{"building": map[string]interface{}{
					"building_type": "SFH", "country": "DE",
					// Envelope present so this test never exercises the TABULA
					// fallback path (internal/tabula) — that has its own tests.
					"envelope": map[string]interface{}{"elements": []interface{}{
						map[string]interface{}{"id": "Wall_1", "type": "wall", "area": 10.0, "azimuth": 0.0, "tilt": 90.0, "U": 1.5},
					}},
				}},
			},
		},
		"to": map[string]interface{}{
			"id":         "trafo-1",
			"geometry":   map[string]interface{}{"type": "Point", "coordinates": []float64{12.6, 48.6}},
			"properties": map[string]interface{}{"feature_type": "Transformer"},
		},
	}
	raw, err := json.Marshal([]interface{}{edge})
	if err != nil {
		t.Fatalf("build test topology: %v", err)
	}
	return raw
}

func assertBuemBlockMerged(t *testing.T, enriched json.RawMessage) {
	t.Helper()
	var edges []struct {
		From struct {
			Properties struct {
				BUEM map[string]interface{} `json:"buem"`
			} `json:"properties"`
		} `json:"from"`
	}
	if err := json.Unmarshal(enriched, &edges); err != nil {
		t.Fatalf("unmarshal enriched topology: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if _, ok := edges[0].From.Properties.BUEM["thermal_load_profile"]; !ok {
		t.Fatalf("expected thermal_load_profile in merged buem block, got %v", edges[0].From.Properties.BUEM)
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
