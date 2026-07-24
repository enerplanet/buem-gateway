package httpclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type echoResult struct {
	Value string `json:"value"`
}

func TestPostJSONAndDecode_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(echoResult{Value: body["name"]})
	}))
	defer server.Close()

	c := New(5, 0, 10)
	var result echoResult
	err := c.PostJSONAndDecode(server.URL, map[string]string{"name": "hello"}, &result)

	if err != nil {
		t.Fatalf("PostJSONAndDecode() error = %v", err)
	}
	if result.Value != "hello" {
		t.Errorf("result.Value = %q, want %q", result.Value, "hello")
	}
}

func TestGetJSONAndDecode_success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(echoResult{Value: "world"})
	}))
	defer server.Close()

	c := New(5, 0, 10)
	var result echoResult
	err := c.GetJSONAndDecode(server.URL, &result)

	if err != nil {
		t.Fatalf("GetJSONAndDecode() error = %v", err)
	}
	if result.Value != "world" {
		t.Errorf("result.Value = %q, want %q", result.Value, "world")
	}
}

func TestGetJSONAndDecode_nilResultSkipsDecode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json, but nobody asked for it"))
	}))
	defer server.Close()

	c := New(5, 0, 10)
	if err := c.GetJSONAndDecode(server.URL, nil); err != nil {
		t.Fatalf("GetJSONAndDecode() error = %v, want nil (result is nil, decode should be skipped)", err)
	}
}

func Test4xxResponse_failsWithoutRetry(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	c := New(5, 3, 1)
	var result echoResult
	err := c.GetJSONAndDecode(server.URL, &result)

	if err == nil {
		t.Fatal("expected an error for a 400 response")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error = %v, want it to mention status 400", err)
	}
	if got := atomic.LoadInt32(&callCount); got != 1 {
		t.Errorf("upstream called %d times, want 1 (4xx should not be retried)", got)
	}
}

func Test5xxResponse_retriesThenSucceeds(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&callCount, 1)
		if n < 3 {
			http.Error(w, "server error", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(echoResult{Value: "recovered"})
	}))
	defer server.Close()

	c := New(5, 3, 1) // 1ms base delay so the test stays fast
	var result echoResult
	err := c.GetJSONAndDecode(server.URL, &result)

	if err != nil {
		t.Fatalf("GetJSONAndDecode() error = %v, want success after retries", err)
	}
	if result.Value != "recovered" {
		t.Errorf("result.Value = %q, want %q", result.Value, "recovered")
	}
	if got := atomic.LoadInt32(&callCount); got != 3 {
		t.Errorf("upstream called %d times, want 3", got)
	}
}

func Test5xxResponse_exhaustsRetriesAndFails(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		http.Error(w, "server error", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	c := New(5, 2, 1)
	var result echoResult
	err := c.GetJSONAndDecode(server.URL, &result)

	if err == nil {
		t.Fatal("expected an error once retries are exhausted")
	}
	if got := atomic.LoadInt32(&callCount); got != 3 { // 1 initial + 2 retries
		t.Errorf("upstream called %d times, want 3 (1 initial + 2 retries)", got)
	}
}

func TestNetworkError_retriesThenFails(t *testing.T) {
	c := New(1, 2, 1)
	var result echoResult
	// Nothing listens on this port — every attempt is a connection error.
	err := c.GetJSONAndDecode("http://127.0.0.1:1", &result)

	if err == nil {
		t.Fatal("expected an error for an unreachable host")
	}
	if !strings.Contains(err.Error(), "all 3 attempts") {
		t.Errorf("error = %v, want it to report all attempts exhausted", err)
	}
}

func TestDecodeJSONResponse_malformedBodyReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	c := New(5, 0, 10)
	var result echoResult
	err := c.GetJSONAndDecode(server.URL, &result)

	if err == nil {
		t.Fatal("expected a decode error for a non-JSON body")
	}
}
