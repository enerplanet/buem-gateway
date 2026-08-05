package main

import (
	"strings"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/version"
)

func TestVersionString(t *testing.T) {
	got := versionString()
	for _, want := range []string{version.Version, version.Commit, version.Date} {
		if !strings.Contains(got, want) {
			t.Errorf("versionString() = %q, want it to contain %q", got, want)
		}
	}
}
