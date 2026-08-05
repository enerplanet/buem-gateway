package buem

import (
	"encoding/json"
	"errors"
)

// ErrMissingEnvelope is returned by RunSingle when building.envelope is
// absent or empty. Callers (e.g. the HTTP handler) can check for it with
// errors.Is to distinguish "you sent an incomplete request" (400) from
// "BuEM tried to run it and failed" (422) — this check runs before BuEM is
// ever called, so the latter status would be misleading.
var ErrMissingEnvelope = errors.New("building.envelope is required with at least one element — buem-gateway does not resolve missing geometry from any external service, the caller must supply a complete envelope")

// requireEnvelope reports ErrMissingEnvelope if the buem block's building
// has no envelope. buem-gateway resolves nothing from any external
// service — an earlier version called ignis to derive TABULA defaults when
// envelope was omitted, but that made buem-gateway's own "standalone,
// independently deployable" claim false (it silently needed a second
// service reachable at ignis-app:8080) and turned a missing-input mistake
// into a confusing downstream error from BuEM two hops away instead of a
// clear one here. The caller is responsible for supplying a complete
// envelope, resolving TABULA defaults itself beforehand if it needs to.
//
// Returns nil if buemRaw doesn't even parse as an object with a building
// key — that's a different, pre-existing failure mode (malformed JSON),
// left to normal request parsing to report.
func requireEnvelope(buemRaw json.RawMessage) error {
	var buem struct {
		Building struct {
			Envelope *struct {
				Elements []json.RawMessage `json:"elements"`
			} `json:"envelope"`
		} `json:"building"`
	}
	if err := json.Unmarshal(buemRaw, &buem); err != nil {
		return nil
	}
	if buem.Building.Envelope == nil || len(buem.Building.Envelope.Elements) == 0 {
		return ErrMissingEnvelope
	}
	return nil
}
