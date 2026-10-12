package buem

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// hourlyValues returns a JSON array of n hourly values of 0.5.
func hourlyValues(n int) string {
	return "[" + strings.TrimSuffix(strings.Repeat("0.5,", n), ",") + "]"
}

// withElectricityProfile is occupancyOnlyInput carrying profile as
// buem.inputs.electricity_load_profile.
func withElectricityProfile(profile string) BuildingInput {
	in := occupancyOnlyInput("b1")
	in.BUEM = json.RawMessage(`{"building":{"building_type":"MFH","country":"NL","A_ref":{"value":900,"unit":"m2"},"residential_units":12},
		"outputs":{"heating":"none","cooling":"none","electricity":"series"},
		"inputs":{"electricity_load_profile":` + profile + `}}`)
	return in
}

func TestTaskFromBuilding_AcceptsElectricityProfile(t *testing.T) {
	for name, tc := range map[string]struct{ profile, start string }{
		"values kWh":     {`{"values":` + hourlyValues(8760) + `,"unit":"kWh"}`, "2018-01-01T00:00:00Z"},
		"values kW":      {`{"values":` + hourlyValues(8760) + `,"unit":"kW"}`, "2018-01-01T00:00:00Z"},
		"values no unit": {`{"values":` + hourlyValues(8760) + `}`, "2018-01-01T00:00:00Z"},
		"values leap":    {`{"values":` + hourlyValues(8784) + `}`, "2020-01-01T00:00:00Z"},
		"path":           {`{"path":"profiles/b1.csv","unit":"Wh"}`, "2018-01-01T00:00:00Z"},
	} {
		if _, err := TaskFromBuilding(withElectricityProfile(tc.profile), tc.start, "", 60, ""); err != nil {
			t.Errorf("%s: TaskFromBuilding() = %v, want accepted", name, err)
		}
	}
}

func TestTaskFromBuilding_RejectsInvalidElectricityProfile(t *testing.T) {
	for name, profile := range map[string]string{
		"both":         `{"values":` + hourlyValues(8760) + `,"path":"profiles/b1.csv"}`,
		"neither":      `{"unit":"kWh"}`,
		"empty":        `{"values":[]}`,
		"short":        `{"values":` + hourlyValues(8759) + `}`,
		"leap length":  `{"values":` + hourlyValues(8784) + `}`,
		"negative":     `{"values":[-1,` + strings.TrimPrefix(hourlyValues(8759), "[") + `}`,
		"null value":   `{"values":[null,` + strings.TrimPrefix(hourlyValues(8759), "[") + `}`,
		"string value": `{"values":["1",` + strings.TrimPrefix(hourlyValues(8759), "[") + `}`,
		"Wh inline":    `{"values":` + hourlyValues(8760) + `,"unit":"Wh"}`,
		"not object":   `[1,2,3]`,
	} {
		_, err := TaskFromBuilding(withElectricityProfile(profile), "2018-01-01T00:00:00Z", "", 60, "")
		if !errors.Is(err, ErrInvalidRequest) || !strings.Contains(err.Error(), "electricity_load_profile") {
			t.Errorf("%s: TaskFromBuilding() = %v, want an ErrInvalidRequest naming electricity_load_profile", name, err)
		}
	}
}
