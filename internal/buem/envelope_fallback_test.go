package buem

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/tabula"
)

type stubResolver struct {
	fallback                       *tabula.Fallback
	err                            error
	called                         bool
	gotCountry, gotType, gotPeriod string
}

func (s *stubResolver) Resolve(country, buildingType, constructionPeriod string) (*tabula.Fallback, error) {
	s.called = true
	s.gotCountry, s.gotType, s.gotPeriod = country, buildingType, constructionPeriod
	return s.fallback, s.err
}

func TestEnsureEnvelope_FillsInWhenMissing(t *testing.T) {
	resolver := &stubResolver{fallback: &tabula.Fallback{
		Elements: []map[string]interface{}{
			{"id": "Wall_1", "type": "wall", "area": map[string]interface{}{"value": 20.0, "unit": "m2"}},
		},
		ARef: 120, HRoom: 2.5, NStoreys: 2,
		NeighbourStatus: "B_Alone", AtticCondition: "N", CellarCondition: "C",
		NAirInfiltration: 0.5, NAirUse: 0.4,
	}}
	buemRaw := []byte(`{"building":{"building_type":"SFH","construction_period":"01","country":"DE"}}`)

	enriched := ensureEnvelope(buemRaw, resolver, "building-1")

	if !resolver.called {
		t.Fatal("expected resolver.Resolve to be called")
	}
	if resolver.gotCountry != "DE" || resolver.gotType != "SFH" || resolver.gotPeriod != "01" {
		t.Fatalf("resolver called with wrong params: country=%s type=%s period=%s", resolver.gotCountry, resolver.gotType, resolver.gotPeriod)
	}

	var buem map[string]interface{}
	if err := json.Unmarshal(enriched, &buem); err != nil {
		t.Fatalf("unmarshal enriched buem: %v", err)
	}
	building := buem["building"].(map[string]interface{})
	envelope, ok := building["envelope"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected envelope to be injected, got %v", building)
	}
	elements := envelope["elements"].([]interface{})
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}
	if aRef := building["A_ref"].(map[string]interface{})["value"]; aRef != 120.0 {
		t.Fatalf("expected A_ref 120, got %v", aRef)
	}
	thermal := building["thermal"].(map[string]interface{})
	if nAir := thermal["n_air_infiltration"].(map[string]interface{})["value"]; nAir != 0.5 {
		t.Fatalf("expected n_air_infiltration 0.5, got %v", nAir)
	}
}

func TestEnsureEnvelope_LeavesExistingEnvelopeUntouched(t *testing.T) {
	resolver := &stubResolver{}
	buemRaw := []byte(`{"building":{"building_type":"SFH","country":"DE","envelope":{"elements":[{"id":"Wall_1"}]}}}`)

	enriched := ensureEnvelope(buemRaw, resolver, "building-1")

	if resolver.called {
		t.Fatal("resolver should not be called when envelope is already present")
	}
	if string(enriched) != string(buemRaw) {
		t.Fatalf("expected buemRaw unchanged, got %s", enriched)
	}
}

func TestEnsureEnvelope_NilResolverPassesThrough(t *testing.T) {
	buemRaw := []byte(`{"building":{"building_type":"SFH","country":"DE"}}`)
	enriched := ensureEnvelope(buemRaw, nil, "building-1")
	if string(enriched) != string(buemRaw) {
		t.Fatalf("expected buemRaw unchanged with nil resolver, got %s", enriched)
	}
}

func TestEnsureEnvelope_ResolverErrorFallsBackToOriginal(t *testing.T) {
	resolver := &stubResolver{err: errors.New("ignis unreachable")}
	buemRaw := []byte(`{"building":{"building_type":"SFH","country":"DE"}}`)

	enriched := ensureEnvelope(buemRaw, resolver, "building-1")

	if !resolver.called {
		t.Fatal("expected resolver.Resolve to be called")
	}
	if string(enriched) != string(buemRaw) {
		t.Fatalf("expected buemRaw unchanged on resolver error, got %s", enriched)
	}
}

func TestEnsureEnvelope_DoesNotOverwriteExplicitFields(t *testing.T) {
	resolver := &stubResolver{fallback: &tabula.Fallback{
		Elements: []map[string]interface{}{{"id": "Wall_1"}},
		ARef:     120,
	}}
	buemRaw := []byte(`{"building":{"building_type":"SFH","country":"DE","A_ref":{"value":999,"unit":"m2"}}}`)

	enriched := ensureEnvelope(buemRaw, resolver, "building-1")

	var buem map[string]interface{}
	json.Unmarshal(enriched, &buem)
	building := buem["building"].(map[string]interface{})
	if aRef := building["A_ref"].(map[string]interface{})["value"]; aRef != 999.0 {
		t.Fatalf("expected caller-supplied A_ref 999 to be preserved, got %v", aRef)
	}
}
