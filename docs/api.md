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
| `POST /buem/building` | One building, no wrapper | You have exactly one building and no grid to describe (e.g. Building Configurator) — a failed run is a clear HTTP error |
| `POST /buem/start` | A topology (`{from, to}` edge list) | You have several buildings, possibly alongside non-building grid nodes — a failed building is left unchanged in the response rather than failing the whole request |

Both share the same `buem` block shape, the same TABULA-fallback behavior, and the same CSV
output — described below.

### TABULA fallback when `envelope` is omitted

BuEM's own validator still hard-requires `building.envelope` — that hasn't changed. But
buem-gateway itself doesn't: if a building's `buem.building.envelope` is missing, the connector
resolves TABULA defaults from [ignis](https://github.com/THD-Spatial-AI/ignis) (reached directly
by service name on the shared `building-simulation` Docker network — see
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

1. Start the stack, from `environment/`: `cp .env.example .env` (then edit `CADDY_DATA_DIR`),
   `docker compose up -d --build`.
2. Serve these docs locally with `mkdocs serve`.
3. If your browser has never trusted the local proxy's certificate, open `https://localhost:8443`
   directly once and accept it (or run `caddy trust`).
4. Click **Authorize** below and enter the API key checked by the reverse proxy (`X-Api-Key`; the
   prototype default is set in `environment/docker.env`). It applies to every **Try it out** call
   from then on.
5. Expand an endpoint, click **Try it out**, fill in the parameters, and **Execute**.

<swagger-ui src="openapi.yaml"/>
