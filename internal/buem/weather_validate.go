package buem

import (
	"encoding/json"
	"errors"
)

// ErrMissingWeather is returned by RunSingle when buem.weather is absent or
// has no usable columns. Callers (e.g. the HTTP handler) can check for it
// with errors.Is to distinguish "you sent an incomplete request" (400) from
// "BuEM tried to run it and failed" (422) — this check runs before BuEM is
// ever called, so the latter status would be misleading.
var ErrMissingWeather = errors.New(`buem.weather is required with "index" and at least one of T/GHI/DNI/DHI under "variables" — buem-gateway does not resolve weather from any external service, the caller must supply a pre-resolved timeseries (see enerplanet/buem#10)`)

// requireWeather reports ErrMissingWeather if the buem block's weather is
// missing or has no usable columns. Mirrors requireEnvelope: buem-gateway
// resolves nothing from any external service, including weather serve —
// the upstream BuEM Flask service itself now rejects a request with no
// weather (enerplanet/buem#10), but a check here surfaces it as a clear
// client-input-error 400 instead of a confusing 422 two hops away.
//
// This is the hand-written half of the schemas/v5/request_schema.json
// $defs/weather contract (required index + anyOf T/GHI/DNI/DHI). Keep the
// two in step; TestValidatorsMatchV5Example fails if they diverge.
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
	if buem.Weather == nil || len(buem.Weather.Index) == 0 || !hasUsableWeatherVariable(buem.Weather.Variables) {
		return ErrMissingWeather
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
