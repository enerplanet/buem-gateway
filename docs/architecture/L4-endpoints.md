# L4 — Endpoint Reference

**Level:** 4 of 4 (interface — HTTP endpoint catalog)
**Audience:** API integrators, frontend developers, QA engineers
**Concern:** Every HTTP endpoint in the ecosystem — method, path, inputs,
outputs, error codes, and usage notes

---

## Services at a Glance

| Service | Host (local) | Docs |
|---------|-------------|------|
| BUEM Flask API | `http://localhost:5000` | `GET /api/docs` (Swagger UI) |
| Simulation gateway | `http://localhost:8089` | This document |
| EnerPlanET backend | `http://localhost:8000` | Internal OpenAPI |
| Auth service | `http://localhost:8001` | Internal |

---

## 1. BUEM Flask API  (`:5000`)

> Called directly by the simulation gateway. Can also be called directly for development and testing.

---

### `POST /api/process`

Simulates all buildings in a GeoJSON FeatureCollection.
This is the primary endpoint used by the simulation gateway.

**Query parameters**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `include_timeseries` | boolean | `false` | When `true`, the response includes hourly arrays (8 760 values per building per energy type) |

**Request body** — `application/json`

GeoJSON `FeatureCollection` conforming to [`request_schema.json`](../../schemas/request_schema.json).

```json
{
  "type": "FeatureCollection",
  "features": [
    {
      "type": "Feature",
      "id": "building_001",
      "geometry": { "type": "Point", "coordinates": [8.5, 52.1] },
      "properties": {
        "start_time": "2018-01-01T00:00:00Z",
        "end_time":   "2018-12-31T23:00:00Z",
        "resolution": 60,
        "buem": {
          "building": {
            "building_type": "SFH",
            "construction_period": "1965-1974",
            "country": "DE",
            "A_ref": { "value": 120.0, "unit": "m2" },
            "envelope": { "elements": [ ... ] },
            "thermal": { "comfortT_lb": { "value": 21.0, "unit": "degC" } }
          },
          "solver": { "use_milp": false }
        }
      }
    }
  ]
}
```

**Response body** — `application/json`

GeoJSON `FeatureCollection` conforming to [`response_schema.json`](../../schemas/response_schema.json).
Each feature echoes the request and appends `thermal_load_profile` and `model_metadata`.

```json
{
  "type": "FeatureCollection",
  "processed_at": "2024-06-01T10:15:30Z",
  "processing_elapsed_s": 3.2,
  "metadata": {
    "total_features": 1,
    "successful_features": 1,
    "failed_features": 0
  },
  "features": [
    {
      "type": "Feature",
      "id": "building_001",
      "geometry": { "type": "Point", "coordinates": [8.5, 52.1] },
      "properties": {
        "buem": {
          "building": { ... },
          "thermal_load_profile": {
            "start_time": "2018-01-01T00:00:00Z",
            "end_time":   "2018-12-31T23:00:00Z",
            "resolution": 60,
            "resolution_unit": "minutes",
            "summary": {
              "heating":     { "total": {"value": 12500, "unit": "kWh"}, "max": {"value": 8.2, "unit": "kW"}, "min": {"value": 0.0, "unit": "kW"}, "mean": {"value": 1.43, "unit": "kW"} },
              "cooling":     { "total": {"value":  1200, "unit": "kWh"}, "max": {"value": 3.1, "unit": "kW"}, "min": {"value": 0.0, "unit": "kW"}, "mean": {"value": 0.14, "unit": "kW"} },
              "electricity": { "total": {"value":  3600, "unit": "kWh"}, "max": {"value": 2.0, "unit": "kW"}, "min": {"value": 0.1, "unit": "kW"}, "mean": {"value": 0.41, "unit": "kW"} },
              "energy_intensity": { "value": 144.2, "unit": "kWh/m2" }
            },
            "timeseries_file": "heating_52.1_8.5_2018.json.gz"
          },
          "model_metadata": {
            "model_version": "0.1.2",
            "solver_used": "scipy-sparse",
            "processing_time": { "value": 2.1, "unit": "s" },
            "weather_year": 2018,
            "validation_warnings": []
          }
        }
      }
    }
  ]
}
```

