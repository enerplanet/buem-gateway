package buem

import (
	"encoding/json"
	"log"

	"github.com/enerplanet/buem-gateway/internal/tabula"
)

// envelopeResolver is the subset of tabula.Client this package depends on —
// lets tests substitute a stub without a real ignis connection.
type envelopeResolver interface {
	Resolve(country, buildingType, constructionPeriod string) (*tabula.Fallback, error)
}

// ensureEnvelope fills in building.envelope from TABULA (via resolver) when
// the caller omitted it. If resolver is nil, envelope is already present, or
// resolution fails, buemRaw is returned unchanged — BuEM's own validator then
// produces a clear "envelope required" error, rather than this failing silently.
func ensureEnvelope(buemRaw json.RawMessage, resolver envelopeResolver, nodeID string) json.RawMessage {
	if resolver == nil {
		return buemRaw
	}

	var buem map[string]interface{}
	if err := json.Unmarshal(buemRaw, &buem); err != nil {
		return buemRaw
	}
	building, ok := buem["building"].(map[string]interface{})
	if !ok || hasEnvelope(building) {
		return buemRaw
	}

	fallback, err := resolveFallback(building, resolver)
	if err != nil {
		log.Printf("buem-gateway | node=%s TABULA fallback failed, forwarding as-is: %v", nodeID, err)
		return buemRaw
	}

	applyFallback(building, fallback)
	buem["building"] = building
	enriched, err := json.Marshal(buem)
	if err != nil {
		log.Printf("buem-gateway | node=%s failed to re-marshal after TABULA fallback: %v", nodeID, err)
		return buemRaw
	}
	return enriched
}

func hasEnvelope(building map[string]interface{}) bool {
	envelope, ok := building["envelope"].(map[string]interface{})
	if !ok {
		return false
	}
	elements, ok := envelope["elements"].([]interface{})
	return ok && len(elements) > 0
}

func resolveFallback(building map[string]interface{}, resolver envelopeResolver) (*tabula.Fallback, error) {
	buildingType, _ := building["building_type"].(string)
	constructionPeriod, _ := building["construction_period"].(string)
	country, _ := building["country"].(string)
	return resolver.Resolve(country, buildingType, constructionPeriod)
}

// applyFallback merges fallback into building in place. Fields the caller
// already supplied are left untouched — the fallback only fills genuine gaps.
func applyFallback(building map[string]interface{}, fallback *tabula.Fallback) {
	building["envelope"] = map[string]interface{}{"elements": fallback.Elements}

	setIfAbsent(building, "A_ref", quantity(fallback.ARef, "m2"))
	setIfAbsent(building, "h_room", quantity(fallback.HRoom, "m"))
	setIfAbsent(building, "n_storeys", fallback.NStoreys)
	setIfAbsent(building, "neighbour_status", fallback.NeighbourStatus)
	setIfAbsent(building, "attic_condition", fallback.AtticCondition)
	setIfAbsent(building, "cellar_condition", fallback.CellarCondition)

	thermal, ok := building["thermal"].(map[string]interface{})
	if !ok {
		thermal = map[string]interface{}{}
	}
	setIfAbsent(thermal, "n_air_infiltration", quantity(fallback.NAirInfiltration, "1/h"))
	setIfAbsent(thermal, "n_air_use", quantity(fallback.NAirUse, "1/h"))
	building["thermal"] = thermal
}

func setIfAbsent(m map[string]interface{}, key string, value interface{}) {
	if _, present := m[key]; !present {
		m[key] = value
	}
}

func quantity(value float64, unit string) map[string]interface{} {
	return map[string]interface{}{"value": value, "unit": unit}
}
