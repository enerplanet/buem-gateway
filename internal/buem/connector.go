// Package buem implements the connector to the upstream BuEM Flask thermal
// model API: it fans a building or a list of buildings out to BuEM
// concurrently and returns each building's load profiles. Callers own
// their own topology/grid concept, if they have one — buem-gateway only
// ever sees individual buildings, keyed by whatever id the caller gave them.
package buem

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/enerplanet/buem-gateway/internal/config"
	"github.com/enerplanet/buem-gateway/internal/httpclient"
)

// Connector runs BuEM requests for a topology, capping concurrency at
// cfg.MaxConcurrentSims so the upstream Flask service is never overloaded.
type Connector struct {
	cfg    *config.Config
	client *httpclient.Client
	sem    chan struct{}
}

// NewConnector creates a Connector bound to the given config.
func NewConnector(cfg *config.Config) *Connector {
	return &Connector{
		cfg:    cfg,
		client: httpclient.New(cfg.RequestTimeout, cfg.RetryAttempts, cfg.RetryBaseDelay),
		sem:    make(chan struct{}, maxConcurrent(cfg)),
	}
}

func maxConcurrent(cfg *config.Config) int {
	if cfg.MaxConcurrentSims <= 0 {
		return 4
	}
	return cfg.MaxConcurrentSims
}

// BuildingResult is one building's outcome from RunBatch — either BUEM (its
// enriched buem block) or Error (why it has no result), never both. Results
// are independent: one building's error never affects another's result.
type BuildingResult struct {
	ID    string
	BUEM  json.RawMessage
	Error string
}

// RunBatch runs BuEM for each of inputs concurrently, capped at
// cfg.MaxConcurrentSims, and returns one BuildingResult per input in the
// same order. A building with no complete buem block never reaches BuEM at
// all (see TaskFromBuilding); one that reaches BuEM and fails is reported
// the same way — both land in that building's own Error, not a request-wide
// failure.
//
// keepTimeseries carries the hourly values back in each result instead of
// only the summary figures. It costs roughly 300 KB of JSON per building for
// a year of hourly values.
func (c *Connector) RunBatch(inputs []BuildingInput, startDate, endDate, modelID string, resolution int, keepTimeseries bool) []BuildingResult {
	// Results are matched to inputs by position, never by id: ids come from
	// the caller and are not guaranteed unique.
	results := make([]BuildingResult, len(inputs))
	tasks := make([]Task, 0, len(inputs))
	inputIndex := make([]int, 0, len(inputs))
	for i, in := range inputs {
		results[i].ID = in.ID
		task, err := TaskFromBuilding(in, startDate, endDate, resolution, modelID)
		if err != nil {
			results[i].Error = err.Error()
			continue
		}
		tasks = append(tasks, task)
		inputIndex = append(inputIndex, i)
	}

	log.Printf("buem-gateway | model=%s running %d/%d buildings with a complete buem block", modelID, len(tasks), len(inputs))
	requestStart := time.Now()

	outcomes := c.runTasks(tasks, keepTimeseries)
	logBatchSummary(outcomes, time.Since(requestStart))

	for j, o := range outcomes {
		i := inputIndex[j]
		if o.errMsg != "" {
			results[i].Error = o.errMsg
			continue
		}
		results[i].BUEM = o.buemBlock
	}
	return results
}

// RunSingle runs BuEM for exactly one building. A missing envelope or
// weather block is reported as the specific ErrMissingEnvelope/
// ErrMissingWeather (see TaskFromBuilding) so the caller gets the precise
// reason, not just a generic failure.
func (c *Connector) RunSingle(id string, geometry, buemRaw json.RawMessage, startDate, endDate, modelID string, resolution int) (json.RawMessage, error) {
	task, err := TaskFromBuilding(BuildingInput{ID: id, Geometry: geometry, BUEM: buemRaw}, startDate, endDate, resolution, modelID)
	if err != nil {
		return nil, err
	}

	// keepTimeseries=true: a RunSingle caller (e.g. a browser client) has no
	// access to the shared volume — it needs the values inline to do anything
	// with them. A RunBatch caller chooses per request.
	result := c.runOne(task, true)
	if result.errMsg != "" {
		return nil, fmt.Errorf("%s", result.errMsg)
	}
	return result.buemBlock, nil
}

// runTasks runs every task concurrently, bounded by c.sem, and returns one
// outcome per task in the same order as tasks.
func (c *Connector) runTasks(tasks []Task, keepTimeseries bool) []outcome {
	outcomes := make([]outcome, len(tasks))
	var wg sync.WaitGroup

	for i, task := range tasks {
		wg.Add(1)
		go func(i int, t Task) {
			defer wg.Done()
			c.sem <- struct{}{}
			defer func() { <-c.sem }()
			outcomes[i] = c.runOne(t, keepTimeseries)
		}(i, task)
	}
	wg.Wait()
	return outcomes
}

// outcome is the result of running one Task.
type outcome struct {
	buemBlock json.RawMessage
	errMsg    string
	metrics   RunMetrics
}

// runOne runs a single task against BuEM. keepTimeseries controls whether
// the response keeps its inline timeseries; see RunFeature.
func (c *Connector) runOne(task Task, keepTimeseries bool) outcome {
	block, metrics, err := RunFeature(c.client, c.cfg, task, keepTimeseries)
	if err != nil {
		log.Printf("buem-gateway | node=%s error: %s", task.NodeID, err)
		return outcome{errMsg: err.Error()}
	}
	return outcome{buemBlock: block, metrics: metrics}
}

func logBatchSummary(results []outcome, requestDuration time.Duration) {
	var successful, failed int
	var totalWall time.Duration
	var totalModelSeconds float64

	for _, o := range results {
		if o.errMsg != "" {
			failed++
			continue
		}
		successful++
		totalWall += o.metrics.WallDuration
		totalModelSeconds += o.metrics.ModelProcessingSeconds
	}

	avgWall, avgModel := time.Duration(0), 0.0
	throughput := 0.0
	if successful > 0 {
		avgWall = totalWall / time.Duration(successful)
		avgModel = totalModelSeconds / float64(successful)
	}
	if requestDuration > 0 {
		throughput = float64(successful) / requestDuration.Seconds()
	}

	log.Printf(
		"buem-gateway | processed=%d failed=%d total=%s avg_wall=%s avg_model=%.3fs throughput=%.2f buildings/s",
		successful, failed, requestDuration.Round(time.Millisecond),
		avgWall.Round(time.Millisecond), avgModel, throughput,
	)
}