**Status codes**

| Code | Meaning |
|------|---------|
| 200 | All features processed (check `metadata.failed_features` for partial failures) |
| 400 | Request does not conform to schema |
| 422 | Schema valid but logically inconsistent (e.g. end\_time ≤ start\_time) |
| 500 | Internal solver error |

---

### `POST /api/run`

Simulates a single building from a plain JSON config (not GeoJSON).
Used for testing and direct integration without the full GeoJSON wrapper.

**Request body** — `application/json`

Plain building configuration dict (no GeoJSON envelope).

**Response body** — `application/json`

Same `thermal_load_profile` structure as `/api/process`, without the GeoJSON wrapper.

**Status codes** — same as `/api/process`

---

### `GET /api/files/{filename}`

Downloads a compressed timeseries file written during a previous `/api/process` call.

**Path parameter**

| Parameter | Description |
|-----------|-------------|
| `filename` | Filename returned in `thermal_load_profile.timeseries_file` (e.g. `heating_52.1_8.5_2018.json.gz`) |

**Response** — `application/gzip`

Gzip-compressed JSON with the hourly arrays. Decompress and parse to obtain:

```json
{
  "unit": "kW",
  "timestamps": ["2018-01-01T00:00:00Z", ...],
  "heating":     [2.5, 3.1, 0.0, ...],
  "cooling":     [0.0, 0.0, 0.5, ...],
  "electricity": [0.4, 0.4, 0.5, ...]
}
```

**Status codes**

| Code | Meaning |
|------|---------|
| 200 | File found and returned |
| 404 | File not found (expired or never written) |

---

### `GET /api/health`

Liveness probe for Docker health checks and load balancer monitoring.

**Response** — `application/json`

```json
{ "status": "ok" }
```

**Status codes** — `200` always if the service is up.

---

### `GET /api/docs`

Serves the Swagger UI for interactive API exploration.

**Response** — `text/html`

Browser-renderable Swagger UI page.

---

## 2. Simulation Gateway  (`:8089`)

> All simulation engines share the same HTTP interface pattern.
> Replace `{svc}` with one of: `buem`, `calliope`, `pypsa`, `csv2json`, `charging`.

---

### `POST /{svc}/start`

Submits a simulation and returns the enriched result. **Execution is synchronous**
for the BuEM engine — the endpoint blocks until all buildings have been processed
and returns the complete enriched topology.

**Request body** — `application/json`

EnerPlanET `config.json` — the canonical energy model config. The gateway reads
only the namespace matching `{svc}` (e.g. `properties.buem` for the BuEM engine).
All other fields pass through unchanged.

Minimum required fields for the BuEM engine:

```json
{
  "model_id": "my-model-001",
  "start_date": "2018-01-01T00:00:00Z",
  "end_date":   "2018-12-31T23:00:00Z",
  "resolution": 60,
  "topology": [
    {
      "from": {
        "id": "building_001",
        "geometry": { "type": "Point", "coordinates": [8.5, 52.1] },
        "properties": {
          "feature_type": "BasePOI",
          "buem": {
            "building": { "building_type": "SFH", "A_ref": { "value": 120.0, "unit": "m2" },
              "envelope": { "elements": [ ... ] }
            },
            "solver": { "use_milp": false }
          }
        }
      },
      "to": { ... }
    }
  ]
}
```

**Response body** — `application/json`

Enriched `config.json` with each BuEM building node updated:

