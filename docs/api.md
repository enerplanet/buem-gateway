# API reference

## How to consume the API

buem-gateway has no authentication of its own — the `buem-reverse-proxy` in front of it is the
only way in. Any caller sends a valid `X-Api-Key` header; the proxy rejects anything else with
`403` before the connector ever sees the request. buem-gateway is meant to be called by a trusted
server-side caller (e.g. the EnerPlanET backend), never directly by a browser or an end user's
client, since the key must not be visible outside that caller.

!!! note "Base URL"
    In local development, `https://localhost:8443` (see `HOST_HTTPS_PORT` in
    [Getting started](getting-started.md)). In a deployment, whatever host the reverse proxy is
    published on.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness check — no auth required at the connector itself, but still only reachable through the proxy |
| `POST` | `/buem/start` | Run BuEM for every building in a topology, return the enriched topology |

## `POST /buem/start`

### Request

The top level describes the run; buildings live inside a `topology` array of `{from, to}` node
pairs (an edge list — the same shape EnerPlanET's grid topology already uses elsewhere). Only
nodes with `properties.feature_type == "BasePOI"` and a non-null `properties.buem` block are
treated as buildings; everything else in the topology passes through unchanged.

| Field | Type | Description |
|---|---|---|
| `model_id` | string | Isolates output — CSVs are written under `{BUEM_DATA_DIR}/{model_id}/` |
| `start_date` | RFC 3339 | Simulation window start; its **year** selects the weather file |
| `end_date` | RFC 3339 | Simulation window end |
| `resolution` | integer | Output resolution in minutes (e.g. `60`) |
| `topology` | array | List of `{ from, to }` node pairs |

Each building node's `properties.buem` is forwarded to BuEM opaquely — buem-gateway doesn't
interpret its contents beyond checking it's present and non-null. It must conform to whatever
schema BuEM's own request validator currently accepts; see `schemas/` in this repo and
`CHANGELOG.md` for what's actually implemented (not everything documented there is built yet —
`envelope` is still required, for example).

!!! info "Full example"
    `testdata/test_buem_topology_request.json` — two buildings, model `demo-model-001`. Used as
    the reference payload in the [reproducibility check](getting-started.md#reproducibility-check).

### Response

The same topology, with each processed building's `buem` block enriched:

| Field | Description |
|---|---|
| `thermal_load_profile.summary` | `heating`/`cooling`/`electricity` (`cooling` only present if `solver.compute_cooling` was `true`), each with `total` (kWh) plus `min`/`max`/`mean`/`median`/`std` (kW) |
| `thermal_load_profile.summary.peak_heating_load` / `peak_cooling_load` | Peak power (kW) |
| `thermal_load_profile.summary.energy_intensity` | Total demand per floor area (kWh/m²) |
| `thermal_load_profile.summary.total_energy_demand` | Sum of computed load types (kWh) |
| `thermal_load_profile.heating_file` / `cooling_file` / `electricity_file` | Path to each CSV on the shared volume — omitted for load types that weren't computed |
| `model_metadata.simulations_run` | Which load types were actually computed, e.g. `["heating", "electricity"]` |

A building that fails (BuEM validation error, upstream timeout, …) is left in the response
exactly as it was sent — check for the presence of `thermal_load_profile` before reading it. The
failure itself is logged by `buem-app`, not returned in the HTTP response body.

### CSV output

One CSV per computed load type, per building, written to `{BUEM_DATA_DIR}/{model_id}/`:

```
heating_{lat}_{lon}_{year}.csv
cooling_{lat}_{lon}_{year}.csv       (only if compute_cooling was true)
electricity_{lat}_{lon}_{year}.csv
```

Each is a single `demand` column of hourly values in kW (8760 rows for a full year), header
included:

```csv
demand
19.00950133202262
19.162132866903892
...
```
