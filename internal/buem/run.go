package buem

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/enerplanet/buem-gateway/internal/config"
	"github.com/enerplanet/buem-gateway/internal/httpclient"
)

const apiProcessPath = "/api/process"

// RunMetrics reports timing for one building's run, used for batch logging.
type RunMetrics struct {
	WallDuration           time.Duration
	ModelProcessingSeconds float64
}

// RunFeature sends one Task to the upstream BuEM service and returns the
// buem block as raw JSON, as BuEM sent it apart from the hourly series.
//
// With outputs in the request, BuEM decides which profiles and series come
// back and the block passes through unchanged. Without outputs, BuEM is asked
// for the series only when keepTimeseries is true, and any series it sends
// otherwise is dropped.
func RunFeature(client *httpclient.Client, cfg *config.Config, task Task, keepTimeseries bool) ([]byte, RunMetrics, error) {
	wallStart := time.Now()

	legacySeries := !task.HasOutputs && keepTimeseries
	block, err := callUpstream(client, cfg, task, legacySeries)
	if err != nil {
		return nil, RunMetrics{}, err
	}

	enriched, modelSeconds, err := finishBlock(block, !task.HasOutputs && !keepTimeseries, legacySeries)
	if err != nil {
		return nil, RunMetrics{}, err
	}

	metrics := RunMetrics{
		WallDuration:           time.Since(wallStart),
		ModelProcessingSeconds: modelSeconds,
	}
	log.Printf("buem-gateway | node=%s lat=%.6f lon=%.6f year=%d wall=%s model=%.3fs",
		task.NodeID, task.Lat, task.Lon, task.Year,
		metrics.WallDuration.Round(time.Millisecond), metrics.ModelProcessingSeconds)

	return enriched, metrics, nil
}

// callUpstream returns the first feature's buem block exactly as BuEM sent it.
func callUpstream(client *httpclient.Client, cfg *config.Config, task Task, includeTimeseries bool) (json.RawMessage, error) {
	singleFC := FeatureCollection{
		Type:     "FeatureCollection",
		Features: []json.RawMessage{task.RawFeature},
	}
	url := cfg.BuEM.URL(apiProcessPath)
	if includeTimeseries {
		url += "?include_timeseries=true"
	}

	var respFC struct {
		Features []struct {
			Properties struct {
				BUEM json.RawMessage `json:"buem"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := client.PostJSONAndDecode(url, singleFC, &respFC); err != nil {
		return nil, fmt.Errorf("BuEM request: %w", err)
	}
	if len(respFC.Features) == 0 || len(respFC.Features[0].Properties.BUEM) == 0 {
		return nil, fmt.Errorf("BuEM response has no features")
	}
	return respFC.Features[0].Properties.BUEM, nil
}

// finishBlock returns block with only the hourly series touched: dropped when
// stripSeries is true, and required to carry a heating series when
// requireSeries is true. Every other field passes through, including ones
// buem-gateway has no type for. It also returns the model's processing time.
func finishBlock(block json.RawMessage, stripSeries, requireSeries bool) ([]byte, float64, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(block, &fields); err != nil {
		return nil, 0, fmt.Errorf("BuEM response buem block is not an object: %w", err)
	}
	var meta struct {
		ProcessingTime Quantity `json:"processing_time"`
	}
	if raw, ok := fields["model_metadata"]; ok {
		_ = json.Unmarshal(raw, &meta) // processing time is for logging only; a malformed value logs as 0
	}
	if !stripSeries && !requireSeries {
		return block, meta.ProcessingTime.Value, nil
	}

	var profile map[string]json.RawMessage
	if err := json.Unmarshal(fields["thermal_load_profile"], &profile); err != nil {
		return nil, 0, fmt.Errorf("BuEM response has no thermal_load_profile object: %w", err)
	}
	if requireSeries {
		var ts struct {
			Heating []float64 `json:"heating"`
		}
		raw, ok := profile["timeseries"]
		if !ok || string(raw) == "null" {
			return nil, 0, fmt.Errorf("BuEM response missing timeseries (include_timeseries=true was requested)")
		}
		if err := json.Unmarshal(raw, &ts); err != nil || len(ts.Heating) == 0 {
			return nil, 0, fmt.Errorf("heating timeseries is empty")
		}
		return block, meta.ProcessingTime.Value, nil
	}

	delete(profile, "timeseries")
	rewritten, err := json.Marshal(profile)
	if err != nil {
		return nil, 0, err
	}
	fields["thermal_load_profile"] = rewritten
	out, err := json.Marshal(fields)
	return out, meta.ProcessingTime.Value, err
}
