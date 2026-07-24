package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/api/handler"
)

func TestNew_routesHealthRequest(t *testing.T) {
	h := New(handler.New(nil))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNew_unknownRouteReturnsNotFound(t *testing.T) {
	h := New(handler.New(nil))

	req := httptest.NewRequest(http.MethodGet, "/does-not-exist", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestNew_appliesCORSHeaders(t *testing.T) {
	t.Setenv("ALLOWED_ORIGINS", "http://localhost:5173")
	h := New(handler.New(nil))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q — router must wrap routes in CORS middleware", got, "http://localhost:5173")
	}
}
