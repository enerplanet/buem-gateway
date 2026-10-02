---
audience: developer
---

# API reference

The interactive reference is a standalone page, [`openapi/index.html`](openapi/index.html), which opens without `mkdocs serve`. It renders [`openapi/openapi.yaml`](openapi/openapi.yaml), which can be downloaded to generate a client or imported into Postman.

## Authentication

None. buem-gateway checks no credential, and neither does anything shipped in front of it. Requests carry no key.

!!! danger "The network is the only thing controlling access"
    Deploy buem-gateway only on a network whose access is already controlled, with callers authenticated by the EnerPlanET platform first. Anyone who can open a connection can run a simulation.

!!! note "Base URL"
    Local development: `http://localhost:8081` running `environment/http`, or `https://localhost:8443` running `environment/https`. Otherwise, whatever host the service is published on. See [Getting started](getting-started.md).

## Endpoints

buem-gateway has no concept of a grid or topology. A caller holding one (EnerPlanET's grid model, for example) resolves it to a flat list of buildings before calling.

| Method | Path | Request | Response |
| --- | --- | --- | --- |
| `GET` | `/buem/health` | None | Service status |
| `POST` | `/api/v1/buem/building` | One building with geometry, envelope, and weather | `buem` block with load profile and model metadata |
| `POST` | `/api/v1/buem/buildings` | Building list, each with geometry and envelope, plus one shared weather block | One result per building, in request order |
| `POST` | `/api/v1/buem/validate` | Same body as `/building` | Whether the request is well-formed. BuEM is not called |

### Buildings share weather, not envelope

`POST /api/v1/buem/buildings` takes one `weather` block for the whole request and attaches it to every building server-side. `envelope` differs per building, so it stays under each entry's own `building` field. Each entry may also carry an optional `solver` object, forwarded to BuEM as `buem.solver`; omit it for BuEM's defaults.

```json
{
  "start_date": "2018-01-01T00:00:00Z",
  "end_date": "2018-12-31T23:00:00Z",
  "resolution": 60,
  "model_id": "demo-model",
  "weather": {"index": ["2018-01-01T00:30:00Z"], "variables": {"T": [1.0]}},
  "buildings": [
    {"id": "111", "geometry": {"type": "Point", "coordinates": [12.5, 48.5]}, "building": {"envelope": {"elements": ["..."]}}},
    {"id": "222", "geometry": {"type": "Point", "coordinates": [12.6, 48.6]}, "building": {"envelope": {"elements": ["..."]}}}
  ]
}
```

The response is a list in the same order as `buildings`, each entry either `{"id": ..., "buem": {...}}` or `{"id": ..., "error": "..."}`. A `weather` block missing or incomplete at the top level fails every building in the request, each with its own `error` entry.

### Hourly values in the batch response

By default `POST /api/v1/buem/buildings` returns summary figures only, not hourly values. Send `"keep_timeseries": true` to get `thermal_load_profile.timeseries` populated for every building that ran, the same arrays `POST /api/v1/buem/building` returns.

!!! warning "Response size"
    A year of hourly values is roughly 300 KB of JSON per building. A batch of several hundred buildings with `keep_timeseries` set returns a response in the hundreds of megabytes.

### Pre-flight validation

`POST /api/v1/buem/validate` takes the same body as `POST /api/v1/buem/building` and runs the identical pre-flight, without ever calling BuEM: `model_id`, if present, uses only letters, digits, `.`, `_` and `-` (it has no other effect), `geometry` is a `Point` with a `[lon, lat]` pair, `start_date` is an ISO 8601 timestamp, `envelope` has at least one element with every field the [contract](versioning.md) marks required (`id`, `type`, and `area`/`azimuth`/`tilt` on non-ventilation elements), and `weather` has `index`, a usable variable, and every variable array the same length as `index`. Returns `{"valid": true}` on success, the same `400` shape described below otherwise, naming the first field that failed. A `200` means every check buem-gateway performs passed; BuEM may still reject the request on a field value buem-gateway does not inspect.

### Envelope is required

`buem.building.envelope` must be present and contain at least one element. buem-gateway does not derive geometry from the classification fields (`building_type`, `construction_period`, `country`), and calls no external service to resolve them.

| `envelope` | Behaviour |
|---|---|
| Present, every element carrying `id`, `type`, and `area`/`azimuth`/`tilt` (non-ventilation only) | Forwarded to BuEM unchanged |
| Missing, empty, or an element missing a required field | Rejected before BuEM is called. `POST /api/v1/buem/building` returns `400` with the field named in the body. `POST /api/v1/buem/buildings` gives that building its own `error` entry; every other building in the request is unaffected. |

To resolve TABULA defaults from classification data, call [ignis](https://github.com/THD-Spatial-AI/ignis) yourself and build a complete `envelope` first: `GET /api/v1/variants/{country}/match?type=...&period=...` then `GET /api/v1/data/{code}`.

!!! warning "construction_period is a TABULA class code, not a year range"
    `"01"`, `"02"` and so on: a country-specific numbered era class, never a literal year range like `"1965-1974"`. Classification metadata only, with no effect on the simulation.

### Weather is required

`buem.weather` must be present with `index` and at least one of `T`/`GHI`/`DNI`/`DHI` under `variables`, the shape [weather serve](https://github.com/enerplanet/weather)'s `GET /v1/weather/point?format=json` returns. buem-gateway does not resolve weather from any external service.

| `weather` | Behaviour |
|---|---|
| Present, with a usable variable | Forwarded to BuEM unchanged |
| Missing, or `variables` has none of T/GHI/DNI/DHI (e.g. only wind) | Rejected before BuEM is called. `POST /api/v1/buem/building` returns `400` with the reason in the body. `POST /api/v1/buem/buildings` gives every building in the request its own `error` entry, because the top-level `weather` field is shared. |

## Testing it yourself

[Open the API reference](openapi/index.html), which can call a locally running buem-gateway directly.

1. Start the stack, from `environment/http`: `docker compose up -d`. This pulls pre-built images from GHCR, so there is no `.env` and no build step. To test a local code change instead, see [Getting started](getting-started.md).
2. Serve `docs/openapi/` on `http://127.0.0.1:8000` (`python -m http.server 8000` from that directory works), since `ALLOWED_ORIGINS` allows that origin already. Opening the file directly (`file://`) works for reading the reference, but **Try it out** needs an allowed origin.
3. Pick the `http://localhost:8081` server in the **Servers** dropdown, which matches the stack you just started.
4. Expand an endpoint, click **Try it out**, fill in the parameters, then **Execute**. No credential is needed; nothing in the stack checks one.

For the TLS environment, start `environment/https`, choose the `https://localhost:8443` server, and first open `https://localhost:8443` in a browser tab and accept the untrusted-certificate warning. The bundled certificate authority is not in the browser's trust store, so the warning is expected.