package buem

import "encoding/json"

// ValidateSingle reports whether a single-building buem block has an envelope
// and weather with usable data, without ever calling BuEM. It backs
// POST /api/v1/buem/validate.
//
// It is narrower than the run path: TaskFromBuilding also requires a Point
// geometry with a [lon, lat] pair and a parseable start_date, so a body that
// passes here can still fail the run with a 400.
func ValidateSingle(buemRaw json.RawMessage) error {
	if err := requireEnvelope(buemRaw); err != nil {
		return err
	}
	if err := requireWeather(buemRaw); err != nil {
		return err
	}
	return nil
}
