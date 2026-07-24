// Package server assembles buem-gateway's HTTP server from its config,
// connector, handlers, and router.
package server

import (
	"net/http"

	"github.com/enerplanet/buem-gateway/internal/api/handler"
	"github.com/enerplanet/buem-gateway/internal/api/router"
	"github.com/enerplanet/buem-gateway/internal/buem"
	"github.com/enerplanet/buem-gateway/internal/config"
)

// New builds buem-gateway's HTTP server, ready to run with ListenAndServe.
func New(cfg *config.Config) *http.Server {
	connector := buem.NewConnector(cfg)
	h := handler.New(connector)

	return &http.Server{
		Addr:    cfg.Addr(),
		Handler: router.New(h),
	}
}
