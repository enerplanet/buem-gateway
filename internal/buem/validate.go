package buem

import "encoding/json"

// ValidateSingle reports whether a single-building buem block is complete
// enough for RunSingle to attempt — envelope and weather present with
// usable data — without ever calling BuEM. RunSingle calls this itself
// before its own upstream request, so this function and the checks
// RunSingle actually enforces can never drift apart; the HTTP layer's
// POST /api/v1/buem/validate calls it directly to pre-flight-check a
// request without running BuEM at all.
func ValidateSingle(buemRaw json.RawMessage) error {
	if err := requireEnvelope(buemRaw); err != nil {
		return err
	}
	if err := requireWeather(buemRaw); err != nil {
		return err
	}
	return nil
}