```json
{
  "model_id": "my-model-001",
  "topology": [
    {
      "from": {
        "properties": {
          "buem": {
            "thermal_load_profile": {
              "summary": {
                "heating":     { "total": { "value": 90067, "unit": "kWh" }, "max": { "value": 10.5, "unit": "kW" }, ... },
                "cooling":     { "total": { "value":     0, "unit": "kWh" }, ... },
                "electricity": { "total": { "value":  3750, "unit": "kWh" }, ... },
                "energy_intensity": { "value": 258.2, "unit": "kWh/m2" }
              },
              "timeseries": {
                "unit": "kW",
                "timestamps": ["2018-01-01T00:00:00Z", ...],
                "heating":     [10.3, 10.4, ...],
                "cooling":     [0.0, 0.0, ...],
                "electricity": [0.04, 0.04, ...]
              },
              "heating_file":     "/webservice/data/buem/my-model-001/heating_52.100000_8.500000_2018.csv",
              "cooling_file":     "/webservice/data/buem/my-model-001/cooling_52.100000_8.500000_2018.csv",
              "electricity_file": "/webservice/data/buem/my-model-001/electricity_52.100000_8.500000_2018.csv"
            },
            "model_metadata": {
              "model_version": "0.1.2",
              "solver_used": "scipy-sparse",
              "processing_time": { "value": 6.8, "unit": "s" },
              "weather_year": 2018
            }
          }
        }
      }
    }
  ]
}
```

**Status codes**

| Code | Meaning |
|------|---------|
| 200 | All buildings processed; enriched config returned |
| 400 | Malformed request body or missing required fields |
| 500 | BuEM service error or CSV write failure |

---

### `GET /{svc}/show`

Returns an empty object `{}` for the BuEM engine — BuEM does not maintain active
simulation state between requests (processing is synchronous via `/start`).

**Status codes** — `200` always.

---

### `GET /{svc}/log`

Returns execution log for the current or most recent job. Useful for debugging failed runs.

**Response body** — `text/plain`

Raw log output from the engine.

---

### `POST /{svc}/configure`

Validates and caches the simulation config without starting execution.
Useful for pre-flight checks before submitting a long job.

**Request body** — same as `/start`

**Response body** — `application/json`

```json
{ "valid": true, "warnings": [] }
```

---

### `POST /{svc}/generate`

Pre-processes inputs (e.g. generates `locations.yaml` for Calliope from the config).
For BUEM this is a no-op — data is transformed inline during `/start`.

---

### `POST /{svc}/finish`

Signals that the client has consumed the results. The gateway may release cached state.

---

### `GET /health`

Gateway liveness probe.

**Response** — `application/json`

```json
{ "status": "ok" }
```

---

### `GET /status`

Returns gateway resource usage and job queue state.

**Response** — `application/json`

```json
{
  "active": true,
  "jobs": [],
  "cpu_usage": 32.5,
  "memory_usage": 48.1
}
```

---

## 3. EnerPlanET Backend  (`:8000`)

> The backend API is internal to the EnerPlanET platform. Listed here for completeness.

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/api/simulate` | Submit a full energy simulation scenario (BUEM + Calliope + PyPSA) |
| `GET` | `/api/results/{id}` | Retrieve stored simulation results |
| `GET` | `/api/scenarios` | List saved scenarios |
| `POST` | `/api/scenarios` | Create a new scenario |

All endpoints require a valid bearer token from Keycloak.

---

## 4. Auth Service  (`:8001`)

| Method | Path | Purpose |
|--------|------|---------|
| `POST` | `/validate` | Validate a Keycloak bearer token; returns user claims |
| `GET` | `/health` | Liveness probe |

---

## Notes

**Measurement units** — All numeric values in BUEM request/response payloads carry
an explicit `unit` field. SI units are assumed when `unit` is omitted.
See [SCHEMA_OVERVIEW.md](../../SCHEMA_OVERVIEW.md) for the full unit table.

**Timeseries on demand** — By default, hourly arrays are **not** returned in the
response body. Pass `?include_timeseries=true` to `/api/process` to embed them.
The gateway always requests timeseries (so it can write CSVs) and retains them
in the enriched `config.json` returned to the EnerPlanET backend.

**Three profile types** — The gateway writes and returns file paths for three
profiles per building: `heating`, `cooling`, and `electricity`. The electricity
profile is BuEM's occupancy-modelled appliance load — it should **replace** the
SLP electricity demand in Calliope for BuEM-simulated buildings, not supplement it.

**Profile isolation** — Profiles are stored under `{BUEM_DATA_DIR}/{model_id}/`.
Profiles for different models never share a directory, preventing filename collisions.
Delete the folder when the model is deleted.

**File naming convention** — `{type}_{lat}_{lon}_{year}.csv`
e.g. `heating_52.100000_8.500000_2018.csv`. Coordinates use 6 decimal places.
