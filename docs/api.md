# API reference

The interactive reference lives in its own standalone page, [`openapi/index.html`](openapi/index.html), so it can be opened directly without running `mkdocs serve`. It renders [`openapi/openapi.yaml`](openapi/openapi.yaml); download that file to generate a client or import it into Postman.

## Authentication

buem-gateway has no authentication of its own. The `buem-reverse-proxy` in front of it is the only way in: callers send a valid `X-Api-Key` header, and the proxy rejects anything else with `403` before the request reaches the connector.

!!! warning "Call it server-side only"
    The API key must not be visible outside the calling service. Call buem-gateway from a trusted server-side caller such as the EnerPlanET backend, never directly from a browser or an end user's client.

!!! note "Base URL"
    Local development: `https://localhost:8443` (see `HOST_HTTPS_PORT` in [Getting started](getting-started.md)). Otherwise, whatever host the reverse proxy is published on.

## Endpoints

BuEM gateway exposes the following endpoints for running building models. It has no concept of a grid or topology — a caller that has one (EnerPlanET's grid model, for example) resolves it down to a flat list of buildings itself before calling either endpoint.

| Method | Path | Request | Response |
| --- | --- | --- | --- |
| `GET` | `/buem/health` | None | Service status |
| `POST` | `/api/v1/buem/building` | One building with geometry, envelope, and weather | `buem` block with load profile and model metadata |
| `POST` | `/api/v1/buem/buildings` | Building list, each with geometry and envelope, plus one shared weather block | One result per building, in request order |
| `POST` | `/api/v1/buem/validate` | Same body as `/building` | Whether the request is well-formed. BuEM is not called |

### Buildings share weather, not envelope

`POST /api/v1/buem/buildings` takes one `weather` block for the whole request, re-attached to every building server-side, instead of one copy per building. Most callers resolve weather once per model run (one point for the model's whole area) — repeating an hourly-for-a-year timeseries once per building would be pure duplication. `envelope` has no such sharing: it's genuinely different per building, so it stays under each entry's own `building` field.

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

The response is a list in the same order as `buildings`, each entry either `{"id": ..., "buem": {...}}` or `{"id": ..., "error": "..."}`. A `weather` block missing or incomplete at the top level fails every building in the request, each with its own `error` entry — it isn't a per-building concern the way `envelope` is.

### Pre-flight validation

`POST /api/v1/buem/validate` takes the same body as `POST /api/v1/buem/building` and checks that `envelope` and `weather` are both present with usable data, without ever calling BuEM. Returns `{"valid": true}` on success, the same `400` shape described below otherwise. Useful for a caller (the Orchestrator) confirming a request is well-formed before paying for the real run -- a `200` here doesn't guarantee BuEM will accept the request, only that the two things buem-gateway itself checks are present.

### Envelope is required

`buem.building.envelope` must be present and contain at least one element. buem-gateway does not derive geometry from the classification fields (`building_type`, `construction_period`, `country`), and calls no external service to resolve them.

| `envelope` | Behaviour |
|---|---|
| Present | Forwarded to BuEM unchanged |
| Missing or empty | Rejected before BuEM is called. `POST /api/v1/buem/building` returns `400` with the reason in the body. `POST /api/v1/buem/buildings` gives that building its own `error` entry; every other building in the request is unaffected. |

To resolve TABULA defaults from classification data, call [ignis](https://github.com/THD-Spatial-AI/ignis) yourself and build a complete `envelope` first: `GET /api/v1/variants/{country}/match?type=...&period=...` then `GET /api/v1/data/{code}`.

!!! warning "construction_period is a TABULA class code, not a year range"
    `"01"`, `"02"` and so on: a country-specific numbered era class, never a literal year range like `"1965-1974"`. Classification metadata only, with no effect on the simulation.

### Weather is required

`buem.weather` must be present with `index` and at least one of `T`/`GHI`/`DNI`/`DHI` under `variables` -- the shape [weather serve](https://github.com/enerplanet/weather)'s `GET /v1/weather/point?format=json` returns. buem-gateway does not resolve weather from any external service either; BuEM itself has raised on missing weather since [enerplanet/buem#10](https://github.com/enerplanet/buem/issues/10), and this check surfaces that as a clear `400` here instead of a `422` after an unnecessary round trip to BuEM.

| `weather` | Behaviour |
|---|---|
| Present, with a usable variable | Forwarded to BuEM unchanged |
| Missing, or `variables` has none of T/GHI/DNI/DHI (e.g. only wind) | Rejected before BuEM is called. `POST /api/v1/buem/building` returns `400` with the reason in the body. `POST /api/v1/buem/buildings` gives every building in the request its own `error` entry — the top-level `weather` field is shared, so a missing one affects the whole batch, not one building. |

### CSV output

One CSV per computed load type, per building, written to `{BUEM_DATA_DIR}/{model_id}/`:

```
heating_{lat}_{lon}_{year}.csv
cooling_{lat}_{lon}_{year}.csv       (only if compute_cooling was true)
electricity_{lat}_{lon}_{year}.csv
```

Each is a single `demand` column of hourly values in kW (8760 rows for a full year), header included:

```csv
demand
19.00950133202262
19.162132866903892
...
```

## Testing it yourself

[Open the API reference](openapi/index.html), which can call a locally running buem-gateway directly.

1. Start the stack, from `environment/`: `docker compose -f docker-compose.quickstart.yml up -d`. This pulls pre-built images from GHCR, so there is no `.env` and no build step. To test a local code change instead, see [Getting started](getting-started.md).
2. Serve `docs/openapi/` on `http://127.0.0.1:8000` (`python -m http.server 8000` from that directory works), since the reverse proxy's `ALLOWED_ORIGINS` allows that origin already. Opening the file directly (`file://`) works for reading the reference, but **Try it out** needs an allowed origin.
3. Open `https://localhost:8443` directly once and click through the untrusted-certificate warning. The quickstart certificate is never added to your trust store (see [Getting started](getting-started.md#try-it-out-no-caddy-setup)), so this is expected rather than a setup mistake.
4. Click **Authorize** and enter the API key checked by the reverse proxy (`X-Api-Key`; the prototype default is set in `environment/env/proxy.env`). It applies to every **Try it out** call from then on. `/buem/health` needs no key.
5. Expand an endpoint, click **Try it out**, fill in the parameters, then **Execute**.