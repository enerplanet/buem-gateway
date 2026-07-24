package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})
}

func TestCORS_allowedOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173,https://app.example.com")
	h := CORS(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "http://localhost:5173")
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "pong" {
		t.Errorf("body = %q, want %q", w.Body.String(), "pong")
	}
}

func TestCORS_disallowedOrigin(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com")
	h := CORS(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("Origin", "https://evil.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty for disallowed origin", got)
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (request still proceeds, just without CORS headers)", w.Code, http.StatusOK)
	}
}

func TestCORS_noOriginUnsetEnv(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "")
	h := CORS(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func TestCORS_optionsPreflight_isAborted(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com")
	called := false
	h := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	req.Header.Set("Origin", "https://app.example.com")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
	if called {
		t.Error("expected the wrapped handler not to run for a preflight request")
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, "https://app.example.com")
	}
}

func TestCORS_noOriginHeader_passesThroughWithoutCORSHeaders(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "https://app.example.com")
	h := CORS(newTestHandler())

	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Access-Control-Allow-Origin = %q, want empty when no Origin header is sent", got)
	}
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
