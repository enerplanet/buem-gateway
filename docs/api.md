# API reference

The interactive reference below is generated from the OpenAPI spec
([`openapi.yaml`](openapi.yaml)), the machine-readable source of truth for every endpoint, schema,
and error. Download it to generate a client, load it into Postman, or import it into another tool.

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

## Two ways to call it

| Endpoint | Shape | Use when |
|---|---|---|
| `POST /api/v1/buem/building` | One building, no wrapper | You have exactly one building and no grid to describe (e.g. Building Configurator) — a failed run is a clear HTTP error |
| `POST /api/v1/buem/start` | A topology (`{from, to}` edge list) | You have several buildings, possibly alongside non-building grid nodes — a failed building is left unchanged in the response rather than failing the whole request |

Both share the same `buem` block shape and the same CSV output — described below.

### Envelope is required

`buem.building.envelope` must be present, with at least one element. buem-gateway resolves
nothing from any external service — it does not call ignis, or anything else, to derive geometry
from classification fields (`building_type`/`construction_period`/`country`) when envelope is
omitted.

!!! note "This used to work differently"
    An earlier version (schema v4.x) called [ignis](https://github.com/THD-Spatial-AI/ignis)
    directly to resolve TABULA defaults when envelope was omitted. That made buem-gateway's own
    "standalone, independently deployable" claim false in practice — it silently needed a second
    service reachable at `ignis-app:8080` to accept a request shape its own schema advertised as
    valid — and the failure mode was bad: an unreachable/non-matching lookup fell through to
    forwarding the incomplete request to BuEM, which rejected it with a generic "invalid GeoJSON
    payload" error that named neither TABULA nor ignis. Removed — see `CHANGELOG.md`'s v5.0.0
    entry.

| Situation | Behavior |
|---|---|
| `envelope` present | Forwarded to BuEM unchanged |
| `envelope` missing or empty | Rejected immediately with a clear error — BuEM is never called. `POST /api/v1/buem/building`: `400` with the reason in the body. `POST /api/v1/buem/start`: that building is skipped and left unchanged in the response, reason logged server-side (same partial-success handling as any other per-building failure) |

If you need to resolve TABULA defaults from classification data, call
[ignis](https://github.com/THD-Spatial-AI/ignis) yourself and build a complete `envelope` before
calling buem-gateway — it's a one-hop lookup (`GET /api/v1/variants/{country}/match?type=...&period=...`
then `GET /api/v1/data/{code}`), not something worth another service silently doing on your behalf.

!!! warning "construction_period is a TABULA class code, not a year range"
    `"01"`, `"02"`, ... — a country-specific numbered era class, never a literal year range like
    `"1965-1974"`. Classification metadata only now — has no effect on the simulation.

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

## Testing it yourself

The Swagger UI below can call a locally running buem-gateway directly.

1. Start the stack, from `environment/`: `docker compose -f docker-compose.quickstart.yml up -d`
   — pre-built images from GHCR, no `.env`, no build. (Testing a local code change instead? Use
   `cp .env.example .env` then `docker compose up -d --build` — see
   [Getting started](getting-started.md).)
2. Serve these docs locally with `mkdocs serve`.
3. The quickstart stack's certificate is never added to your OS/browser trust store (see
   [Getting started](getting-started.md#try-it-out-no-caddy-setup)), so open
   `https://localhost:8443` directly once and click through the untrusted-certificate warning —
   expected, not a setup mistake.
4. Click **Authorize** below and enter the API key checked by the reverse proxy (`X-Api-Key`; the
   prototype default is set in `environment/env/proxy.env`). It applies to every **Try it out**
   call from then on. `/health` needs no key.
5. Expand an endpoint, click **Try it out**, fill in the parameters, and **Execute**.

!!! bug "Swagger UI Bug"
    The Swagger UI might not load correctly, just reload the browser page and it should load correctly.

<swagger-ui src="openapi.yaml"/>
