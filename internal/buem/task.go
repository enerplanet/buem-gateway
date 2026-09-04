package buem

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Task is one building extracted from a request, ready to send to the
// upstream BuEM Flask API.
type Task struct {
	NodeID     string
	Lat, Lon   float64
	Year       int
	ModelID    string // isolates CSV output per model; empty for a bare test request
	RawFeature json.RawMessage
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
	if err := requireEnvelope(in.BUEM); err != nil {
		return Task{}, err
	}
	if err := requireWeather(in.BUEM); err != nil {
		return Task{}, err
	}

	var geom struct {
		Type        string    `json:"type"`
		Coordinates []float64 `json:"coordinates"`
	}
	if err := json.Unmarshal(in.Geometry, &geom); err != nil || len(geom.Coordinates) < 2 {
		return Task{}, fmt.Errorf("geometry.coordinates must be a [lon, lat] pair")
	}
	// BuEM's schema fixes geometry.type to "Point"; without it BuEM rejects
	// the feature with a generic "Invalid GeoJSON payload" that names no field.
	if geom.Type != "Point" {
		return Task{}, fmt.Errorf("geometry.type must be \"Point\", got %q", geom.Type)
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
		ModelID:    modelID,
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
		return 0, fmt.Errorf("start_time too short: %q", s)
	}
	return strconv.Atoi(s[:4])
}
