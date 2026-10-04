package buem

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
)

// modelIDPattern is the character set allowed in model_id.
var modelIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateModelID rejects a model_id outside modelIDPattern. model_id is
// accepted for compatibility and has no effect on a run; an empty value is
// valid.
func ValidateModelID(modelID string) error {
	if modelID == "" {
		return nil
	}
	if !modelIDPattern.MatchString(modelID) || modelID == "." || modelID == ".." {
		return fmt.Errorf("model_id may contain only letters, digits, '.', '_' and '-', and may not be \".\" or \"..\", got %q: %w", modelID, ErrInvalidRequest)
	}
	return nil
}

// Task is one building extracted from a request, ready to send to the
// upstream BuEM Flask API.
type Task struct {
	NodeID   string
	Lat, Lon float64
	Year     int
	// HasOutputs is true when the request's buem block carries outputs, which
	// then decides on BuEM's side which profiles and series come back.
	HasOutputs bool
	RawFeature json.RawMessage
}

// outputLevels are the values each profile in buem.outputs may take.
var outputLevels = map[string]bool{"": true, "none": true, "summary": true, "series": true}

// outputProfiles are the profile names buem.outputs may contain.
var outputProfiles = map[string]bool{"heating": true, "cooling": true, "electricity": true, "hot_water": true, "kitchen": true}

// readOutputs reports whether buemBlock carries outputs and whether it selects
// a thermal profile. An omitted profile counts as selected, so a block
// without outputs selects heating and cooling.
func readOutputs(buemBlock json.RawMessage) (present, thermal bool, err error) {
	var block struct {
		Outputs *map[string]string `json:"outputs"`
	}
	if err := json.Unmarshal(buemBlock, &block); err != nil {
		return false, false, fmt.Errorf("buem.outputs must map profile names to none, summary or series: %w", ErrInvalidRequest)
	}
	if block.Outputs == nil {
		return false, true, nil
	}
	for name, level := range *block.Outputs {
		if !outputProfiles[name] {
			return false, false, fmt.Errorf("buem.outputs.%s is not a profile; use heating, cooling, electricity, hot_water or kitchen: %w", name, ErrInvalidRequest)
		}
		if !outputLevels[level] {
			return false, false, fmt.Errorf("buem.outputs.%s must be none, summary or series, got %q: %w", name, level, ErrInvalidRequest)
		}
	}
	o := *block.Outputs
	return true, o["heating"] != "none" || o["cooling"] != "none", nil
}

// BuildingInput is one building's request data — the id/geometry/buem shape
// shared by /api/v1/buem/building and /api/v1/buem/buildings.
type BuildingInput struct {
	ID       string
	Geometry json.RawMessage
	BUEM     json.RawMessage
}

// TaskFromBuilding validates in's envelope and weather and builds the Task
// that will be sent to BuEM. Returns ErrMissingEnvelope or ErrMissingWeather
// (see requireEnvelope/requireWeather) if in isn't ready to run — the
// caller must supply a complete buem block, buem-gateway resolves nothing
// from any external service.
func TaskFromBuilding(in BuildingInput, startDate, endDate string, resolution int, modelID string) (Task, error) {
	if err := ValidateModelID(modelID); err != nil {
		return Task{}, err
	}
	hasOutputs, thermal, err := readOutputs(in.BUEM)
	if err != nil {
		return Task{}, err
	}
	// Envelope and weather feed only the thermal model; a request selecting
	// neither heating nor cooling may omit both.
	if thermal {
		if err := requireEnvelope(in.BUEM); err != nil {
			return Task{}, err
		}
		if err := requireWeather(in.BUEM); err != nil {
			return Task{}, err
		}
	}

	var geom struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(in.Geometry, &geom); err != nil || len(geom.Coordinates) < 2 {
		return Task{}, fmt.Errorf("geometry.coordinates must be a [lon, lat] pair: %w", ErrInvalidRequest)
	}
	// BuEM's schema fixes geometry.type to "Point"; without it BuEM rejects
	// the feature with a generic "Invalid GeoJSON payload" that names no field.
	if geom.Type != "Point" {
		return Task{}, fmt.Errorf("geometry.type must be \"Point\", got %q: %w", geom.Type, ErrInvalidRequest)
	}

	year, err := yearFromStartTime(startDate)
	if err != nil {
		return Task{}, err
	}

	rawFeature, err := buildFeature(in, startDate, endDate, resolution)
	if err != nil {
		return Task{}, err
	}

	return Task{
		NodeID:     in.ID,
		Lat:        geom.Coordinates[1],
		Lon:        geom.Coordinates[0],
		Year:       year,
		HasOutputs: hasOutputs,
		RawFeature: rawFeature,
	}, nil
}

// buildFeature wraps a building input in the GeoJSON Feature shape BuEM's
// API expects.
func buildFeature(in BuildingInput, startDate, endDate string, resolution int) (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"type":     "Feature",
		"id":       in.ID,
		"geometry": in.Geometry,
		"properties": map[string]interface{}{
			"start_time":      startDate,
			"end_time":        endDate,
			"resolution":      strconv.Itoa(resolution),
			"resolution_unit": "minutes",
			"buem":            in.BUEM,
		},
	})
}

// yearFromStartTime parses the 4-digit year from an ISO 8601 timestamp
// string — it selects which MERRA-2 weather file BuEM uses.
func yearFromStartTime(s string) (int, error) {
	if len(s) < 4 {
		return 0, fmt.Errorf("start_date is required as an ISO 8601 timestamp, got %q: %w", s, ErrInvalidRequest)
	}
	year, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0, fmt.Errorf("start_date does not begin with a 4-digit year: %q: %w", s, ErrInvalidRequest)
	}
	return year, nil
}
