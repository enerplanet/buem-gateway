package buem

import (
	"encoding/json"
	"fmt"
)

// ErrMissingEnvelope is returned by RunSingle when building.envelope is
// absent or empty. Callers (e.g. the HTTP handler) can check for it, or the
// broader ErrInvalidRequest it wraps, with errors.Is to distinguish "you
// sent an incomplete request" (400) from "BuEM tried to run it and failed"
// (422) — this check runs before BuEM is ever called, so the latter status
// would be misleading.
var ErrMissingEnvelope = fmt.Errorf("building.envelope is required with at least one element — buem-gateway does not resolve missing geometry from any external service, the caller must supply a complete envelope: %w", ErrInvalidRequest)

// envelopeElementTypes is the fixed set from schemas/v5 $defs/envelope_element
// (type enum).
var envelopeElementTypes = map[string]bool{
	"wall": true, "roof": true, "floor": true,
	"window": true, "door": true, "ventilation": true,
}

// requireEnvelope reports ErrMissingEnvelope if the buem block's building has
// no envelope, and a wrapped ErrInvalidRequest if any element is missing a
// field schemas/v5/request_schema.json marks required. This is the
// hand-written half of that contract (building.envelope.elements minItems 1;
// per-element id + type; area/azimuth/tilt on every non-ventilation
// element); TestValidatorsMatchV5Example fails if they diverge.
//
// buem-gateway resolves nothing from any external
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
	for i, raw := range buem.Building.Envelope.Elements {
		if err := checkEnvelopeElement(i, raw); err != nil {
			return err
		}
	}
	return nil
}

// checkEnvelopeElement enforces the schemas/v5 envelope_element contract:
// id and type on every element (type from envelopeElementTypes), plus area,
// azimuth and tilt on every non-ventilation element. Fields are checked for
// presence only — BuEM is the authority on their values.
func checkEnvelopeElement(i int, raw json.RawMessage) error {
	var el map[string]json.RawMessage
	if err := json.Unmarshal(raw, &el); err != nil {
		return fmt.Errorf("building.envelope.elements[%d] is not an object: %w", i, ErrInvalidRequest)
	}
	if !present(el["id"]) || string(el["id"]) == `""` {
		return fmt.Errorf("building.envelope.elements[%d].id is required: %w", i, ErrInvalidRequest)
	}
	label := fmt.Sprintf("building.envelope.elements[%d] (id %s)", i, jsonString(el["id"]))
	if !present(el["type"]) {
		return fmt.Errorf("%s.type is required: %w", label, ErrInvalidRequest)
	}
	typ := jsonString(el["type"])
	if !envelopeElementTypes[typ] {
		return fmt.Errorf("%s.type %s is not one of wall/roof/floor/window/door/ventilation: %w", label, el["type"], ErrInvalidRequest)
	}
	if typ == "ventilation" {
		return nil
	}
	for _, field := range [...]string{"area", "azimuth", "tilt"} {
		if !present(el[field]) {
			return fmt.Errorf("%s.%s is required for a %s element: %w", label, field, typ, ErrInvalidRequest)
		}
	}
	return nil
}

// present reports whether a JSON object member was set to something other
// than null.
func present(raw json.RawMessage) bool {
	return len(raw) > 0 && string(raw) != "null"
}

// jsonString returns raw decoded as a JSON string, or "" if it is absent or
// not a string. The error is dropped deliberately: a non-string value where
// the schema requires one is reported by the caller as a bad type, using the
// raw JSON in the message.
func jsonString(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}
