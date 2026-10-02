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
// buem block as raw JSON. BuEM is asked for the hourly series only when
// keepTimeseries is true; otherwise it returns the summary figures alone and
// writes no intermediate file.
func RunFeature(client *httpclient.Client, cfg *config.Config, task Task, keepTimeseries bool) ([]byte, RunMetrics, error) {
	wallStart := time.Now()

	block, err := callUpstream(client, cfg, task, keepTimeseries)
	if err != nil {
		return nil, RunMetrics{}, err
	}

	enriched, err := finishBlock(block, keepTimeseries)
	if err != nil {
		return nil, RunMetrics{}, err
	}

	metrics := RunMetrics{
		WallDuration:           time.Since(wallStart),
		ModelProcessingSeconds: block.ModelMetadata.ProcessingTime.Value,
	}
	log.Printf("buem-gateway | node=%s lat=%.6f lon=%.6f year=%d wall=%s model=%.3fs",
		task.NodeID, task.Lat, task.Lon, task.Year,
		metrics.WallDuration.Round(time.Millisecond), metrics.ModelProcessingSeconds)

	return enriched, metrics, nil
}

func callUpstream(client *httpclient.Client, cfg *config.Config, task Task, includeTimeseries bool) (*ResponseBlock, error) {
	singleFC := FeatureCollection{
		Type:     "FeatureCollection",
		Features: []json.RawMessage{task.RawFeature},
	}
	url := cfg.BuEM.URL(apiProcessPath)
	if includeTimeseries {
		url += "?include_timeseries=true"
	}

	var respFC ResponseFeatureCollection
	if err := client.PostJSONAndDecode(url, singleFC, &respFC); err != nil {
		return nil, fmt.Errorf("BuEM request: %w", err)
	}
	if len(respFC.Features) == 0 {
		return nil, fmt.Errorf("BuEM response has no features")
	}
	return &respFC.Features[0].Properties.BUEM, nil
}

// finishBlock checks that BuEM returned the timeseries when it was asked for
// one. Without keepTimeseries no series was requested, and any BuEM sends is
// dropped. buem-model 6.4.0 and later write no file for an inline series, so
// there is nothing to clean up.
func finishBlock(block *ResponseBlock, keepTimeseries bool) ([]byte, error) {
	if keepTimeseries {
		ts := block.ThermalLoadProfile.Timeseries
		if ts == nil {
			return nil, fmt.Errorf("BuEM response missing timeseries (include_timeseries=true was requested)")
		}
		if len(ts.Heating) == 0 {
			return nil, fmt.Errorf("heating timeseries is empty")
		}
	} else {
		block.ThermalLoadProfile.Timeseries = nil
	}

	return json.Marshal(block)
}
