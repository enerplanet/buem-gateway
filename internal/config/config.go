// Package config loads buem-gateway's runtime configuration from environment
// variables, with defaults suitable for local development.
package config

import (
	"fmt"
	"os"
	"strconv"
	"sync"
)

// UpstreamService holds the host/port of the BuEM Flask API this gateway calls.
type UpstreamService struct {
	Host string
	Port int
}

// URL builds the full upstream URL for the given path.
func (s UpstreamService) URL(path string) string {
	return fmt.Sprintf("http://%s:%d%s", s.Host, s.Port, path)
}

// Config holds all buem-gateway configuration.
type Config struct {
	ServerHost string
	ServerPort int

	// MaxConcurrentSims caps how many buildings are sent to BuEM at once.
	MaxConcurrentSims int
	RequestTimeout    int // seconds; 0 = no timeout
	RetryAttempts     int
	RetryBaseDelay    int // milliseconds

	BuEM UpstreamService

	// BuemDataDir is where heating/cooling/electricity CSVs are written,
	// under {BuemDataDir}/{model_id}/. BuemResultsDir is where BuEM's own
	// Flask service writes its intermediate .json.gz timeseries file,
	// deleted once the CSV has been extracted from it.
	BuemDataDir    string
	BuemResultsDir string
}

var (
	instance *Config
	once     sync.Once
)

// Get returns the process-wide Config, loading it from the environment on
// first call.
func Get() *Config {
	once.Do(func() {
		instance = load()
	})
	return instance
}

func load() *Config {
	return &Config{
		ServerHost: envString("SERVER_HOST", "0.0.0.0"),
		ServerPort: envInt("SERVER_PORT", 8080),

		MaxConcurrentSims: envInt("MAX_CONCURRENT_SIMS", 4),
		RequestTimeout:    envInt("REQUEST_TIMEOUT", 0),
		RetryAttempts:     envInt("RETRY_ATTEMPTS", 0),
		RetryBaseDelay:    envInt("RETRY_BASE_DELAY", 1000),

		BuEM: UpstreamService{
			Host: envString("BUEM_SERVICE_HOST", "buem-model"),
			Port: envInt("BUEM_SERVICE_PORT", 5000),
		},

		BuemDataDir:    envString("BUEM_DATA_DIR", "data"),
		BuemResultsDir: envString("BUEM_RESULTS_DIR", "results"),
	}
}

// Addr returns the address this gateway's HTTP server should listen on.
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.ServerHost, c.ServerPort)
}

func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
