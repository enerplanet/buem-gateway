package buem

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/config"
)

// fakeUpstream returns a stub BuEM /api/process server. Its response carries
// just enough of a real BuEM response, one heating series, to exercise the
// merge-back path.
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

// TestConnectorRunBatch_PassesThroughHotWaterAndKitchen confirms BuEM's
// hot_water/kitchen summary stats (v6-draft) survive the round trip through
// the connector's typed structs rather than being silently dropped, the way
// they were before ThermalSummary had fields for them.
func TestConnectorRunBatch_PassesThroughHotWaterAndKitchen(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req FeatureCollection
		json.NewDecoder(r.Body).Decode(&req)
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
							Heating:  LoadStats{Total: Quantity{Value: 1000, Unit: "kWh"}},
							HotWater: &LoadStats{Total: Quantity{Value: 1840.2, Unit: "kWh"}},
							Kitchen:  &LoadStats{Total: Quantity{Value: 386.1, Unit: "kWh_gas"}},
						},
						Timeseries: &Timeseries{
							Unit:        "kW",
							Heating:     []float64{0.114, 0.223},
							HotWater:    []float64{0.31, 0.29},
							Kitchen:     []float64{0.0, 1.1},
							KitchenUnit: "kW_gas",
						},
					},
					ModelMetadata: ModelMetadata{ProcessingTime: Quantity{Value: 1.2, Unit: "s"}},
				}},
			}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer upstream.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	cfg := &config.Config{
		MaxConcurrentSims: 4,
		BuEM:              config.UpstreamService{Host: host, Port: port},
	}
	conn := NewConnector(cfg)

	results := conn.RunBatch([]BuildingInput{testBuildingInput("building-1")}, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60, false)

	if len(results) != 1 || results[0].Error != "" {
		t.Fatalf("expected 1 clean result, got %+v", results)
	}

	var block struct {
		ThermalLoadProfile struct {
			Summary struct {
				HotWater struct{ Total Quantity } `json:"hot_water"`
				Kitchen  struct{ Total Quantity } `json:"kitchen"`
			} `json:"summary"`
		} `json:"thermal_load_profile"`
	}
	if err := json.Unmarshal(results[0].BUEM, &block); err != nil {
		t.Fatalf("unmarshal result buem block: %v", err)
	}
	summary := block.ThermalLoadProfile.Summary
	if summary.HotWater.Total != (Quantity{Value: 1840.2, Unit: "kWh"}) {
		t.Errorf("expected summary.hot_water.total {1840.2 kWh}, got %+v", summary.HotWater.Total)
	}
	if summary.Kitchen.Total != (Quantity{Value: 386.1, Unit: "kWh_gas"}) {
		t.Errorf("expected summary.kitchen.total {386.1 kWh_gas}, got %+v", summary.Kitchen.Total)
	}

	assertNoFilePaths(t, results[0].BUEM)
}

