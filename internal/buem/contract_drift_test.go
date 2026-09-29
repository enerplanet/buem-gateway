package buem

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// v5ExampleBuemBlock returns features[0].properties.buem from the frozen
// contract example, schemas/v5/example_request.json.
func v5ExampleBuemBlock(t *testing.T) json.RawMessage {
	t.Helper()
	path := filepath.Join("..", "..", "schemas", "v5", "example_request.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Features []struct {
			Properties struct {
				BUEM json.RawMessage `json:"buem"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	if len(doc.Features) == 0 || len(doc.Features[0].Properties.BUEM) == 0 {
		t.Fatalf("%s has no features[0].properties.buem", path)
	}
	return doc.Features[0].Properties.BUEM
}

// TestValidatorsMatchV5Example ties requireEnvelope/requireWeather to the
// frozen schema: the contract's own example must pass the hand-written
// gates, and stripping the fields the schema marks required must fail them.
// If schemas/v5/request_schema.json changes shape without a matching change
// here, this fails.
func TestValidatorsMatchV5Example(t *testing.T) {
	block := v5ExampleBuemBlock(t)

	if err := requireEnvelope(block); err != nil {
		t.Fatalf("requireEnvelope rejected the v5 example: %v", err)
	}
	if err := requireWeather(block); err != nil {
		t.Fatalf("requireWeather rejected the v5 example: %v", err)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(block, &m); err != nil {
		t.Fatalf("parse buem block: %v", err)
	}

	withoutBuilding := cloneWithout(t, m, "building")
	if requireEnvelope(withoutBuilding) == nil {
		t.Error("requireEnvelope accepted a block with no building")
	}

	withoutWeather := cloneWithout(t, m, "weather")
	if requireWeather(withoutWeather) == nil {
		t.Error("requireWeather accepted a block with no weather")
	}
}

// TestValidatorsMatchV5ElementAndWeatherFields ties the per-element and
// weather-array-length checks to the frozen schema: dropping a field
// schemas/v5/request_schema.json marks required, or truncating a weather
// variable below index length, must fail the hand-written gates.
func TestValidatorsMatchV5ElementAndWeatherFields(t *testing.T) {
	var block map[string]any
	if err := json.Unmarshal(v5ExampleBuemBlock(t), &block); err != nil {
		t.Fatalf("parse v5 example buem block: %v", err)
	}

	elements := block["building"].(map[string]any)["envelope"].(map[string]any)["elements"].([]any)
	firstWall := elements[0].(map[string]any)
	if firstWall["type"] != "wall" {
		t.Fatalf("expected the v5 example's first element to be a wall, got %v", firstWall["type"])
	}
	delete(firstWall, "tilt")
	if err := requireEnvelope(mustMarshal(t, block)); err == nil {
		t.Error("requireEnvelope accepted a wall element with no tilt")
	}

	// Reload a clean copy and truncate a weather variable instead.
	if err := json.Unmarshal(v5ExampleBuemBlock(t), &block); err != nil {
		t.Fatalf("re-parse v5 example buem block: %v", err)
	}
	variables := block["weather"].(map[string]any)["variables"].(map[string]any)
	for name, values := range variables {
		variables[name] = values.([]any)[:0]
		break
	}
	if err := requireWeather(mustMarshal(t, block)); err == nil {
		t.Error("requireWeather accepted a weather block with a variable shorter than index")
	}
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}

func cloneWithout(t *testing.T, m map[string]json.RawMessage, key string) json.RawMessage {
	t.Helper()
	clone := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		if k == key {
			continue
		}
		clone[k] = v
	}
	out, err := json.Marshal(clone)
	if err != nil {
		t.Fatalf("marshal clone: %v", err)
	}
	return out
}
