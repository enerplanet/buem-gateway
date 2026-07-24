package tabula

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/enerplanet/buem-gateway/internal/config"
	"github.com/enerplanet/buem-gateway/internal/httpclient"
)

// refurbishmentLabel ignis uses for the non-refurbished, as-built variant —
// the correct baseline for a fallback (no refurbishment was requested).
const existingStateLabel = "Existing state"

// Client resolves TABULA building defaults from ignis.
type Client struct {
	http    *httpclient.Client
	baseURL string
}

// New creates a Client bound to cfg.Ignis.
func New(cfg *config.Config) *Client {
	return &Client{
		http:    httpclient.New(cfg.RequestTimeout, cfg.RetryAttempts, cfg.RetryBaseDelay),
		baseURL: cfg.Ignis.URL(""),
	}
}

// Resolve fetches the "existing state" TABULA variant matching country,
// buildingType, and constructionPeriod (a TABULA class code, e.g. "01" — not
// a year range; see the package doc) and maps it into a Fallback ready to
// merge into a request's building block. Returns an error if ignis is
// unreachable or no matching variant exists.
func (c *Client) Resolve(country, buildingType, constructionPeriod string) (*Fallback, error) {
	code, err := c.matchVariant(country, buildingType, constructionPeriod)
	if err != nil {
		return nil, err
	}
	data, err := c.fetchData(code)
	if err != nil {
		return nil, err
	}
	fallback := buildFallback(data)
	return &fallback, nil
}

func (c *Client) matchVariant(country, buildingType, constructionPeriod string) (string, error) {
	u := fmt.Sprintf("%s/api/v1/variants/%s/match?type=%s&period=%s",
		c.baseURL,
		url.PathEscape(strings.ToUpper(country)),
		url.QueryEscape(strings.ToUpper(buildingType)),
		url.QueryEscape(constructionPeriod),
	)
	var resp matchResponse
	if err := c.http.GetJSONAndDecode(u, &resp); err != nil {
		return "", fmt.Errorf("match TABULA variant (country=%s type=%s period=%s): %w", country, buildingType, constructionPeriod, err)
	}
	for _, m := range resp.Data {
		if m.Label == existingStateLabel {
			return m.Code, nil
		}
	}
	if len(resp.Data) > 0 {
		return resp.Data[0].Code, nil
	}
	return "", fmt.Errorf("no TABULA variant matches country=%s type=%s period=%s", country, buildingType, constructionPeriod)
}

func (c *Client) fetchData(code string) (*dataResponse, error) {
	u := fmt.Sprintf("%s/api/v1/data/%s", c.baseURL, url.PathEscape(code))
	var resp dataResponse
	if err := c.http.GetJSONAndDecode(u, &resp); err != nil {
		return nil, fmt.Errorf("fetch TABULA data for %s: %w", code, err)
	}
	return &resp, nil
}
