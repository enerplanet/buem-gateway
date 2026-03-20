# File Naming Standard — Load Profiles

All load profile files shared across the EnerPlanET Docker environment follow a
single deterministic naming convention based on the building's geographic location.
Any container that knows the building coordinates, profile type, and simulation year
can construct the file path without a lookup.

This convention extends the existing EnerPlanET pattern used for PV, Wind, Biomass,
and Geothermal profiles (`{type}_{lat}_{lon}.csv`). BUEM adds a `{year}` segment
because heating and cooling profiles are weather-year dependent.

---

## Building identifier — centroid coordinate

The building identifier is derived from the **centroid coordinates** of the Point
feature geometry (`feature.geometry.coordinates`). Coordinates are the only
identifier that is:

- Always present — regardless of data source (OSM, CityGML, user-defined)
- Stable — a building does not move
- Source-agnostic — OSM IDs, CityGML object IDs, and user IDs are all
  source-specific and inconsistent across datasets

---

## Pattern

```
{profile_type}_{lat}_{lon}_{year}.{ext}
```

| Part | Rule | Example |
|---|---|---|
| `profile_type` | `electricity`, `heating`, or `cooling` | `electricity` |
| `lat` | Latitude from geometry, 6 decimal places | `48.833911` |
| `lon` | Longitude from geometry, 6 decimal places | `12.957720` |
| `year` | 4-digit simulation year from `start_time` | `2018` |
| `ext` | `csv` for inputs · `json.gz` for outputs | `csv` |

**Example:** `electricity_48.833911_12.957720_2018.csv`

Coordinates are written exactly as they appear in the GeoJSON geometry —
no character substitution needed. The `.` in floating-point values is valid in
Linux filenames (the Docker environment). Negative values (western longitudes,
southern latitudes) retain their `-` sign: e.g. `heating_51.507351_-0.127758_2018.json.gz`.

---

## Directory layout on the shared Docker volume

```
$DATA_DIR/
  profiles/          ← input profiles written by EnerPlanET
    electricity_48.833911_12.957720_2018.csv
    electricity_48.833847_12.958071_2018.csv

  results/           ← output timeseries written by model containers
    heating_48.833911_12.957720_2018.json.gz
    cooling_48.833911_12.957720_2018.json.gz
```

`$DATA_DIR` is set via the `BUEM_DATA_DIR` environment variable. Every container
that reads or writes profile files must mount the same Docker volume at this path.

---

## File formats

### Input — CSV

Single column, no header, one value per line, ordered from `start_time` to
`end_time`.

```csv
0.42
0.38
0.35
```

### Output — gzipped JSON

```json
{
  "lat": 48.833911,
  "lon": 12.957720,
  "profile_type": "heating",
  "year": 2018,
  "unit": "kWh",
  "index": ["2018-01-01T00:00:00", "2018-01-01T01:00:00"],
  "values": [1.2, 0.8]
}
```
