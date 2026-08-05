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

func versionString() string {
	return fmt.Sprintf("%s (commit %s, built %s)", version.Version, version.Commit, version.Date)
}

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
	flag.Parse()
	if *showVersion {
		fmt.Println(versionString())
		os.Exit(0)
	}

	cfg := config.Get()
	srv := server.New(cfg)
	log.Printf("buem-gateway listening on %s (upstream BuEM: %s)", cfg.Addr(), cfg.BuEM.URL(""))
	log.Fatal(srv.ListenAndServe())
}
