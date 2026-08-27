# Getting started

## Prerequisites

| Dependency | Version |
|---|---|
| Docker + Compose plugin | any recent |
| Go | 1.26+ (only needed to build or test outside Docker) |
| Caddy (host, optional) | for `caddy trust`, see below |

## Try it out (no Caddy setup)

`docker-compose.quickstart.yml` pulls the pre-built, CI-published `buem-gateway` and `buem-model` images from GHCR instead of building from source: no Go toolchain, no conda, and no local Caddy install or `caddy trust` needed.

```bash
cd environment
docker compose -f docker-compose.quickstart.yml up -d
curl -sk https://localhost:8443/buem/health
```

No `.env` is required. Every `${...}` in that file has a default (`APP_PORT` 8080, `HOST_HTTPS_PORT` 8443, `BUEM_IMAGE_TAG` `latest`).

!!! warning "Trade-off against a real deployment"
    Caddy's local CA lives in a Docker-managed volume here instead of a host bind mount, so it is never added to your OS or browser trust store. `https://localhost:8443` will show an untrusted-certificate warning: expected, not a bug. Pass `-k` or `--no-check-certificate` (curl, wget), or click through the browser warning. Use `docker-compose.prod.yml` for a real trust chain or public domain.

## Local dev (building from source)

For testing local code changes to `buem-gateway` or `buem-model` itself:

```bash
cd environment
cp .env.example .env   # then edit CADDY_DATA_DIR for your machine
docker compose up -d --build
```

This starts three containers: `buem-model` (the actual BuEM Flask model, built by `git clone`-ing `enerplanet/buem` at image build time), `buem-gateway` (this repo's Go connector), and `buem-reverse-proxy` (Caddy, the only one reachable from the host).

| Variable | File | Purpose |
|---|---|---|
| `HOST_HTTPS_PORT` | `.env` | Host port the reverse proxy publishes (default `8443`, not `443`, so it does not collide with ignis's own reverse proxy on the same host) |
| `APP_PORT` | `.env` | Internal port `buem-gateway` listens on |
| `CADDY_DATA_DIR` | `.env` | Host path to Caddy's trusted local CA. Run `caddy trust` once on the host, then point this at where that created the CA |
| `ALLOWED_ORIGINS` | `env/common.env` | CORS origins the reverse proxy accepts |
| `BUEM_API_KEY` | `env/proxy.env` | Value callers must send as `X-Api-Key`. Local-dev placeholder, rotate before any real deployment |

!!! note "buem-gateway is not reachable from the host"
    Neither `buem-gateway` nor `buem-model` publishes a port. The reverse proxy is the only way in, the same pattern as [ignis](https://github.com/THD-Spatial-AI/ignis).

## The `building-simulation` namespace

`docker-compose.yml` declares `name: building-simulation`, the same project name the standalone `ignis` repo's compose file uses. Bringing both stacks up, from their own repos and independently, puts every container on the same `building-simulation_default` Docker network. This is purely for co-location: grouping the two services conceptually and avoiding host port collisions. **Nothing on either side calls across it.** buem-gateway does not reach ignis, and ignis does not reach buem-gateway.

!!! warning "Do not share this project name with anything else"
    Compose tracks ownership by `(project name, service key)`, not `container_name`. Sharing `building-simulation` with a compose file that happens to reuse a service key, `buem-model` for instance, will cause `docker compose up` in one repo to silently recreate the other's container using its own definition. This happened once during development against `simulation-engine`'s bundled deployment; that deployment intentionally does **not** share this namespace as a result.

## Weather data

Weather is supplied per request in the payload's `buem.weather` block (`index` timestamps plus `T`/`GHI`/`DHI`/`DNI` variables), not read from a mounted archive. See [API reference: Weather is required](api.md#weather-is-required) for the exact shape and validation rules.

!!! info "No more mounted MERRA-2 archive or synthetic fallback"
    Earlier versions mounted a MERRA-2 archive into `buem-model` (`BUEM_WEATHER_DIR_HOST`) and fell back to synthetic data when a file was missing. buem-gateway now rejects any request missing `buem.weather` with a `400` before it reaches BuEM (see `internal/buem/weather_validate.go`), so no server-side weather resolution happens here at all: the caller must supply a pre-resolved timeseries (see `enerplanet/buem#10`).

`testdata/test_buem_buildings_request.json` is a two-building fixture for `POST /api/v1/buem/buildings` (Germany, one SFH, one MFH, full envelope and thermal data) usable as an envelope-structure template, but it does not include a `weather` block, so posting it as-is now gets every building its own `400`-equivalent `error` entry, not a result.

## Deployment

!!! danger "Do not expose buem-gateway or buem-model directly"
    Neither has authentication of its own. `buem-reverse-proxy` (Caddy, `X-Api-Key`) is the only intended entry point, the same model as ignis. Run the stack on a private network with only the reverse proxy's port published.

For a shared host running both `ignis` and `buem-gateway`, bring each stack up from its own repo independently, as described in [The `building-simulation` namespace](#the-building-simulation-namespace). `HOST_HTTPS_PORT` must differ between the two (ignis defaults to `443`, buem-gateway to `8443`) since both publish through Caddy on the same host.

Use `docker-compose.prod.yml`, which pulls published images and needs no source tree on the target machine. Copy across: the compose file, `.env`, the `env/` directory, and the `caddy/` directory.

### 1. Prepare the `env/` files

!!! danger "Do not deploy the committed env/ files"
    Change both of the following before starting the stack:

    - `BUEM_API_KEY` (`env/proxy.env`) to a rotated value
    - `ALLOWED_ORIGINS` (`env/common.env`) to the real caller origins, or unset for server-to-server only

### 2. Prepare `.env`

`CADDY_DATA_DIR` is required. `APP_PORT` defaults to `8080`. Set `BUEM_IMAGE_TAG` to pin a release rather than tracking `latest`.

!!! warning "HOST_HTTPS_PORT must be 443 for a real domain"
    The `8443` default above is for the local, self-signed trust model in [Try it out](#try-it-out-no-caddy-setup) and [Local dev](#local-dev-building-from-source), where port choice is arbitrary since nothing validates it against a real certificate authority. Caddy's default ACME challenge (TLS-ALPN-01) validates against port 443 specifically, so a real domain needs `HOST_HTTPS_PORT=443`. That means buem-gateway and ignis cannot both terminate real, publicly-trusted TLS on the same host/IP at the same time, only one can hold port 443. Running them on separate hosts, or behind a single shared front proxy, avoids this; neither is set up here.

### 3. Set the site address

!!! warning "Set BUEM_SITE_ADDRESS before deploying"
    It defaults to `localhost`. Set it in `env/proxy.env` to the deployment's real domain, or Caddy will neither serve it nor provision a certificate for it.

### 4. Pull and start

```bash
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

### 5. Verify

```bash
curl -s -o /dev/null -w '%{http_code}\n' https://your-domain/buem/health
curl -s -o /dev/null -w '%{http_code}\n' https://your-domain/
curl -s -o /dev/null -w '%{http_code}\n' -H "X-Api-Key: your-key" https://your-domain/
```

Expect `200`, `403`, then a response from the app. A `200` on the second call means the API key gate is not working, and the deployment should be stopped.