func TestConnectorRunBatch_ReturnsSummaryAndWritesNoFiles(t *testing.T) {
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

	cfg := &config.Config{
		MaxConcurrentSims: 4,
		BuEM:              config.UpstreamService{Host: host, Port: port},
	}
	conn := NewConnector(cfg)

	inputs := []BuildingInput{testBuildingInput("building-1")}
	results := conn.RunBatch(inputs, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60, false)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	assertBuemBlockPresent(t, results[0])
	assertNoFilePaths(t, results[0].BUEM)
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

	cfg := &config.Config{
		MaxConcurrentSims: 4,
		BuEM:              config.UpstreamService{Host: host, Port: port},
	}
	conn := NewConnector(cfg)

	broken := testBuildingInput("building-broken")
	broken.BUEM = json.RawMessage(`{"building":{"building_type":"SFH","country":"DE"}}`) // no envelope

	inputs := []BuildingInput{testBuildingInput("building-good"), broken}
	results := conn.RunBatch(inputs, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60, false)

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

	cfg := &config.Config{
		MaxConcurrentSims: 4,
		BuEM:              config.UpstreamService{Host: host, Port: port},
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
	// RunSingle always returns the series inline.
	ts, ok := tlp["timeseries"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected timeseries to be present in RunSingle's response, got %v", tlp)
	}
	if _, ok := ts["heating"]; !ok {
		t.Fatalf("expected timeseries.heating to be present, got %v", ts)
	}
	assertNoFilePaths(t, enriched)
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
	buemBlock := json.RawMessage(`{"building":{"envelope":{"elements":[
		{"id":"Wall_1","type":"wall","area":10.0,"azimuth":0.0,"tilt":90.0,"U":1.5}
	]}},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}`)

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

// TestTaskFromBuilding_RejectsGeometryWithoutPointType covers the geometry.type
// guard. BuEM's schema fixes geometry.type to "Point"; a geometry with
// coordinates but no type, or a non-Point type, is rejected before BuEM is
// called, with an error naming the field rather than BuEM's generic
// "Invalid GeoJSON payload".
func TestTaskFromBuilding_RejectsGeometryWithoutPointType(t *testing.T) {
	for _, tc := range []struct {
		name string
		geom string
	}{
		{"no type", `{"coordinates":[12.5,48.5]}`},
		{"wrong type", `{"type":"Polygon","coordinates":[12.5,48.5]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := testBuildingInput("b1")
			in.Geometry = json.RawMessage(tc.geom)
			_, err := TaskFromBuilding(in, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60, "m")
			if err == nil || !strings.Contains(err.Error(), "geometry.type") {
				t.Fatalf("TaskFromBuilding() error = %v, want one naming geometry.type", err)
			}
		})
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
	// RunBatch strips the inline timeseries unless keepTimeseries is set,
	// unlike RunSingle's response.
	if _, present := tlp["timeseries"]; present {
		t.Fatalf("expected timeseries to be stripped from RunBatch's response, got %v", tlp["timeseries"])
	}
}

// assertNoFilePaths checks that the response carries no file paths: load
// profiles are returned in the response only.
func assertNoFilePaths(t *testing.T, block json.RawMessage) {
	t.Helper()
	for _, field := range []string{"heating_file", "cooling_file", "electricity_file", "hot_water_file", "kitchen_file"} {
		if strings.Contains(string(block), `"`+field+`"`) {
			t.Errorf("response carries %s, want no file paths: %s", field, block)
		}
	}
}

// TestTaskFromBuilding_ForwardsBuildingLevelWindowFields confirms the
// building block reaches BuEM verbatim: the v6-draft building-level window
// and door fields are neither validated nor stripped on the way through.
func TestTaskFromBuilding_ForwardsBuildingLevelWindowFields(t *testing.T) {
	in := testBuildingInput("b1")
	var block map[string]interface{}
	if err := json.Unmarshal(in.BUEM, &block); err != nil {
		t.Fatalf("unmarshal test buem block: %v", err)
	}
	want := map[string]interface{}{
		"window_to_wall_ratio": 0.25,
		"window_U":             map[string]interface{}{"value": 1.8, "unit": "W/(m2K)"},
		"window_g_gl":          0.5,
		"door_U":               map[string]interface{}{"value": 2.0, "unit": "W/(m2K)"},
	}
	for k, v := range want {
		block["building"].(map[string]interface{})[k] = v
	}
	in.BUEM, _ = json.Marshal(block)

	task, err := TaskFromBuilding(in, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60, "m")
	if err != nil {
		t.Fatalf("TaskFromBuilding() error = %v", err)
	}
	var feature struct {
		Properties struct {
			BUEM struct {
				Building map[string]interface{} `json:"building"`
			} `json:"buem"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(task.RawFeature, &feature); err != nil {
		t.Fatalf("unmarshal forwarded feature: %v", err)
	}
	for k, v := range want {
		if got := feature.Properties.BUEM.Building[k]; !reflect.DeepEqual(got, v) {
			t.Errorf("forwarded building.%s = %v, want %v", k, got, v)
		}
	}
}

// TestConnectorRunBatch_KeepTimeseriesReturnsInlineSeries covers the opt-in:
// the hourly values come back in the response instead of only the summary.
// The default stays false; TestConnectorRunBatch_ReturnsSummaryAndWritesNoFiles
// pins that side.
func TestConnectorRunBatch_KeepTimeseriesReturnsInlineSeries(t *testing.T) {
	upstream := fakeUpstream(t)
	defer upstream.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse upstream port: %v", err)
	}
	cfg := &config.Config{
		MaxConcurrentSims: 4,
		BuEM:              config.UpstreamService{Host: host, Port: port},
	}
	conn := NewConnector(cfg)

	results := conn.RunBatch([]BuildingInput{testBuildingInput("building-1")}, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "demo-model", 60, true)

	if len(results) != 1 || results[0].Error != "" {
		t.Fatalf("expected 1 clean result, got %+v", results)
	}
	var block struct {
		ThermalLoadProfile struct {
			Timeseries *struct {
				Unit    string    `json:"unit"`
				Heating []float64 `json:"heating"`
			} `json:"timeseries"`
		} `json:"thermal_load_profile"`
	}
	if err := json.Unmarshal(results[0].BUEM, &block); err != nil {
		t.Fatalf("unmarshal result buem block: %v", err)
	}
	ts := block.ThermalLoadProfile.Timeseries
	if ts == nil {
		t.Fatalf("expected timeseries in the response with keepTimeseries=true, got none")
	}
	if !reflect.DeepEqual(ts.Heating, []float64{0.114, 0.223}) {
		t.Errorf("timeseries.heating = %v, want [0.114 0.223]", ts.Heating)
	}
	assertNoFilePaths(t, results[0].BUEM)
}

// TestConnectorRunBatch_AsksForTimeseriesOnlyWhenKept confirms buem-gateway
// requests BuEM's hourly series only when the caller keeps it. Without
// include_timeseries BuEM still returns the full summary and writes no
// intermediate file.
func TestConnectorRunBatch_AsksForTimeseriesOnlyWhenKept(t *testing.T) {
	for _, tc := range []struct {
		keep      bool
		wantQuery string
	}{{false, ""}, {true, "include_timeseries=true"}} {
		var gotQuery string
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotQuery = r.URL.RawQuery
			block := ResponseBlock{ThermalLoadProfile: ThermalLoadProfile{
				Summary: ThermalSummary{Heating: LoadStats{Total: Quantity{Value: 1000, Unit: "kWh"}}},
			}}
			if r.URL.Query().Get("include_timeseries") == "true" {
				block.ThermalLoadProfile.Timeseries = &Timeseries{Unit: "kW", Heating: []float64{0.1}}
			}
			json.NewEncoder(w).Encode(ResponseFeatureCollection{
				Type:     "FeatureCollection",
				Features: []ResponseFeature{{Properties: ResponseProperties{BUEM: block}}},
			})
		}))
		host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
		port, _ := strconv.Atoi(portStr)
		conn := NewConnector(&config.Config{MaxConcurrentSims: 1, BuEM: config.UpstreamService{Host: host, Port: port}})

		results := conn.RunBatch([]BuildingInput{testBuildingInput("b1")}, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "", 60, tc.keep)
		upstream.Close()

		if gotQuery != tc.wantQuery {
			t.Errorf("keep=%v: BuEM query = %q, want %q", tc.keep, gotQuery, tc.wantQuery)
		}
		if len(results) != 1 || results[0].Error != "" {
			t.Errorf("keep=%v: results = %+v, want one clean result", tc.keep, results)
		}
	}
}

