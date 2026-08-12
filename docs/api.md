# API reference

The interactive reference below is generated from the OpenAPI spec ([`openapi.yaml`](openapi.yaml)). Download it to generate a client or import it into Postman.

## Authentication

buem-gateway has no authentication of its own. The `buem-reverse-proxy` in front of it is the only way in: callers send a valid `X-Api-Key` header, and the proxy rejects anything else with `403` before the request reaches the connector.

!!! warning "Call it server-side only"
    The API key must not be visible outside the calling service. Call buem-gateway from a trusted server-side caller such as the EnerPlanET backend, never directly from a browser or an end user's client.

!!! note "Base URL"
    Local development: `https://localhost:8443` (see `HOST_HTTPS_PORT` in [Getting started](getting-started.md)). Otherwise, whatever host the reverse proxy is published on.

## Two ways to call it

| Endpoint | Payload | Use when |
|---|---|---|
| `POST /api/v1/buem/building` | One building, no wrapper | A single building with no grid to describe, for example the Building Configurator. A failed run returns a clear HTTP error. |
| `POST /api/v1/buem/topology` | A topology (`{from, to}` edge list) | Several buildings, possibly alongside non-building grid nodes. A failed building is left unchanged in the response rather than failing the whole request. |

### Pre-flight validation

`POST /api/v1/buem/validate` takes the same body as `POST /api/v1/buem/building` and checks that `envelope` and `weather` are both present with usable data, without ever calling BuEM. Returns `{"valid": true}` on success, the same `400` shape described below otherwise. Useful for a caller (the Orchestrator) confirming a request is well-formed before paying for the real run -- a `200` here doesn't guarantee BuEM will accept the request, only that the two things buem-gateway itself checks are present.

### Envelope is required

`buem.building.envelope` must be present and contain at least one element. buem-gateway does not derive geometry from the classification fields (`building_type`, `construction_period`, `country`), and calls no external service to resolve them.

| `envelope` | Behaviour |
|---|---|
| Present | Forwarded to BuEM unchanged |
| Missing or empty | Rejected before BuEM is called. `POST /api/v1/buem/building` returns `400` with the reason in the body. `POST /api/v1/buem/topology` skips that building, leaves it unchanged in the response, and logs the reason server-side. |

To resolve TABULA defaults from classification data, call [ignis](https://github.com/THD-Spatial-AI/ignis) yourself and build a complete `envelope` first: `GET /api/v1/variants/{country}/match?type=...&period=...` then `GET /api/v1/data/{code}`.

!!! warning "construction_period is a TABULA class code, not a year range"
    `"01"`, `"02"` and so on: a country-specific numbered era class, never a literal year range like `"1965-1974"`. Classification metadata only, with no effect on the simulation.

### Weather is required

`buem.weather` must be present with `index` and at least one of `T`/`GHI`/`DNI`/`DHI` under `variables` -- the shape [weather serve](https://github.com/enerplanet/weather)'s `GET /v1/weather/point?format=json` returns. buem-gateway does not resolve weather from any external service either; BuEM itself has raised on missing weather since [enerplanet/buem#10](https://github.com/enerplanet/buem/issues/10), and this check surfaces that as a clear `400` here instead of a `422` after an unnecessary round trip to BuEM.

| `weather` | Behaviour |
|---|---|
| Present, with a usable variable | Forwarded to BuEM unchanged |
| Missing, or `variables` has none of T/GHI/DNI/DHI (e.g. only wind) | Rejected before BuEM is called. `POST /api/v1/buem/building` returns `400` with the reason in the body. `POST /api/v1/buem/topology` skips that building, leaves it unchanged in the response, and logs the reason server-side. |

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

The Swagger UI below can call a locally running buem-gateway directly.

1. Start the stack, from `environment/`: `docker compose -f docker-compose.quickstart.yml up -d`. This pulls pre-built images from GHCR, so there is no `.env` and no build step. To test a local code change instead, see [Getting started](getting-started.md).
2. Serve these docs locally with `mkdocs serve`.
3. Open `https://localhost:8443` directly once and click through the untrusted-certificate warning. The quickstart certificate is never added to your trust store (see [Getting started](getting-started.md#try-it-out-no-caddy-setup)), so this is expected rather than a setup mistake.
4. Click **Authorize** below and enter the API key checked by the reverse proxy (`X-Api-Key`; the prototype default is set in `environment/env/proxy.env`). It applies to every **Try it out** call from then on. `/health` needs no key.
5. Expand an endpoint, click **Try it out**, fill in the parameters, then **Execute**.

!!! bug "Swagger UI bug"
    The Swagger UI sometimes fails to load. Reload the browser page and it should come up correctly.

<swagger-ui src="openapi.yaml"/>