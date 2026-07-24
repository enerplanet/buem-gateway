package buem

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Task is one building extracted from a request topology, ready to send to
// the upstream BuEM Flask API.
type Task struct {
	NodeID     string
	Lat, Lon   float64
	Year       int
	ModelID    string // isolates CSV output per model; empty for a bare test request
	RawFeature json.RawMessage
}

// ExtractTasks walks a topology's edges, finds BasePOI nodes that carry a
// "buem" block, deduplicates by node ID, and returns one Task per building.
// resolver fills in a missing building.envelope from TABULA defaults; pass
// nil to disable that (a request lacking envelope then fails at BuEM instead).
func ExtractTasks(rawTopology json.RawMessage, startDate, endDate string, resolution int, modelID string, resolver envelopeResolver) ([]Task, error) {
	var rawEdges []json.RawMessage
	if err := json.Unmarshal(rawTopology, &rawEdges); err != nil {
		return nil, fmt.Errorf("parse topology edges: %w", err)
	}

	var tasks []Task
	seen := make(map[string]bool)
	for _, rawEdge := range rawEdges {
		for _, task := range tasksFromEdge(rawEdge, startDate, endDate, resolution, modelID, resolver) {
			if seen[task.NodeID] {
				continue
			}
			seen[task.NodeID] = true
			tasks = append(tasks, task)
		}
	}
	return tasks, nil
}

func tasksFromEdge(rawEdge json.RawMessage, startDate, endDate string, resolution int, modelID string, resolver envelopeResolver) []Task {
	var edge struct {
		From json.RawMessage `json:"from"`
		To   json.RawMessage `json:"to"`
	}
	if err := json.Unmarshal(rawEdge, &edge); err != nil {
		return nil
	}

	var tasks []Task
	for _, rawNode := range []json.RawMessage{edge.From, edge.To} {
		if task, ok := nodeToTask(rawNode, startDate, endDate, resolution, modelID, resolver); ok {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// nodeToTask checks whether a topology node is a building with a buem block
// and, if so, builds the BuEM Feature that will be sent to the service.
func nodeToTask(rawNode json.RawMessage, startDate, endDate string, resolution int, modelID string, resolver envelopeResolver) (Task, bool) {
	node, ok := parseTopologyNode(rawNode)
	if !ok {
		return Task{}, false
	}

	year, err := yearFromStartTime(startDate)
	if err != nil {
		return Task{}, false
	}

	node.Properties.BUEM = ensureEnvelope(node.Properties.BUEM, resolver, node.ID)

	rawFeature, err := buildFeature(node, startDate, endDate, resolution)
	if err != nil {
		return Task{}, false
	}

	return Task{
		NodeID:     node.ID,
		Lat:        node.Geometry.Coordinates[1],
		Lon:        node.Geometry.Coordinates[0],
		Year:       year,
		ModelID:    modelID,
		RawFeature: rawFeature,
	}, true
}

type topologyNode struct {
	ID       string `json:"id"`
	Geometry struct {
		Coordinates []float64 `json:"coordinates"`
	} `json:"geometry"`
	Properties struct {
		FeatureType string          `json:"feature_type"`
		BUEM        json.RawMessage `json:"buem"`
	} `json:"properties"`
}

// parseTopologyNode decodes a raw topology node and reports whether it is a
// building carrying a non-empty buem block.
func parseTopologyNode(rawNode json.RawMessage) (topologyNode, bool) {
	var node topologyNode
	if err := json.Unmarshal(rawNode, &node); err != nil {
		return topologyNode{}, false
	}
	if node.Properties.FeatureType != "BasePOI" {
		return topologyNode{}, false
	}
	if len(node.Properties.BUEM) == 0 || string(node.Properties.BUEM) == "null" {
		return topologyNode{}, false
	}
	if len(node.Geometry.Coordinates) < 2 {
		return topologyNode{}, false
	}
	return node, true
}

// buildFeature wraps a topology node's buem block in the GeoJSON Feature
// shape BuEM's API expects.
func buildFeature(node topologyNode, startDate, endDate string, resolution int) (json.RawMessage, error) {
	return json.Marshal(map[string]interface{}{
		"type": "Feature",
		"id":   node.ID,
		"geometry": map[string]interface{}{
			"type":        "Point",
			"coordinates": node.Geometry.Coordinates,
		},
		"properties": map[string]interface{}{
			"start_time":      startDate,
			"end_time":        endDate,
			"resolution":      strconv.Itoa(resolution),
			"resolution_unit": "minutes",
			"buem":            node.Properties.BUEM,
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
