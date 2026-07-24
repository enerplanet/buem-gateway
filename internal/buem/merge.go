package buem

import (
	"encoding/json"
	"fmt"
)

// outcome is the result of running one Task, keyed by NodeID for merging
// back into the topology.
type outcome struct {
	nodeID    string
	buemBlock json.RawMessage // enriched properties.buem to merge back
	errMsg    string
	metrics   RunMetrics
}

// mergeIntoTopology injects each successful outcome's buem block into its
// matching topology node, leaving all other node fields — and nodes with no
// outcome or a failed one — unchanged.
func mergeIntoTopology(rawTopology json.RawMessage, results map[string]outcome) (json.RawMessage, error) {
	var edges []interface{}
	if err := json.Unmarshal(rawTopology, &edges); err != nil {
		return nil, fmt.Errorf("parse topology: %w", err)
	}

	for _, edge := range edges {
		mergeIntoEdge(edge, results)
	}
	return json.Marshal(edges)
}

func mergeIntoEdge(edge interface{}, results map[string]outcome) {
	edgeMap, ok := edge.(map[string]interface{})
	if !ok {
		return
	}
	for _, side := range []string{"from", "to"} {
		mergeIntoNode(edgeMap[side], results)
	}
}

func mergeIntoNode(rawNode interface{}, results map[string]outcome) {
	nodeMap, ok := rawNode.(map[string]interface{})
	if !ok {
		return
	}
	propsMap, ok := nodeMap["properties"].(map[string]interface{})
	if !ok {
		return
	}

	nodeID := fmt.Sprintf("%v", nodeMap["id"])
	result, found := results[nodeID]
	if !found || result.errMsg != "" {
		return
	}

	var buemData interface{}
	if err := json.Unmarshal(result.buemBlock, &buemData); err != nil {
		return
	}
	propsMap["buem"] = buemData
}
