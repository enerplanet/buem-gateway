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

// parseVersionFlag reports whether args request the version string, so the
// flag-handling decision can be tested without starting a server or exiting.
func parseVersionFlag(args []string) bool {
	fs := flag.NewFlagSet("buem-gateway", flag.ExitOnError)
	showVersion := fs.Bool("version", false, "print version and exit")
	fs.BoolVar(showVersion, "v", false, "print version and exit (shorthand)")
	fs.Parse(args)
	return *showVersion
}

func main() {
	if parseVersionFlag(os.Args[1:]) {
		fmt.Println(versionString())
		os.Exit(0)
	}

	cfg := config.Get()
	srv := server.New(cfg)
	log.Printf("buem-gateway listening on %s (upstream BuEM: %s)", cfg.Addr(), cfg.BuEM.URL(""))
	log.Fatal(srv.ListenAndServe())
}
