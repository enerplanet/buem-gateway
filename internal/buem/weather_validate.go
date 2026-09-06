package buem

import (
	"encoding/json"
	"fmt"
	"sort"
)

// ErrMissingWeather is returned by RunSingle when buem.weather is absent or
// has no usable columns. Callers (e.g. the HTTP handler) can check for it, or
// the broader ErrInvalidRequest it wraps, with errors.Is to distinguish "you
// sent an incomplete request" (400) from "BuEM tried to run it and failed"
// (422) — this check runs before BuEM is ever called, so the latter status
// would be misleading.
var ErrMissingWeather = fmt.Errorf(`buem.weather is required with "index" and at least one of T/GHI/DNI/DHI under "variables" — buem-gateway does not resolve weather from any external service, the caller must supply a pre-resolved timeseries (see enerplanet/buem#10): %w`, ErrInvalidRequest)

// requireWeather reports ErrMissingWeather if the buem block's weather is
// missing or has no usable columns, and a wrapped ErrInvalidRequest if any
// variable array's length does not match index. Mirrors requireEnvelope:
// buem-gateway resolves nothing from any external service, including weather
// serve — the upstream BuEM Flask service itself now rejects a request with
// no weather (enerplanet/buem#10), but a check here surfaces it as a clear
// client-input-error 400 instead of a confusing 422 two hops away.
//
// This is the hand-written half of the schemas/v5/request_schema.json
// $defs/weather contract (required index + anyOf T/GHI/DNI/DHI; every
// variable array the same length as index). Keep the two in step;
// TestValidatorsMatchV5Example fails if they diverge.
//
// Shape matches weather serve's GET /v1/weather/point?format=json response
// exactly: {"index": [...], "variables": {"T": [...], "GHI": [...], ...}}.
//
// Returns nil if buemRaw doesn't even parse as an object with a weather
// key — that's a different, pre-existing failure mode (malformed JSON),
// left to normal request parsing to report.
func requireWeather(buemRaw json.RawMessage) error {
	var buem struct {
		Weather *struct {
			Index     []json.RawMessage          `json:"index"`
			Variables map[string]json.RawMessage `json:"variables"`
		} `json:"weather"`
	}
	if err := json.Unmarshal(buemRaw, &buem); err != nil {
		return nil
	}
	w := buem.Weather
	if w == nil || len(w.Index) == 0 || !hasUsableWeatherVariable(w.Variables) {
		return ErrMissingWeather
	}
	names := make([]string, 0, len(w.Variables))
	for name := range w.Variables {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		var values []json.RawMessage
		if err := json.Unmarshal(w.Variables[name], &values); err != nil {
			return fmt.Errorf("buem.weather.variables.%s is not an array: %w", name, ErrInvalidRequest)
		}
		if len(values) != len(w.Index) {
			return fmt.Errorf("buem.weather.variables.%s has %d values, index has %d: %w", name, len(values), len(w.Index), ErrInvalidRequest)
		}
	}
	return nil
}

// hasUsableWeatherVariable reports whether variables contains at least one
// of the columns BuEM actually reads (T/GHI/DNI/DHI) — matching
// geojson_processor.py::_weather_from_payload's own check, so a caller
// that only supplied e.g. wind variables is rejected here the same way
// BuEM itself would reject it, not with a misleading 200.
func hasUsableWeatherVariable(variables map[string]json.RawMessage) bool {
	for _, name := range [...]string{"T", "GHI", "DNI", "DHI"} {
		if _, ok := variables[name]; ok {
			return true
		}
	}
	return false
}
