package buem

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
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
	CSVWriteDuration       time.Duration
	ModelProcessingSeconds float64
}

// RunFeature sends one Task to the upstream BuEM service, writes its
// heating/cooling/electricity CSVs to the shared data directory, and returns
// the enriched buem block (as raw JSON, with file paths injected and the
// inline timeseries stripped out).
func RunFeature(client *httpclient.Client, cfg *config.Config, task Task) ([]byte, RunMetrics, error) {
	wallStart := time.Now()

	block, err := callUpstream(client, cfg, task)
	if err != nil {
		return nil, RunMetrics{}, err
	}
	modelSeconds := block.ModelMetadata.ProcessingTime.Value

	enriched, csvWriteDuration, err := writeCSVsAndAnnotate(cfg, block, task)
	if err != nil {
		return nil, RunMetrics{}, err
	}

	metrics := RunMetrics{
		WallDuration:           time.Since(wallStart),
		CSVWriteDuration:       csvWriteDuration,
		ModelProcessingSeconds: modelSeconds,
	}
	log.Printf("buem-gateway | node=%s lat=%.6f lon=%.6f year=%d wall=%s model=%.3fs csv=%s",
		task.NodeID, task.Lat, task.Lon, task.Year,
		metrics.WallDuration.Round(time.Millisecond), metrics.ModelProcessingSeconds, metrics.CSVWriteDuration.Round(time.Millisecond))

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

// writeCSVsAndAnnotate writes heating, cooling, and electricity CSVs to
// {BuemDataDir}/{modelID}/, injects the file paths into the buem block, and
// marshals it. The timeseries arrays are removed once written to CSV.
func writeCSVsAndAnnotate(cfg *config.Config, block *ResponseBlock, task Task) ([]byte, time.Duration, error) {
	ts := block.ThermalLoadProfile.Timeseries
	if ts == nil {
		return nil, 0, fmt.Errorf("BuEM response missing timeseries (include_timeseries=true was requested)")
	}
	if len(ts.Heating) == 0 {
		return nil, 0, fmt.Errorf("heating timeseries is empty")
	}

	resultsDir := cfg.BuemDataDir
	if task.ModelID != "" {
		resultsDir = filepath.Join(cfg.BuemDataDir, task.ModelID)
	}
	suffix := fmt.Sprintf("%.6f_%.6f_%s", task.Lat, task.Lon, strconv.Itoa(task.Year))

	writeStart := time.Now()
	if err := writeLoadCSV(resultsDir, "heating", suffix, ts.Heating, &block.ThermalLoadProfile.HeatingFile); err != nil {
		return nil, 0, err
	}
	if len(ts.Cooling) > 0 {
		if err := writeLoadCSV(resultsDir, "cooling", suffix, ts.Cooling, &block.ThermalLoadProfile.CoolingFile); err != nil {
			return nil, 0, err
		}
	}
	if len(ts.Electricity) > 0 {
		if err := writeLoadCSV(resultsDir, "electricity", suffix, ts.Electricity, &block.ThermalLoadProfile.ElectricityFile); err != nil {
			return nil, 0, err
		}
	}
	csvWriteDuration := time.Since(writeStart)

	deleteSourceTimeseries(cfg, block.ThermalLoadProfile.TimeseriesFile)
	block.ThermalLoadProfile.Timeseries = nil

	enriched, err := json.Marshal(block)
	if err != nil {
		return nil, 0, err
	}
	return enriched, csvWriteDuration, nil
}

// writeLoadCSV writes one load-type CSV (e.g. "heating") and records its path
// into destPath for the response block.
func writeLoadCSV(dir, loadType, suffix string, values []float64, destPath *string) error {
	path := filepath.Join(dir, fmt.Sprintf("%s_%s.csv", loadType, suffix))
	if err := writeProfileCSV(path, values); err != nil {
		return fmt.Errorf("write %s CSV: %w", loadType, err)
	}
	*destPath = path
	return nil
}

// writeProfileCSV writes float64 values to a CSV file with a single "demand"
// header column. Parent directories are created if they do not exist.
func writeProfileCSV(path string, values []float64) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var sb strings.Builder
	sb.WriteString("demand\n")
	for _, v := range values {
		sb.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		sb.WriteByte('\n')
	}
	_, err = f.WriteString(sb.String())
	return err
}

// deleteSourceTimeseries removes the .json.gz file BuEM's Flask service wrote
// to the shared volume for this run — redundant once the CSV is written.
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
