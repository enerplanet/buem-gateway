package tabula

// Fallback synthesizes BuEM's per-element envelope.elements from TABULA's
// aggregate category data, plus the handful of building-level fields TABULA
// supplies directly.
type Fallback struct {
	Elements         []map[string]interface{}
	ARef             float64
	HRoom            float64
	NStoreys         int
	NeighbourStatus  string
	AtticCondition   string
	CellarCondition  string
	NAirInfiltration float64
	NAirUse          float64
}

// windowOrientations maps TABULA's window-orientation fields to a
// representative azimuth. "Horizontal" (skylights) gets tilt 0 instead of 90
// in buildWindowElements — there is no meaningful azimuth for a flat surface,
// so it's set to 0 (ignored by the model for a horizontal element).
var windowOrientations = []struct {
	area    func(e *dataResponse) float64
	azimuth float64
	label   string
}{
	{func(e *dataResponse) float64 { return e.TabulaData.BasicParameters.Envelope.AWindowNorth }, 0, "North"},
	{func(e *dataResponse) float64 { return e.TabulaData.BasicParameters.Envelope.AWindowEast }, 90, "East"},
	{func(e *dataResponse) float64 { return e.TabulaData.BasicParameters.Envelope.AWindowSouth }, 180, "South"},
	{func(e *dataResponse) float64 { return e.TabulaData.BasicParameters.Envelope.AWindowWest }, 270, "West"},
	{func(e *dataResponse) float64 { return e.TabulaData.BasicParameters.Envelope.AWindowHorizontal }, 0, "Horizontal"},
}

// wallOrientations splits each wall category's aggregate area evenly across
// the 4 cardinal directions — TABULA gives no per-wall orientation, and an
// even split is the standard assumption for a building whose actual facade
// layout isn't known (see decisions/2026-07-24-buem-gateway-standalone-repo.md).
var wallOrientations = []struct {
	azimuth float64
	label   string
}{
	{0, "N"}, {90, "E"}, {180, "S"}, {270, "W"},
}

// buildFallback maps ignis's raw TABULA response into a Fallback ready to
// merge into a request's building block.
func buildFallback(data *dataResponse) Fallback {
	env := data.TabulaData.BasicParameters.Envelope
	uval := data.TabulaData.AdvancedParameters.Uvalues
	air := data.TabulaData.AdvancedParameters.AirInfiltration
	appearance := data.TabulaData.BasicParameters.BuildingAppearance

	var elements []map[string]interface{}
	elements = append(elements, buildWallElements("Wall_1", env.AWall1, uval.UWall1)...)
	elements = append(elements, buildWallElements("Wall_2", env.AWall2, uval.UWall2)...)
	elements = append(elements, buildWallElements("Wall_3", env.AWall3, uval.UWall3)...)
	elements = append(elements, buildSingleElement("Roof_1", "roof", env.ARoof1, uval.URoof1, 180, 30)...)
	elements = append(elements, buildSingleElement("Roof_2", "roof", env.ARoof2, uval.URoof2, 180, 30)...)
	elements = append(elements, buildSingleElement("Floor_1", "floor", env.AFloor1, uval.UFloor1, 0, 0)...)
	elements = append(elements, buildSingleElement("Floor_2", "floor", env.AFloor2, uval.UFloor2, 0, 0)...)
	elements = append(elements, buildWindowElements(data)...)
	elements = append(elements, buildSingleElement("Door_1", "door", env.ADoor1, uval.UDoor1, 0, 90)...)
	elements = append(elements, map[string]interface{}{
		"id": "Vent_1", "type": "ventilation",
		"air_changes": quantity(air.NAirUse, "1/h"),
	})

	aRef := env.ACRefEstim
	if aRef <= 0 {
		aRef = env.ACLiving
	}

	return Fallback{
		Elements:         elements,
		ARef:             aRef,
		HRoom:            appearance.HRoom,
		NStoreys:         appearance.NStorey,
		NeighbourStatus:  appearance.AttachedNeighb,
		AtticCondition:   appearance.AtticCondition,
		CellarCondition:  appearance.CellarCondition,
		NAirInfiltration: air.NAirInfiltration,
		NAirUse:          air.NAirUse,
	}
}

// buildWallElements splits one TABULA wall category's aggregate area evenly
// across the 4 cardinal directions. Returns nothing if area is 0 (category unused).
func buildWallElements(idPrefix string, area, uValue float64) []map[string]interface{} {
	if area <= 0 {
		return nil
	}
	perElement := area / float64(len(wallOrientations))
	elements := make([]map[string]interface{}, 0, len(wallOrientations))
	for _, o := range wallOrientations {
		elements = append(elements, map[string]interface{}{
			"id": idPrefix + "_" + o.label, "type": "wall",
			"area": quantity(perElement, "m2"), "azimuth": quantity(o.azimuth, "deg"), "tilt": quantity(90, "deg"),
			"U": quantity(uValue, "W/(m2K)"),
		})
	}
	return elements
}

// buildSingleElement builds one envelope element for a TABULA category that
// has no per-orientation data (roof, floor, door) — one element at the full
// category area, with an assumed representative azimuth/tilt. Returns
// nothing if area is 0.
func buildSingleElement(id, elemType string, area, uValue, azimuth, tilt float64) []map[string]interface{} {
	if area <= 0 {
		return nil
	}
	return []map[string]interface{}{{
		"id": id, "type": elemType,
		"area": quantity(area, "m2"), "azimuth": quantity(azimuth, "deg"), "tilt": quantity(tilt, "deg"),
		"U": quantity(uValue, "W/(m2K)"),
	}}
}

// buildWindowElements uses TABULA's real per-orientation window areas
// directly (unlike walls, TABULA does track this). The U-value is an
// area-weighted average of the two window categories, since TABULA doesn't
// give U-value per orientation.
func buildWindowElements(data *dataResponse) []map[string]interface{} {
	env := data.TabulaData.BasicParameters.Envelope
	uval := data.TabulaData.AdvancedParameters.Uvalues
	uWindow := weightedAverage(
		[]float64{env.AWindow1, env.AWindow2},
		[]float64{uval.UWindow1, uval.UWindow2},
	)

	var elements []map[string]interface{}
	for _, o := range windowOrientations {
		area := o.area(data)
		if area <= 0 {
			continue
		}
		tilt := 90.0
		if o.label == "Horizontal" {
			tilt = 0
		}
		elements = append(elements, map[string]interface{}{
			"id": "Window_" + o.label, "type": "window",
			"area": quantity(area, "m2"), "azimuth": quantity(o.azimuth, "deg"), "tilt": quantity(tilt, "deg"),
			"U": quantity(uWindow, "W/(m2K)"), "g_gl": quantity(0.5, "-"), // TABULA's raw g_gl isn't in the data endpoint; 0.5 matches BuEM's own default.
		})
	}
	return elements
}

// weightedAverage returns the area-weighted average of values, skipping
// zero-area categories. Returns 0 if every area is 0.
func weightedAverage(areas, values []float64) float64 {
	var totalArea, weighted float64
	for i, a := range areas {
		if a <= 0 {
			continue
		}
		totalArea += a
		weighted += a * values[i]
	}
	if totalArea == 0 {
		return 0
	}
	return weighted / totalArea
}

// quantity wraps a value in BuEM's {value, unit} measurement shape.
func quantity(value float64, unit string) map[string]interface{} {
	return map[string]interface{}{"value": value, "unit": unit}
}
