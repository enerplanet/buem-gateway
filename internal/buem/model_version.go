package buem

import (
	"fmt"

	"github.com/enerplanet/buem-gateway/internal/httpclient"
)

const apiHealthPath = "/api/health"

// ModelVersion is the release and commit buem-model reports in its health.
// Both are empty for a buem-model release that does not report them.
type ModelVersion struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

// healthClient has its own short timeout and no retries: it serves
// buem-gateway's liveness check, which must answer even when buem-model hangs.
var healthClient = httpclient.New(2, 0, 0)

// ModelVersion asks buem-model's health endpoint for its version.
func (c *Connector) ModelVersion() (ModelVersion, error) {
	var v ModelVersion
	if err := healthClient.GetJSONAndDecode(c.cfg.BuEM.URL(apiHealthPath), &v); err != nil {
		return ModelVersion{}, fmt.Errorf("buem-model health: %w", err)
	}
	return v, nil
}