// TestConnectorRunBatch_DuplicateIDsKeepTheirOwnResults confirms results are
// matched to buildings by position, not by id: buildings sharing an id each
// get their own result, and one building's preflight failure does not mark
// the others with the same id as failed.
func TestConnectorRunBatch_DuplicateIDsKeepTheirOwnResults(t *testing.T) {
	// The stub reports the request's longitude as the heating total, so each
	// result shows which building it came from.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req FeatureCollection
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var feature struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
		}
		json.Unmarshal(req.Features[0], &feature)
		json.NewEncoder(w).Encode(ResponseFeatureCollection{
			Type: "FeatureCollection",
			Features: []ResponseFeature{{Properties: ResponseProperties{BUEM: ResponseBlock{
				ThermalLoadProfile: ThermalLoadProfile{
					Summary:    ThermalSummary{Heating: LoadStats{Total: Quantity{Value: feature.Geometry.Coordinates[0], Unit: "kWh"}}},
					Timeseries: &Timeseries{Unit: "kW", Heating: []float64{1}},
				},
			}}}},
		})
	}))
	defer upstream.Close()

	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	conn := NewConnector(&config.Config{MaxConcurrentSims: 4, BuEM: config.UpstreamService{Host: host, Port: port}})

	atLon := func(lon float64) BuildingInput {
		in := testBuildingInput("dup")
		in.Geometry, _ = json.Marshal(map[string]interface{}{"type": "Point", "coordinates": []float64{lon, 48.5}})
		return in
	}
	noEnvelope := BuildingInput{ID: "dup", Geometry: atLon(15).Geometry, BUEM: json.RawMessage(`{"building":{},"weather":{"index":["2018-01-01T00:30:00Z"],"variables":{"T":[1.0]}}}`)}

	results := conn.RunBatch([]BuildingInput{atLon(10), noEnvelope, atLon(20)}, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "", 60, false)

	if len(results) != 3 {
		t.Fatalf("got %d results, want 3", len(results))
	}
	heatingTotal := func(r BuildingResult) float64 {
		var block struct {
			ThermalLoadProfile struct {
				Summary struct {
					Heating struct{ Total Quantity } `json:"heating"`
				} `json:"summary"`
			} `json:"thermal_load_profile"`
		}
		if err := json.Unmarshal(r.BUEM, &block); err != nil {
			t.Fatalf("unmarshal result: %v (result=%+v)", err, r)
		}
		return block.ThermalLoadProfile.Summary.Heating.Total.Value
	}
	for _, want := range []struct {
		i   int
		lon float64
	}{{0, 10}, {2, 20}} {
		r := results[want.i]
		if r.Error != "" {
			t.Errorf("results[%d].Error = %q, want a result", want.i, r.Error)
			continue
		}
		if got := heatingTotal(r); got != want.lon {
			t.Errorf("results[%d] heating total = %v, want %v (another building's result)", want.i, got, want.lon)
		}
	}
	if results[1].Error == "" {
		t.Errorf("results[1].Error is empty, want the missing-envelope error")
	}
}
