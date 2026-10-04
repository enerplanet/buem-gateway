package buem

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/config"
)

// occupancyOnlyInput is a building that selects no thermal output and so
// carries neither an envelope nor weather.
func occupancyOnlyInput(id string) BuildingInput {
	return BuildingInput{
		ID:       id,
		Geometry: json.RawMessage(`{"type":"Point","coordinates":[6.02,52.10]}`),
		BUEM: json.RawMessage(`{"building":{"building_type":"MFH","country":"NL","A_ref":{"value":900,"unit":"m2"},"residential_units":12},
			"outputs":{"heating":"none","cooling":"none","electricity":"summary","hot_water":"summary","kitchen":"none"}}`),
	}
}

func TestTaskFromBuilding_OccupancyOnlyNeedsNoEnvelopeOrWeather(t *testing.T) {
	if _, err := TaskFromBuilding(occupancyOnlyInput("b1"), "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60, ""); err != nil {
		t.Fatalf("TaskFromBuilding() = %v, want an occupancy-only request accepted", err)
	}
}

func TestTaskFromBuilding_ThermalOutputStillNeedsEnvelopeAndWeather(t *testing.T) {
	for _, outputs := range []string{
		`{"heating":"summary","cooling":"none"}`,
		`{"cooling":"none"}`, // omitted heating counts as selected
		`{"heating":"none","cooling":"series"}`,
	} {
		in := occupancyOnlyInput("b1")
		in.BUEM = json.RawMessage(`{"building":{"building_type":"MFH","country":"NL","A_ref":{"value":900,"unit":"m2"},"residential_units":12},"outputs":` + outputs + `}`)
		_, err := TaskFromBuilding(in, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60, "")
		if !errors.Is(err, ErrMissingEnvelope) {
			t.Errorf("outputs %s: TaskFromBuilding() = %v, want ErrMissingEnvelope", outputs, err)
		}
	}
}

func TestTaskFromBuilding_RejectsUnknownOutputLevel(t *testing.T) {
	in := occupancyOnlyInput("b1")
	in.BUEM = json.RawMessage(`{"building":{},"outputs":{"heating":"none","cooling":"none","electricity":"hourly"}}`)
	_, err := TaskFromBuilding(in, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60, "")
	if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "outputs.electricity") {
		t.Fatalf("TaskFromBuilding() = %v, want an ErrInvalidRequest naming outputs.electricity", err)
	}
}

func TestTaskFromBuilding_RejectsUnknownOutputProfile(t *testing.T) {
	in := occupancyOnlyInput("b1")
	in.BUEM = json.RawMessage(`{"building":{},"outputs":{"heatin":"none","cooling":"none"}}`)
	_, err := TaskFromBuilding(in, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60, "")
	if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "outputs.heatin") {
		t.Fatalf("TaskFromBuilding() = %v, want an ErrInvalidRequest naming outputs.heatin", err)
	}
}

// TestConnectorRunBatch_OutputsResponsePassesThrough covers the response side:
// with outputs set, BuEM's block reaches the caller as BuEM sent it, without
// include_timeseries on the request, without zero entries for unselected
// loads, and with fields buem-gateway has no struct for.
func TestConnectorRunBatch_OutputsResponsePassesThrough(t *testing.T) {
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"type":"FeatureCollection","features":[{"type":"Feature","id":"b1","properties":{"buem":{
			"thermal_load_profile":{"summary":{"electricity":{"total":{"value":21000,"unit":"kWh"}},"hot_water":{"total":{"value":30000,"unit":"kWh"}}}},
			"model_metadata":{"processing_time":{"value":0.1,"unit":"s"},"resolved_inputs":{"building_type":"MFH","residential_units":12,"num_persons":1.54}}}}}]}`))
	}))
	defer upstream.Close()
	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	conn := NewConnector(&config.Config{MaxConcurrentSims: 1, BuEM: config.UpstreamService{Host: host, Port: port}})

	results := conn.RunBatch([]BuildingInput{occupancyOnlyInput("b1")}, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "", 60, false)

	if len(results) != 1 || results[0].Error != "" {
		t.Fatalf("results = %+v, want one clean result", results)
	}
	if gotQuery != "" {
		t.Errorf("BuEM query = %q, want none when outputs is set", gotQuery)
	}
	body := string(results[0].BUEM)
	if strings.Contains(body, `"heating"`) || strings.Contains(body, `"cooling"`) {
		t.Errorf("response carries an unselected load: %s", body)
	}
	if !strings.Contains(body, `"resolved_inputs"`) || !strings.Contains(body, `"hot_water"`) {
		t.Errorf("response lost a field BuEM sent: %s", body)
	}
}

// TestConnectorRunBatch_UntypedFieldsSurviveWithoutOutputs covers the path
// without outputs: the hourly series is stripped, and every other field BuEM
// sends reaches the caller, including ones buem-gateway has no type for.
func TestConnectorRunBatch_UntypedFieldsSurviveWithoutOutputs(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"type":"FeatureCollection","features":[{"properties":{"buem":{
			"thermal_load_profile":{"summary":{"heating":{"total":{"value":1000,"unit":"kWh"}},"energy_intensity":{"value":10,"unit":"kWh/m2"}},"timeseries":{"heating":[0.1]}},
			"model_metadata":{"processing_time":{"value":1.2,"unit":"s"},"validation_warnings":["w1"],"resolved_inputs":{"num_persons":2.55}}}}}]}`))
	}))
	defer upstream.Close()
	host, portStr, _ := strings.Cut(strings.TrimPrefix(upstream.URL, "http://"), ":")
	port, _ := strconv.Atoi(portStr)
	conn := NewConnector(&config.Config{MaxConcurrentSims: 1, BuEM: config.UpstreamService{Host: host, Port: port}})

	results := conn.RunBatch([]BuildingInput{testBuildingInput("b1")}, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", "", 60, false)

	if len(results) != 1 || results[0].Error != "" {
		t.Fatalf("results = %+v, want one clean result", results)
	}
	body := string(results[0].BUEM)
	if strings.Contains(body, `"timeseries"`) {
		t.Errorf("series not stripped without keep_timeseries: %s", body)
	}
	for _, field := range []string{`"validation_warnings"`, `"resolved_inputs"`, `"energy_intensity"`} {
		if !strings.Contains(body, field) {
			t.Errorf("response lost %s: %s", field, body)
		}
	}
}
