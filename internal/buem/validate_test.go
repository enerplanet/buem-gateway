package buem

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// TestRequireEnvelope_ElementFields covers the per-element half of the
// schemas/v5 envelope_element contract that requireEnvelope enforces: id and
// type on every element, and area/azimuth/tilt on every non-ventilation
// element.
func TestRequireEnvelope_ElementFields(t *testing.T) {
	block := func(elements string) json.RawMessage {
		return json.RawMessage(`{"building":{"envelope":{"elements":[` + elements + `]}}}`)
	}
	for _, tc := range []struct {
		name     string
		elements string
		wantErr  string // "" means the block must pass
	}{
		{"complete wall", `{"id":"W1","type":"wall","area":10,"azimuth":0,"tilt":90}`, ""},
		{"ventilation needs no geometry", `{"id":"V1","type":"ventilation","air_changes":0.5}`, ""},
		{"missing id", `{"type":"wall","area":10,"azimuth":0,"tilt":90}`, "id is required"},
		{"empty id", `{"id":"","type":"wall","area":10,"azimuth":0,"tilt":90}`, "id is required"},
		{"missing type", `{"id":"W1","area":10,"azimuth":0,"tilt":90}`, "type is required"},
		{"unknown type", `{"id":"W1","type":"balcony","area":10,"azimuth":0,"tilt":90}`, "not one of"},
		{"wall missing area", `{"id":"W1","type":"wall","azimuth":0,"tilt":90}`, "area is required"},
		{"wall missing tilt", `{"id":"W1","type":"wall","area":10,"azimuth":0}`, "tilt is required"},
		{"null tilt", `{"id":"W1","type":"wall","area":10,"azimuth":0,"tilt":null}`, "tilt is required"},
		{"second element bad", `{"id":"W1","type":"wall","area":10,"azimuth":0,"tilt":90},{"id":"W2","type":"wall"}`, "elements[1]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := requireEnvelope(block(tc.elements))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("requireEnvelope() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("requireEnvelope() = %v, want an error containing %q", err, tc.wantErr)
			}
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("requireEnvelope() error does not wrap ErrInvalidRequest: %v", err)
			}
		})
	}
}

// TestRequireWeather_IndexParity covers the array-length half of the
// schemas/v5 weather contract: every variable array must be the same length
// as index.
func TestRequireWeather_IndexParity(t *testing.T) {
	for _, tc := range []struct {
		name    string
		weather string
		wantErr string
	}{
		{"matching lengths", `{"index":["a","b"],"variables":{"T":[1,2]}}`, ""},
		{"variable shorter than index", `{"index":["a","b","c"],"variables":{"T":[1,2]}}`, "index has 3"},
		{"variable longer than index", `{"index":["a"],"variables":{"T":[1,2]}}`, "index has 1"},
		{"one of several mismatched", `{"index":["a","b"],"variables":{"T":[1,2],"GHI":[1]}}`, "GHI has 1"},
		{"variable not an array", `{"index":["a"],"variables":{"T":5}}`, "not an array"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := requireWeather(json.RawMessage(`{"weather":` + tc.weather + `}`))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("requireWeather() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("requireWeather() = %v, want an error containing %q", err, tc.wantErr)
			}
			if !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("requireWeather() error does not wrap ErrInvalidRequest: %v", err)
			}
		})
	}
}

// TestValidateSingle_MatchesRunPath confirms ValidateSingle rejects exactly
// what TaskFromBuilding rejects: the guarantee that /api/v1/buem/validate
// and /api/v1/buem/building never disagree.
func TestValidateSingle_MatchesRunPath(t *testing.T) {
	in := testBuildingInput("b1")
	if err := ValidateSingle(in, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60); err != nil {
		t.Fatalf("ValidateSingle() rejected a complete request: %v", err)
	}

	in.Geometry = json.RawMessage(`{"type":"Point","coordinates":[12.5]}`)
	err := ValidateSingle(in, "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("ValidateSingle() = %v, want it to wrap ErrInvalidRequest", err)
	}
}
