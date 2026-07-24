// Package buem implements the connector between EnerPlanET's topology-JSON
// request format and the upstream BuEM Flask thermal model API: it extracts
// buildings from a topology, fans requests out to BuEM concurrently, writes
// the resulting load profiles to CSV, and merges the results back into the
// topology.
package buem

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/enerplanet/buem-gateway/internal/config"
	"github.com/enerplanet/buem-gateway/internal/httpclient"
	"github.com/enerplanet/buem-gateway/internal/tabula"
)

// Connector runs BuEM requests for a topology, capping concurrency at
// cfg.MaxConcurrentSims so the upstream Flask service is never overloaded.
type Connector struct {
	cfg    *config.Config
	client *httpclient.Client
	tabula envelopeResolver
	sem    chan struct{}
}

// NewConnector creates a Connector bound to the given config.
func NewConnector(cfg *config.Config) *Connector {
	return &Connector{
		cfg:    cfg,
		client: httpclient.New(cfg.RequestTimeout, cfg.RetryAttempts, cfg.RetryBaseDelay),
		tabula: tabula.New(cfg),
		sem:    make(chan struct{}, maxConcurrent(cfg)),
	}
}

func maxConcurrent(cfg *config.Config) int {
	if cfg.MaxConcurrentSims <= 0 {
		return 4
	}
	return cfg.MaxConcurrentSims
}

// Run extracts buildings from rawTopology, runs them through BuEM
// concurrently, and returns the topology with each building's buem block
// enriched with thermal_load_profile. Buildings with no buem block pass
// through unchanged; a building that fails is left as it was, and its error
// is logged.
func (c *Connector) Run(rawTopology json.RawMessage, startDate, endDate, modelID string, resolution int) (json.RawMessage, error) {
	tasks, err := ExtractTasks(rawTopology, startDate, endDate, resolution, modelID, c.tabula)
	if err != nil {
		return nil, fmt.Errorf("parse topology: %w", err)
	}
	if len(tasks) == 0 {
		return rawTopology, nil
	}

	log.Printf("buem-gateway | model=%s found %d buildings with buem block", modelID, len(tasks))
	requestStart := time.Now()

	results := c.runTasks(tasks)
	logBatchSummary(results, time.Since(requestStart))

	return mergeIntoTopology(rawTopology, results)
}

// runTasks runs every task concurrently, bounded by c.sem, and collects each
// outcome keyed by node ID.
func (c *Connector) runTasks(tasks []Task) map[string]outcome {
	ch := make(chan outcome, len(tasks))
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			c.sem <- struct{}{}
			defer func() { <-c.sem }()
			ch <- c.runOne(t)
		}(task)
	}
	wg.Wait()
	close(ch)

	results := make(map[string]outcome, len(tasks))
	for o := range ch {
		results[o.nodeID] = o
	}
	return results
}

func (c *Connector) runOne(task Task) outcome {
	block, metrics, err := RunFeature(c.client, c.cfg, task)
	if err != nil {
		log.Printf("buem-gateway | node=%s error: %s", task.NodeID, err)
		return outcome{nodeID: task.NodeID, errMsg: err.Error()}
	}
	return outcome{nodeID: task.NodeID, buemBlock: block, metrics: metrics}
}

func logBatchSummary(results map[string]outcome, requestDuration time.Duration) {
	var successful, failed int
	var totalWall, totalCSV time.Duration
	var totalModelSeconds float64

	for _, o := range results {
		if o.errMsg != "" {
			failed++
			continue
		}
		successful++
		totalWall += o.metrics.WallDuration
		totalCSV += o.metrics.CSVWriteDuration
		totalModelSeconds += o.metrics.ModelProcessingSeconds
	}

	avgWall, avgCSV, avgModel := time.Duration(0), time.Duration(0), 0.0
	throughput := 0.0
	if successful > 0 {
		avgWall = totalWall / time.Duration(successful)
		avgCSV = totalCSV / time.Duration(successful)
		avgModel = totalModelSeconds / float64(successful)
	}
	if requestDuration > 0 {
		throughput = float64(successful) / requestDuration.Seconds()
	}

	log.Printf(
		"buem-gateway | processed=%d failed=%d total=%s avg_wall=%s avg_model=%.3fs avg_csv=%s throughput=%.2f buildings/s",
		successful, failed, requestDuration.Round(time.Millisecond),
		avgWall.Round(time.Millisecond), avgModel, avgCSV.Round(time.Millisecond), throughput,
	)
}
