package server

import (
	"testing"

	"github.com/enerplanet/buem-gateway/internal/config"
)

func TestNew_buildsServerWithConfiguredAddr(t *testing.T) {
	cfg := &config.Config{
		ServerHost: "127.0.0.1",
		ServerPort: 9999,
		BuEM:       config.UpstreamService{Host: "buem-model", Port: 5000},
	}

	srv := New(cfg)

	if srv == nil {
		t.Fatal("New() returned nil")
	}
	if srv.Addr != "127.0.0.1:9999" {
		t.Errorf("srv.Addr = %q, want %q", srv.Addr, "127.0.0.1:9999")
	}
	if srv.Handler == nil {
		t.Error("srv.Handler is nil, want a routed handler")
	}
}
