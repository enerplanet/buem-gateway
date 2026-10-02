package buem

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enerplanet/buem-gateway/internal/config"
	"github.com/enerplanet/buem-gateway/internal/httpclient"
)

const (
	apiProcessPath  = "/api/process"
	buemFilesPrefix = "/api/files/"
)

// RunMetrics reports timing for one building's run, used for batch logging.
type RunMetrics struct {
	WallDuration           time.Duration
	ModelProcessingSeconds float64
}

// RunFeature sends one Task to the upstream BuEM service and returns the
// buem block as raw JSON. The inline timeseries is stripped unless
// keepTimeseries is true: a batch caller that needs only the summary figures
// gets a response roughly 300 KB per building smaller.
func RunFeature(client *httpclient.Client, cfg *config.Config, task Task, keepTimeseries bool) ([]byte, RunMetrics, error) {
	wallStart := time.Now()

	block, err := callUpstream(client, cfg, task)
	if err != nil {
		return nil, RunMetrics{}, err
	}

	enriched, err := finishBlock(cfg, block, keepTimeseries)
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

func callUpstream(client *httpclient.Client, cfg *config.Config, task Task) (*ResponseBlock, error) {
	singleFC := FeatureCollection{
		Type:     "FeatureCollection",
		Features: []json.RawMessage{task.RawFeature},
	}
	url := cfg.BuEM.URL(apiProcessPath) + "?include_timeseries=true"

	var respFC ResponseFeatureCollection
	if err := client.PostJSONAndDecode(url, singleFC, &respFC); err != nil {
		return nil, fmt.Errorf("BuEM request: %w", err)
	}
	if len(respFC.Features) == 0 {
		return nil, fmt.Errorf("BuEM response has no features")
	}
	return &respFC.Features[0].Properties.BUEM, nil
}

// finishBlock checks that BuEM returned the timeseries it was asked for,
// deletes the intermediate file BuEM wrote for it, and strips the inline
// series unless keepTimeseries is true.
func finishBlock(cfg *config.Config, block *ResponseBlock, keepTimeseries bool) ([]byte, error) {
	ts := block.ThermalLoadProfile.Timeseries
	if ts == nil {
		return nil, fmt.Errorf("BuEM response missing timeseries (include_timeseries=true was requested)")
	}
	if len(ts.Heating) == 0 {
		return nil, fmt.Errorf("heating timeseries is empty")
	}

	deleteSourceTimeseries(cfg, block.ThermalLoadProfile.TimeseriesFile)
	if !keepTimeseries {
		block.ThermalLoadProfile.Timeseries = nil
	}
	return json.Marshal(block)
}

// deleteSourceTimeseries removes the .json.gz file BuEM's Flask service wrote
// to the shared volume for this run. The same series is already in the
// response, so the file is redundant.
// Failures are logged but never fail the request.
func deleteSourceTimeseries(cfg *config.Config, timeseriesFile string) {
	if !strings.HasPrefix(timeseriesFile, buemFilesPrefix) {
		return
	}
	fname := timeseriesFile[len(buemFilesPrefix):]
	if fname == "" || cfg.BuemResultsDir == "" {
		return
	}

	fullPath := filepath.Join(cfg.BuemResultsDir, fname)
	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		log.Printf("buem-gateway | warning: failed to delete source timeseries file %s: %v", fullPath, err)
	}
}
