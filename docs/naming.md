# Load profile file naming

Every load profile file written into the shared Docker volume follows one deterministic naming convention based on the building's geographic location. Any container that knows a building's coordinates, profile type, and simulation year can construct the file path directly, with no lookup and no index file.

This extends the existing EnerPlanET pattern used for PV, wind, biomass, and geothermal profiles (`{type}_{lat}_{lon}.csv`). BuEM adds a `{year}` segment because heating and cooling profiles are weather-year dependent.

## Pattern

```
{profile_type}_{lat}_{lon}_{year}.csv
```

| Part | Rule | Example |
|---|---|---|
| `profile_type` | `heating`, `cooling`, or `electricity` | `heating` |
| `lat` | Latitude from the feature geometry, 6 decimal places | `48.833911` |
| `lon` | Longitude from the feature geometry, 6 decimal places | `12.957720` |
| `year` | 4-digit simulation year taken from `start_time` | `2018` |

Example: `heating_48.833911_12.957720_2018.csv`

Coordinates are written exactly as they appear in the GeoJSON geometry, with no character substitution. The `.` in floating-point values is valid in Linux filenames, which is the only environment these containers run in. Negative values for western longitudes and southern latitudes keep their `-` sign: `heating_51.507351_-0.127758_2018.csv`.

## Why coordinates and not an ID

The identifier is the centroid of the Point feature geometry (`feature.geometry.coordinates`). Coordinates are the only identifier always present regardless of data source, and a building does not move, so the name is stable across runs.

OSM IDs, CityGML object IDs, and user-supplied IDs are each specific to one source and inconsistent across datasets, so none of them can name a file that a second container is expected to find unaided.

## Where files are written

One CSV per computed load type, per building, under a per-model directory:

```
${BUEM_DATA_DIR}/
  demo-model-001/
    heating_48.833911_12.957720_2018.csv
    electricity_48.833911_12.957720_2018.csv
    heating_48.833847_12.958071_2018.csv
    electricity_48.833847_12.958071_2018.csv
```

A `cooling_*.csv` is written only when `compute_cooling` was true in the request.

`BUEM_DATA_DIR` is set through the environment. Every container that reads or writes these files must mount the same Docker volume at that path.

## File format

A single `demand` column of hourly values in kW, header included, 8760 rows for a full year, ordered from `start_time` to `end_time`:

```csv
demand
19.00950133202262
19.162132866903892
...
```