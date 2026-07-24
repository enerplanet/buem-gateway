// Command app runs buem-gateway's HTTP server.
package main

import (
	"log"

	"github.com/enerplanet/buem-gateway/internal/api/server"
	"github.com/enerplanet/buem-gateway/internal/config"
)

func main() {
	cfg := config.Get()
	srv := server.New(cfg)
	log.Printf("buem-gateway listening on %s (upstream BuEM: %s)", cfg.Addr(), cfg.BuEM.URL(""))
	log.Fatal(srv.ListenAndServe())
}
