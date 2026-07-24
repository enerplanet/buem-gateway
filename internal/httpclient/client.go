// Package httpclient provides a small HTTP client with retry/backoff, used to
// call the upstream BuEM Flask API.
package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"
)

// Client is an HTTP client that retries failed or 5xx requests with
// exponential backoff.
type Client struct {
	http          *http.Client
	retryAttempts int
	baseDelay     time.Duration
}

// New creates a Client. timeoutSeconds of 0 means no request timeout.
func New(timeoutSeconds, retryAttempts, baseDelayMs int) *Client {
	return &Client{
		http:          &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
		retryAttempts: retryAttempts,
		baseDelay:     time.Duration(baseDelayMs) * time.Millisecond,
	}
}

// PostJSONAndDecode POSTs body as JSON to url and decodes the JSON response
// into result. Retries on network errors and 5xx responses.
func (c *Client) PostJSONAndDecode(url string, body, result interface{}) error {
	requestBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}
	resp, err := c.doWithRetry(func() (*http.Response, error) {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(requestBody))
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		return c.http.Do(req)
	}, url)
	if err != nil {
		return err
	}
	return decodeJSONResponse(url, resp, result)
}

// GetJSONAndDecode GETs url and decodes the JSON response into result.
// Retries on network errors and 5xx responses, same as PostJSONAndDecode.
func (c *Client) GetJSONAndDecode(url string, result interface{}) error {
	resp, err := c.doWithRetry(func() (*http.Response, error) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, fmt.Errorf("create request: %w", err)
		}
		return c.http.Do(req)
	}, url)
	if err != nil {
		return err
	}
	return decodeJSONResponse(url, resp, result)
}

func decodeJSONResponse(url string, resp *http.Response, result interface{}) error {
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request to %s failed with status %d: %s", url, resp.StatusCode, respBody)
	}
	if result == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode response from %s: %w", url, err)
	}
	return nil
}

// doWithRetry runs makeRequest, retrying network errors and 5xx responses
// with exponential backoff, up to c.retryAttempts additional times.
func (c *Client) doWithRetry(makeRequest func() (*http.Response, error), url string) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retryAttempts; attempt++ {
		c.waitBeforeRetry(attempt)

		resp, err := makeRequest()
		if err != nil {
			lastErr = err
			log.Printf("buem-gateway: request to %s failed (attempt %d): %v", url, attempt+1, err)
			continue
		}
		if resp.StatusCode < 500 {
			return resp, nil
		}
		resp.Body.Close()
		lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
		log.Printf("buem-gateway: request to %s returned %d (attempt %d)", url, resp.StatusCode, attempt+1)
	}
	return nil, fmt.Errorf("all %d attempts to %s failed: %w", c.retryAttempts+1, url, lastErr)
}

func (c *Client) waitBeforeRetry(attempt int) {
	if attempt == 0 {
		return
	}
	delay := c.baseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	log.Printf("buem-gateway: retry attempt %d/%d after %v", attempt, c.retryAttempts, delay)
	time.Sleep(delay)
}
