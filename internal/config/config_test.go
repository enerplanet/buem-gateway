package config

import "testing"

func TestUpstreamService_URL(t *testing.T) {
	s := UpstreamService{Host: "buem-service", Port: 5000}
	got := s.URL("/api/process")
	want := "http://buem-service:5000/api/process"
	if got != want {
		t.Errorf("URL() = %q, want %q", got, want)
	}
}

func TestConfig_Addr(t *testing.T) {
	c := &Config{ServerHost: "0.0.0.0", ServerPort: 8080}
	got := c.Addr()
	want := "0.0.0.0:8080"
	if got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

func TestEnvString_usesEnvValueWhenSet(t *testing.T) {
	t.Setenv("TEST_ENV_STRING", "custom-value")
	if got := envString("TEST_ENV_STRING", "fallback"); got != "custom-value" {
		t.Errorf("envString() = %q, want %q", got, "custom-value")
	}
}

func TestEnvString_usesFallbackWhenUnset(t *testing.T) {
	if got := envString("TEST_ENV_STRING_UNSET", "fallback"); got != "fallback" {
		t.Errorf("envString() = %q, want %q", got, "fallback")
	}
}

func TestEnvInt_usesEnvValueWhenSet(t *testing.T) {
	t.Setenv("TEST_ENV_INT", "42")
	if got := envInt("TEST_ENV_INT", 7); got != 42 {
		t.Errorf("envInt() = %d, want %d", got, 42)
	}
}

func TestEnvInt_usesFallbackWhenUnset(t *testing.T) {
	if got := envInt("TEST_ENV_INT_UNSET", 7); got != 7 {
		t.Errorf("envInt() = %d, want %d", got, 7)
	}
}

func TestEnvInt_usesFallbackWhenUnparseable(t *testing.T) {
	t.Setenv("TEST_ENV_INT_BAD", "not-a-number")
	if got := envInt("TEST_ENV_INT_BAD", 7); got != 7 {
		t.Errorf("envInt() = %d, want fallback %d", got, 7)
	}
}

func TestLoad_defaultsWhenEnvUnset(t *testing.T) {
	for _, key := range []string{
		"SERVER_HOST", "SERVER_PORT", "MAX_CONCURRENT_SIMS", "REQUEST_TIMEOUT",
		"RETRY_ATTEMPTS", "RETRY_BASE_DELAY", "BUEM_SERVICE_HOST", "BUEM_SERVICE_PORT",
		"IGNIS_SERVICE_HOST", "IGNIS_SERVICE_PORT", "BUEM_DATA_DIR", "BUEM_RESULTS_DIR",
	} {
		t.Setenv(key, "")
	}

	c := load()

	if c.ServerHost != "0.0.0.0" || c.ServerPort != 8080 {
		t.Errorf("server addr = %s:%d, want 0.0.0.0:8080", c.ServerHost, c.ServerPort)
	}
	if c.MaxConcurrentSims != 4 {
		t.Errorf("MaxConcurrentSims = %d, want 4", c.MaxConcurrentSims)
	}
	if c.BuEM.Host != "buem-service" || c.BuEM.Port != 5000 {
		t.Errorf("BuEM = %s:%d, want buem-service:5000", c.BuEM.Host, c.BuEM.Port)
	}
	if c.Ignis.Host != "ignis-app" || c.Ignis.Port != 8080 {
		t.Errorf("Ignis = %s:%d, want ignis-app:8080", c.Ignis.Host, c.Ignis.Port)
	}
	if c.BuemDataDir != "data" || c.BuemResultsDir != "results" {
		t.Errorf("data dirs = %s, %s, want data, results", c.BuemDataDir, c.BuemResultsDir)
	}
}

func TestLoad_readsEnvOverrides(t *testing.T) {
	t.Setenv("SERVER_HOST", "127.0.0.1")
	t.Setenv("SERVER_PORT", "9090")
	t.Setenv("BUEM_SERVICE_HOST", "custom-buem")
	t.Setenv("BUEM_SERVICE_PORT", "6000")

	c := load()

	if c.ServerHost != "127.0.0.1" || c.ServerPort != 9090 {
		t.Errorf("server addr = %s:%d, want 127.0.0.1:9090", c.ServerHost, c.ServerPort)
	}
	if c.BuEM.Host != "custom-buem" || c.BuEM.Port != 6000 {
		t.Errorf("BuEM = %s:%d, want custom-buem:6000", c.BuEM.Host, c.BuEM.Port)
	}
}
