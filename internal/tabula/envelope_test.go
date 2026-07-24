package tabula

import "testing"

func TestBuildFallback_SplitsWallsAcrossFourOrientations(t *testing.T) {
	var data dataResponse
	data.TabulaData.BasicParameters.Envelope.AWall1 = 60
	data.TabulaData.AdvancedParameters.Uvalues.UWall1 = 1.4

	fallback := buildFallback(&data)

	walls := elementsOfType(fallback.Elements, "wall")
	if len(walls) != 4 {
		t.Fatalf("expected 4 wall elements (N/E/S/W), got %d", len(walls))
	}
	var total float64
	for _, w := range walls {
		total += w["area"].(map[string]interface{})["value"].(float64)
		if u := w["U"].(map[string]interface{})["value"]; u != 1.4 {
			t.Errorf("expected U=1.4 on every split wall element, got %v", u)
		}
	}
	if total != 60 {
		t.Fatalf("expected split areas to sum back to 60, got %v", total)
	}
}

func TestBuildFallback_ZeroAreaCategoryProducesNoElements(t *testing.T) {
	var data dataResponse
	data.TabulaData.BasicParameters.Envelope.AWall1 = 0
	data.TabulaData.BasicParameters.Envelope.AWall2 = 0
	data.TabulaData.BasicParameters.Envelope.AWall3 = 0

	fallback := buildFallback(&data)

	if len(elementsOfType(fallback.Elements, "wall")) != 0 {
		t.Fatal("expected no wall elements when every wall category has zero area")
	}
}

func TestBuildFallback_WindowsUseRealOrientationData(t *testing.T) {
	var data dataResponse
	env := &data.TabulaData.BasicParameters.Envelope
	env.AWindowSouth = 10
	env.AWindowNorth = 5
	env.AWindowEast = 0 // zero → no element
	data.TabulaData.AdvancedParameters.Uvalues.UWindow1 = 2.8

	fallback := buildFallback(&data)

	windows := elementsOfType(fallback.Elements, "window")
	if len(windows) != 2 {
		t.Fatalf("expected 2 window elements (south, north — east is zero), got %d", len(windows))
	}
	for _, w := range windows {
		azimuth := w["azimuth"].(map[string]interface{})["value"].(float64)
		area := w["area"].(map[string]interface{})["value"].(float64)
		switch azimuth {
		case 180:
			if area != 10 {
				t.Errorf("south window: expected area 10, got %v", area)
			}
		case 0:
			if area != 5 {
				t.Errorf("north window: expected area 5, got %v", area)
			}
		default:
			t.Errorf("unexpected azimuth %v", azimuth)
		}
	}
}

func TestBuildFallback_WindowUValueIsAreaWeightedAverage(t *testing.T) {
	var data dataResponse
	env := &data.TabulaData.BasicParameters.Envelope
	env.AWindow1, env.AWindow2 = 30, 10 // 3:1 weighting
	env.AWindowSouth = 40               // all area, so the one window element carries the blended U
	uval := &data.TabulaData.AdvancedParameters.Uvalues
	uval.UWindow1, uval.UWindow2 = 2.0, 6.0 // weighted avg = (30*2 + 10*6) / 40 = 3.0

	fallback := buildFallback(&data)

	windows := elementsOfType(fallback.Elements, "window")
	if len(windows) != 1 {
		t.Fatalf("expected 1 window element, got %d", len(windows))
	}
	if u := windows[0]["U"].(map[string]interface{})["value"]; u != 3.0 {
		t.Fatalf("expected area-weighted U=3.0, got %v", u)
	}
}

func TestBuildFallback_ARefFallsBackToLivingAreaWhenEstimIsZero(t *testing.T) {
	var data dataResponse
	data.TabulaData.BasicParameters.Envelope.ACRefEstim = 0
	data.TabulaData.BasicParameters.Envelope.ACLiving = 150

	fallback := buildFallback(&data)

	if fallback.ARef != 150 {
		t.Fatalf("expected ARef to fall back to ACLiving=150, got %v", fallback.ARef)
	}
}

func elementsOfType(elements []map[string]interface{}, elemType string) []map[string]interface{} {
	var out []map[string]interface{}
	for _, e := range elements {
		if e["type"] == elemType {
			out = append(out, e)
		}
	}
	return out
}
