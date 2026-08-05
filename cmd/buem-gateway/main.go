// Command buem-gateway runs buem-gateway's HTTP server.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/enerplanet/buem-gateway/internal/api/server"
	"github.com/enerplanet/buem-gateway/internal/config"
	"github.com/enerplanet/buem-gateway/internal/version"
)

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
	flag.Parse()
	if *showVersion {
		fmt.Printf("%s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
		os.Exit(0)
	}

	cfg := config.Get()
	srv := server.New(cfg)
	log.Printf("buem-gateway listening on %s (upstream BuEM: %s)", cfg.Addr(), cfg.BuEM.URL(""))
	log.Fatal(srv.ListenAndServe())
}
