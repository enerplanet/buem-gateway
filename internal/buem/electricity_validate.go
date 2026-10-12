package buem

import (
	"encoding/json"
	"fmt"
	"time"
)

// requireElectricityProfile checks buem.inputs.electricity_load_profile, when
// present, against schemas/v6-draft $defs/inputs: exactly one of values or
// path, and inline values non-empty, non-negative, in kWh or kW, one per hour
// of year. A path is checked by buem-model, which owns the file. Every
// failure wraps ErrInvalidRequest.
func requireElectricityProfile(buemRaw json.RawMessage, year int) error {
	var buem struct {
		Inputs struct {
			Profile json.RawMessage `json:"electricity_load_profile"`
		} `json:"inputs"`
	}
	if err := json.Unmarshal(buemRaw, &buem); err != nil || len(buem.Inputs.Profile) == 0 || string(buem.Inputs.Profile) == "null" {
		return nil
	}
	const field = "buem.inputs.electricity_load_profile"
	var profile struct {
		Values json.RawMessage `json:"values"`
		Path   *string         `json:"path"`
		Unit   string          `json:"unit"`
	}
	if err := json.Unmarshal(buem.Inputs.Profile, &profile); err != nil {
		return fmt.Errorf("%s must be an object with values or path: %w", field, ErrInvalidRequest)
	}
	hasValues := len(profile.Values) > 0 && string(profile.Values) != "null"
	if hasValues == (profile.Path != nil) {
		return fmt.Errorf("%s needs exactly one of values or path: %w", field, ErrInvalidRequest)
	}
	if !hasValues {
		return nil
	}
	if profile.Unit != "" && profile.Unit != "kWh" && profile.Unit != "kW" {
		return fmt.Errorf("%s.unit must be kWh or kW with values, got %q: %w", field, profile.Unit, ErrInvalidRequest)
	}
	// Pointers, because a JSON null decodes into a plain float64 as 0 without error.
	var values []*float64
	if err := json.Unmarshal(profile.Values, &values); err != nil {
		return fmt.Errorf("%s.values must be an array of numbers: %w", field, ErrInvalidRequest)
	}
	for i, v := range values {
		if v == nil || *v < 0 {
			return fmt.Errorf("%s.values[%d] must be a number of at least 0: %w", field, i, ErrInvalidRequest)
		}
	}
	hours := int(time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Sub(time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)).Hours())
	if len(values) != hours {
		return fmt.Errorf("%s.values has %d values, the start_date year %d has %d hours: %w", field, len(values), year, hours, ErrInvalidRequest)
	}
	return nil
}
