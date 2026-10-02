package buem

import (
	"errors"
	"strings"
	"testing"
)

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
