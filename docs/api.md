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
interpret its contents beyond checking it's present and non-null, and one exception:
`building.envelope`, described next. The rest must conform to whatever schema BuEM's own request
validator currently accepts; see `schemas/` in this repo and `CHANGELOG.md` for what's actually
implemented (not everything documented there is built — BuEM's own model doesn't support
`solver.compute_cooling`'s upstream request format changing shape yet, for example, only the
opt-in flag itself).

!!! info "Full example"
    `testdata/test_buem_topology_request.json` — two buildings, model `demo-model-001`. Used as
    the reference payload in the [reproducibility check](getting-started.md#reproducibility-check).

### TABULA fallback when `envelope` is omitted

BuEM's own validator still hard-requires `building.envelope` — that hasn't changed. But
buem-gateway itself doesn't: if a building node's `buem.building.envelope` is missing, the
connector resolves TABULA defaults from [ignis](https://github.com/THD-Spatial-AI/ignis) (reached
directly by service name on the shared `building-simulation` Docker network — see
[Getting started](getting-started.md#the-building-simulation-namespace)) using
`building_type`/`construction_period`/`country`, maps them into BuEM's per-element `envelope`
shape, and injects them before the request ever reaches BuEM. BuEM itself never sees a request
without `envelope` — from BuEM's side, nothing has changed.

!!! warning "construction_period is a TABULA class code, not a year range"
    `"01"`, `"02"`, ... — a country-specific numbered era class, never a literal year range like
    `"1965-1974"`. See `CHANGELOG.md`'s "Unreleased" entry for why.

Because TABULA gives no per-element orientation for walls/roof/floor (only an aggregate area per
category), the mapping makes an explicit assumption for those: each wall category's area is split
evenly across the 4 cardinal directions (N/E/S/W), and roof/floor become one south-facing/
untilted element per category. Windows are the exception — TABULA does track a real
North/East/South/West/Horizontal area split, used directly rather than assumed.

| Situation | Behavior |
|---|---|
| `envelope` present | Forwarded to BuEM unchanged — TABULA/ignis are never consulted |
| `envelope` missing, ignis reachable, variant found | TABULA-derived envelope injected, request proceeds |
| `envelope` missing, ignis unreachable or no matching variant | Request forwarded to BuEM unchanged; BuEM rejects it with its own `envelope required` error — logged, not silently swallowed |

`A_ref`, `h_room`, `n_storeys`, `neighbour_status`, `attic_condition`, `cellar_condition`, and
`thermal.n_air_infiltration`/`n_air_use` are also filled from TABULA when the request omits them
— but never overwritten if the request already supplied a value.

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
