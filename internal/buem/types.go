package buem

import "encoding/json"

// APIVersion is the BuEM wire-format version this connector currently speaks
// to the upstream Flask API. Confirmed against a live BuEM instance: the
// v4.2.1 example request/response round-trip validates cleanly against
// schemas/v4/{request,response}_schema.json, including compute_cooling
// opt-in, model_metadata.simulations_run/electricity_source, and the
// electricity_load_profile file-path input.
const APIVersion = "v4"

// FeatureCollection is the GeoJSON request body sent to BuEM. Features are
// kept as raw JSON so the caller-supplied buem block is forwarded unchanged.
type FeatureCollection struct {
	Type     string            `json:"type"`
	Features []json.RawMessage `json:"features"`
}

// ResponseFeatureCollection is the top-level BuEM API response.
type ResponseFeatureCollection struct {
	Type     string             `json:"type"`
	Metadata CollectionMetadata `json:"metadata"`
	Features []ResponseFeature  `json:"features"`
}

// CollectionMetadata reports overall processing counts for the batch.
type CollectionMetadata struct {
	TotalFeatures      int `json:"total_features"`
	SuccessfulFeatures int `json:"successful_features"`
	FailedFeatures     int `json:"failed_features"`
}

// ResponseFeature is one building's result entry.
type ResponseFeature struct {
	Type       string             `json:"type"`
	ID         string             `json:"id"`
	Geometry   json.RawMessage    `json:"geometry"`
	Properties ResponseProperties `json:"properties"`
}

// ResponseProperties wraps the buem output block in a response feature.
type ResponseProperties struct {
	BUEM ResponseBlock `json:"buem"`
}

// ResponseBlock is the buem object returned per building.
type ResponseBlock struct {
	ThermalLoadProfile ThermalLoadProfile `json:"thermal_load_profile"`
	ModelMetadata      ModelMetadata      `json:"model_metadata"`
}

// ThermalLoadProfile contains simulation results for one building.
type ThermalLoadProfile struct {
	StartTime      string         `json:"start_time"`
	EndTime        string         `json:"end_time"`
	Resolution     string         `json:"resolution"`
	ResolutionUnit string         `json:"resolution_unit"`
	Summary        ThermalSummary `json:"summary"`
	Timeseries     *Timeseries    `json:"timeseries,omitempty"`
	TimeseriesFile string         `json:"timeseries_file,omitempty"`

	// HeatingFile, CoolingFile, and ElectricityFile are injected by the
	// connector after it writes the CSVs — they are not part of BuEM's own
	// response.
	HeatingFile     string `json:"heating_file,omitempty"`
	CoolingFile     string `json:"cooling_file,omitempty"`
	ElectricityFile string `json:"electricity_file,omitempty"`
}

// ThermalSummary holds annual aggregate statistics per load type.
type ThermalSummary struct {
	Heating           LoadStats  `json:"heating"`
	Cooling           *LoadStats `json:"cooling,omitempty"`     // absent when compute_cooling=false (v4+)
	Electricity       *LoadStats `json:"electricity,omitempty"` // absent when not computed
	EnergyIntensity   *Quantity  `json:"energy_intensity,omitempty"`
	PeakHeatingLoad   *Quantity  `json:"peak_heating_load,omitempty"`
	PeakCoolingLoad   *Quantity  `json:"peak_cooling_load,omitempty"`
	TotalEnergyDemand *Quantity  `json:"total_energy_demand,omitempty"`
}

// LoadStats contains aggregate statistics for one load type.
type LoadStats struct {
	Total  Quantity  `json:"total"`
	Max    Quantity  `json:"max"`
	Min    Quantity  `json:"min"`
	Mean   Quantity  `json:"mean"`
	Median *Quantity `json:"median,omitempty"`
	Std    *Quantity `json:"std,omitempty"`
}

// Timeseries holds hourly load arrays. Present only when
// include_timeseries=true was requested.
type Timeseries struct {
	Unit        string    `json:"unit"`
	Timestamps  []string  `json:"timestamps"`
	Heating     []float64 `json:"heating"`
	Cooling     []float64 `json:"cooling,omitempty"`
	Electricity []float64 `json:"electricity,omitempty"`
}

// ModelMetadata describes how the simulation was executed.
type ModelMetadata struct {
	ModelVersion      string   `json:"model_version"`
	SolverUsed        string   `json:"solver_used"`
	ProcessingTime    Quantity `json:"processing_time"`
	WeatherYear       int      `json:"weather_year"`
	SimulationsRun    []string `json:"simulations_run,omitempty"`    // v4+
	ElectricitySource string   `json:"electricity_source,omitempty"` // v4+
}

// Quantity is a measurable value with an explicit unit.
type Quantity struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}
