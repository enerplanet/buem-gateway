---
audience: developer
---

# Getting started

## Prerequisites

| Dependency | Version |
|---|---|
| Docker + Compose plugin | any recent |
| Go | 1.26+ (only needed to build or test outside Docker) |
| Caddy (host) | only for a deployment with a trusted certificate, see [Deployment](#deployment) |

## Choose how to run it

`environment/docker-compose.yml` runs every variant. Run each command from `environment/`. None needs a `.env` file and none checks a credential.

| Command | What runs | Reach it at |
|---|---|---|
| `docker compose up -d --pull always --no-build` | the published `buem-gateway` and `buem-model` images | `http://localhost:8081` |
| `docker compose --profile https up -d --pull always --no-build` | the same, plus Caddy terminating TLS | `https://localhost:8443` |
| `docker compose up -d --build` | `buem-gateway` built from this checkout, the published `buem-model` | `http://localhost:8081` |

TLS is a deployment choice, not a property of the service. Where transport security and access control sit upstream, leave the `https` profile off; that is also the usual choice for local development. `COMPOSE_PROFILES=https` in `.env` turns it on for every command.

!!! danger "Nothing in any variant authenticates a caller"
    There is no API key and no other credential. What restricts who can reach the service is the network it is published on. See [`SECURITY.md`](https://github.com/enerplanet/buem-gateway/blob/main/SECURITY.md).

## Try it out

Pre-built images from GHCR, so no Go toolchain, no conda and no local Caddy install:

```bash
cd environment
docker compose up -d --pull always --no-build
curl -s http://localhost:8081/buem/health
```

The port is published on loopback, so the service answers only on the local machine. Set `HOST_BIND=0.0.0.0` in a `.env` only where something upstream controls access and has to reach the container from another host.

For TLS as well:

```bash
docker compose --profile https up -d --pull always --no-build
curl -sk https://localhost:8443/buem/health
```

!!! warning "The bundled certificate is not trusted"
    Unless `CADDY_DATA_DIR` is set, Caddy's certificate authority lives in a Docker-managed volume and is not added to the operating system or browser trust store. `https://localhost:8443` shows an untrusted-certificate warning. Pass `-k` or `--no-check-certificate`, or click through it in the browser. [Deployment](#deployment) covers a real trust chain.

## Local dev (building from source)

To run a local code change to `buem-gateway`:

```bash
cd environment
docker compose up -d --build
```

This builds the connector from this source tree and pulls `buem-model` from `ghcr.io/enerplanet/buem-model`, which `enerplanet/buem` builds and publishes. The compose file defaults to `buem-model` 6.10.0, which supports output selection, all 37 equipment ids and an inline electricity profile, reports its release and commit in its health and in each result, and applies an 18 to 21 degC comfort band when a request omits `comfortT_lb` and `comfortT_ub`; set `BUEM_MODEL_IMAGE_TAG` to pin another version.

!!! warning "Pass `--build` or `--no-build`"
    The connector service names both an image and a build. Pass `--build` to run this checkout, or `--no-build` to run the published image, so it is never ambiguous which one is running. A local build is tagged with `BUEM_GATEWAY_IMAGE_TAG` (default `latest`); set it to something else to keep a published image of that tag intact.

!!! warning "buem-model 6.4.0 or later"
    Older `buem-model` images write a `.json.gz` file inside their container for every building whose hourly series is requested, and nothing removes it. buem-gateway no longer shares a volume with `buem-model` to delete those files, so pair it with 6.4.0 or later.

The dockerfile stays at `environment/gateway.dockerfile`. CI builds the published image from that one path.

| Variable | File | Purpose |
|---|---|---|
| `COMPOSE_PROFILES` | `environment/.env` | `https` adds Caddy to every command |
| `BUEM_GATEWAY_IMAGE_TAG` | `environment/.env` | `buem-gateway` release that `--no-build` runs, and the tag a local build gets (default `latest`) |
| `BUEM_MODEL_IMAGE_TAG` | `environment/.env` | `buem-model` release (default `6.10.0`) |
| `HOST_BIND` | `environment/.env` | Host interface the gateway is published on (default `127.0.0.1`) |
| `HOST_PORT` | `environment/.env` | Host port the gateway is published on (default `8081`. Not `8080`: the platform's Keycloak holds that, and ignis holds `8088`) |
| `APP_PORT` | `environment/.env` | Internal port `buem-gateway` listens on |
| `HOST_HTTPS_PORT` | `environment/.env` | `https` profile: host port Caddy publishes (default `8443`, not `443`, so it does not collide with ignis's own reverse proxy on the same host) |
| `BUEM_SITE_ADDRESS` | `environment/.env` | `https` profile: domain Caddy serves and provisions a certificate for (default `localhost`) |
| `CADDY_DATA_DIR` | `environment/.env` | `https` profile: host path to Caddy's trusted local CA. Run `caddy trust` once on the host, then point this at where that created the CA |
| `ALLOWED_ORIGINS` | `environment/env/common.env` | CORS origins accepted from browser pages |

!!! note "buem-model is never reachable from the host"
    The `model` service publishes no port and is called only by `api`, by service name on the compose network.

## Compose project and service names

The compose project is `buem-gateway`. A compose project is the unit compose acts on destructively, for example with `--remove-orphans`, so it shares a name with no other repository.

| Service | Runs | Profile |
|---|---|---|
| `api` | the `buem-gateway` connector | always |
| `model` | `buem-model` | always |
| `proxy` | Caddy, terminating TLS | `https` |

Compose names the containers `<project>-<service>-<n>`, for example `buem-gateway-api-1`. Address a container through compose from `environment/`, for example `docker compose logs api`, rather than by container name.

Other containers reach the connector on `tentacron-net` by the alias `buem-gateway`. The alias `buem-gateway-http` answers too, for callers that have not moved to `buem-gateway` yet. Never address `api`: the API services of other repositories on `tentacron-net` share that name, so it resolves to any of them.

!!! warning "One stack per host on tentacron-net"
    Every stack started from this repository joins `tentacron-net` under the same aliases. Two of them on one host would split requests between the two stacks without any error. Run a second stack, for example for testing, under its own project name and without `tentacron-net`.

!!! info "Containers from earlier versions of the compose files"
    Earlier versions ran as the projects `buem-gateway-http` and `buem-gateway-https`, from `environment/http` and `environment/https`. Compose here does not see those containers. Stop them with `docker compose -p buem-gateway-http down` (or `-p buem-gateway-https`); both project names belonged to this repository only.

    Versions before those fixed the container names to `buem-gateway`, `buem-model` and `buem-reverse-proxy` under the project `building-simulation`. Remove those containers by name:

    ```bash
    docker stop buem-gateway buem-model buem-reverse-proxy
    docker rm   buem-gateway buem-model buem-reverse-proxy
    ```

    `stop` before `rm`, not `rm -f`, so the connector finishes the requests in flight instead of being killed mid-run.

    Do **not** use `docker compose -p building-simulation down`. That project was shared with [ignis](https://github.com/THD-Spatial-AI/ignis), so it also removes `ignis-app`, `ignis-db` and `ignis-reverse-proxy`. Volumes are untouched either way.

!!! info "Volumes from earlier releases"
    buem-gateway and `buem-model` share no volume. Earlier releases mounted `building-simulation_buem-csv-data` (CSV output, before v7.0.0) and `building-simulation_buem-results-data` (intermediate files, before v7.1.0). Nothing mounts either any more; remove them with `docker volume rm building-simulation_buem-csv-data building-simulation_buem-results-data` once you no longer need their contents.

    `caddy-data` holds Caddy's self-signed authority when `CADDY_DATA_DIR` is unset. It is never added to a trust store, so losing it costs one click through a certificate warning.

## Weather data

Weather is supplied per request in the payload's `buem.weather` block (`index` timestamps plus `T`/`GHI`/`DHI`/`DNI` variables), not read from a mounted archive. See [API reference: Weather is required](api.md#weather-is-required) for the exact shape and validation rules. buem-gateway rejects any request missing `buem.weather` with a `400` before it reaches BuEM (see `internal/buem/weather_validate.go`).

!!! info "No mounted archive, no weather service"
    The compose file sets `BUEM_WEATHER_FALLBACK=false` on `buem-model`. That makes a request missing `buem.weather` fail loudly rather than have BuEM resolve its own, and it also skips the default-location timeseries `buem-model` would otherwise fetch when its config module loads. The container boots with no weather data of any kind.

`testdata/test_buem_buildings_request.json` is a two-building fixture for `POST /api/v1/buem/buildings` (Germany, one SFH, one MFH, full envelope and thermal data) usable as an envelope-structure template. It has no `weather` block, so posting it unchanged returns an `error` entry for every building.

## Deployment

!!! danger "Neither service authenticates anything"
    Run the stack only on a network that already controls who can reach it. `buem-model` publishes no port; `buem-gateway` publishes one on `HOST_BIND`, and Caddy publishes one under the `https` profile. None checks a credential.

For a shared host running both `ignis` and `buem-gateway`, bring each stack up from its own repository independently. They share nothing but the host, so the only thing that has to differ is the ports they publish. If both terminate TLS, `HOST_HTTPS_PORT` must differ between them.

### Behind an upstream proxy

If the platform's own reverse proxy or a firewall already terminates TLS, run without the `https` profile and let that upstream handle transport security. Set `HOST_BIND` so the upstream can reach the container, and make sure nothing else on that network can.

Copy across `environment/docker-compose.yml`, a `.env` if you changed anything, and `environment/env/`, then start it with `docker compose up -d --pull always --no-build`.

### Terminating TLS here

Run the published images under the `https` profile; no source tree is needed on the target machine. Copy across: `environment/docker-compose.yml`, `.env`, the `caddy/` directory, and `environment/env/`.

#### 1. Prepare the env files

!!! danger "Do not deploy the committed env file"
    Set `ALLOWED_ORIGINS` (`environment/env/common.env`) to the real caller origins, or leave it unset for server-to-server callers, which send no `Origin` header and ignore the response headers anyway.

#### 2. Prepare `.env`

Set `COMPOSE_PROFILES=https`, `BUEM_SITE_ADDRESS` to the domain, and `CADDY_DATA_DIR` for a host-trusted local CA. Unset, `BUEM_SITE_ADDRESS` serves `localhost` and `CADDY_DATA_DIR` falls back to a self-signed authority, so neither fails loudly: check both before starting. `APP_PORT` defaults to `8080`. Set `BUEM_GATEWAY_IMAGE_TAG` and `BUEM_MODEL_IMAGE_TAG` to pin releases rather than tracking `latest`. The two images are versioned independently, so pin each to its own release.

!!! warning "HOST_HTTPS_PORT must be 443 for a real domain"
    Caddy's default ACME challenge (TLS-ALPN-01) validates against port 443 specifically, so a real domain needs `HOST_HTTPS_PORT=443` (the default is `8443`). That means buem-gateway and ignis cannot both terminate publicly trusted TLS on the same host and IP: only one can hold port 443. Running them on separate hosts, or behind a single shared front proxy, avoids this; neither is set up here.

#### 3. Pull and start

```bash
cd environment
docker compose pull
docker compose up -d --no-build
```

#### 4. Verify

```bash
curl -s -o /dev/null -w '%{http_code}\n' https://your-domain/buem/health
```

Expect `200`. A connection error usually means the certificate was not provisioned, which `docker compose logs proxy` will show.
