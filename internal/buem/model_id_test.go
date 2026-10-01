package buem

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enerplanet/buem-gateway/internal/config"
)

// TestWriteCSVs_RefusesModelIDOutsideDataDir covers the guard at the write
// itself, independent of the request preflight: a model_id that resolves
// outside BUEM_DATA_DIR writes nothing.
func TestWriteCSVs_RefusesModelIDOutsideDataDir(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	cfg := &config.Config{BuemDataDir: dataDir}
	block := &ResponseBlock{ThermalLoadProfile: ThermalLoadProfile{
		Timeseries: &Timeseries{Heating: []float64{0.1, 0.2}},
	}}
	task := Task{NodeID: "b1", Lat: 48.5, Lon: 12.5, Year: 2018, ModelID: "../escape"}

	if _, _, err := writeCSVsAndAnnotate(cfg, block, task, false); err == nil {
		t.Fatal("writeCSVsAndAnnotate() = nil error, want a refusal for a model_id outside BUEM_DATA_DIR")
	}
	if _, err := os.Stat(filepath.Join(root, "escape")); !os.IsNotExist(err) {
		t.Errorf("directory outside BUEM_DATA_DIR exists (stat err = %v), want nothing written", err)
	}
}

func TestValidateModelID(t *testing.T) {
	for _, id := range []string{"", "demo", "demo-model_001", "run.2026-10-01", "A1"} {
		if err := ValidateModelID(id); err != nil {
			t.Errorf("ValidateModelID(%q) = %v, want nil", id, err)
		}
	}
	for _, id := range []string{".", "..", "../escape", "a/b", "/etc", `a\b`, "has space", "model\x00", "ümlaut"} {
		err := ValidateModelID(id)
		if !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("ValidateModelID(%q) = %v, want it to wrap ErrInvalidRequest", id, err)
		}
	}
}

func TestTaskFromBuilding_RejectsUnsafeModelID(t *testing.T) {
	_, err := TaskFromBuilding(testBuildingInput("b1"), "2018-01-01T00:00:00Z", "2018-12-31T23:00:00Z", 60, "../escape")
	if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "model_id") {
		t.Fatalf("TaskFromBuilding() error = %v, want an ErrInvalidRequest naming model_id", err)
	}
}
