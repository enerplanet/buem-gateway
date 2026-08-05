# Getting started

## Prerequisites

| Dependency | Version |
|---|---|
| Docker + Compose plugin | any recent |
| Go | 1.26+ (only needed to build/test outside Docker) |
| Caddy (host, optional) | for `caddy trust` — see below |

## Local dev

```bash
cd environment
cp .env.example .env   # then edit CADDY_DATA_DIR for your machine
docker compose up -d --build
```

This starts three containers: `buem-model` (the actual BuEM Flask model, built by
`git clone`-ing `enerplanet/buem` at image build time), `buem-gateway` (this repo's Go connector),
and `buem-reverse-proxy` (Caddy — the only one reachable from the host).

| Variable | File | Purpose |
|---|---|---|
| `HOST_HTTPS_PORT` | `.env` | Host port the reverse proxy publishes (default `8443`, not `443`, so it doesn't collide with ignis's own reverse proxy on the same host) |
| `APP_PORT` | `.env` | Internal port `buem-gateway` listens on |
| `CADDY_DATA_DIR` | `.env` | Host path to Caddy's trusted local CA — run `caddy trust` once on the host, then point this at where that created the CA |
| `ALLOWED_ORIGINS` | `env/common.env` | CORS origins the reverse proxy accepts |
| `BUEM_API_KEY` | `env/proxy.env` | Value callers must send as `X-Api-Key`. Local-dev placeholder — rotate before any real deployment |
| `BUEM_WEATHER_DIR_HOST` | `.env` | Host path to MERRA-2 weather data — see below |

!!! note "buem-gateway is not reachable from the host"
    Neither `buem-gateway` nor `buem-model` publish a port. The reverse proxy is the only way in —
    same pattern as [ignis](https://github.com/THD-Spatial-AI/ignis).

## The `building-simulation` namespace

`docker-compose.yml` declares `name: building-simulation` — the same project name the standalone
`ignis` repo's own compose file uses. Bringing both stacks up (from their own repos,
independently) puts every container on the same `building-simulation_default` Docker network, so
`buem-gateway` can reach `ignis-app` directly by service name if it ever needs to (e.g. the planned
TABULA-fallback lookup).

!!! warning "Don't share this project name with anything else"
    Compose tracks ownership by `(project name, service key)`, not `container_name`. Sharing
    `building-simulation` with a compose file that happens to reuse a service key — `buem-model`,
    say — will cause `docker compose up` in one repo to silently recreate the other's container
    using its own definition. This actually happened once during development against
    `simulation-engine`'s bundled deployment; that deployment intentionally does **not** share
    this namespace as a result.

## Weather data

BuEM reads MERRA-2 NetCDF files named `combined_merra_{year}.nc`, organised by country
sub-directory, mounted read-only into `buem-model` at `/buem/data/weather`:

```
${BUEM_WEATHER_DIR_HOST}/
├── germany/       combined_merra_2015.nc … combined_merra_2025.nc
├── austria/
├── czech/
└── netherlands/
```

!!! warning "No weather data → synthetic fallback, not a hard failure"
    BuEM starts and reports itself healthy either way. Without a matching file for a building's
    country and simulation year, it silently falls back to a synthetic zero-filled weather
    profile — heating/cooling numbers will be present but physically meaningless. Confirm the
    files are visible **inside** the container before trusting any result:
    ```bash
    docker exec buem-model ls /buem/data/weather/germany
    ```

## Reproducibility check

`testdata/test_buem_topology_request.json` is a two-building fixture (Germany, one SFH, one MFH,
full envelope + thermal data). With the stack up:

```bash
curl -sk -X POST https://localhost:8443/buem/start \
  -H "Content-Type: application/json" -H "X-Api-Key: dev-placeholder-change-me" \
  -d @testdata/test_buem_topology_request.json | jq '{
    building_1_heating_kWh: .topology[0].from.properties.buem.thermal_load_profile.summary.heating.total,
    building_2_heating_kWh: .topology[0].to.properties.buem.thermal_load_profile.summary.heating.total
  }'
```

!!! info "Numbers below use the synthetic weather fallback"
    No MERRA-2 data is mounted in a fresh clone, so these come from BuEM's zero-filled weather
    substitute — they confirm the pipeline works end-to-end, not realistic energy demand. Point
    `BUEM_WEATHER_DIR_HOST` at real MERRA-2 data for physically meaningful numbers.

```json
{
  "building_1_heating_kWh": { "unit": "kWh", "value": 39450.809 },
  "building_2_heating_kWh": { "unit": "kWh", "value": 82893.873 }
}
```

Four CSVs should also exist (heating + electricity per building — `compute_cooling` is `false`
in the fixture, so no cooling CSV):

```bash
docker exec buem-gateway ls /app/data/demo-model-001/
```

## Deployment

!!! danger "Do not expose buem-gateway or buem-model directly"
    Neither has authentication of its own. `buem-reverse-proxy` (Caddy, `X-Api-Key`) is the only
    intended entry point — same model as ignis. Run the stack on a private network with only the
    reverse proxy's port published.

For a shared host running both `ignis` and `buem-gateway`, bring each stack up from its own repo
independently — see [The `building-simulation` namespace](#the-building-simulation-namespace)
above. `HOST_HTTPS_PORT` must differ between the two (ignis defaults to `443`, buem-gateway to
`8443`) since both publish through Caddy on the same host.
