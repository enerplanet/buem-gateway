# Getting started

## Prerequisites

| Dependency | Version |
|---|---|
| Docker + Compose plugin | any recent |
| Go | 1.26+ (only needed to build or test outside Docker) |
| Caddy (host) | only for a deployment with a trusted certificate, see [Deployment](#deployment) |

## Choose an environment

`environment/` holds two, and you pick one. Neither needs a `.env` file and neither checks a credential.

| Directory | What runs | Reach it at |
|---|---|---|
| `environment/http` | `buem-gateway` and `buem-model` | `http://localhost:8080` |
| `environment/https` | the same, plus Caddy terminating TLS | `https://localhost:8443` |

TLS is a deployment choice rather than a property of this service. In the deployment these are written for, transport security and access control sit upstream, so `environment/http` is the one local development usually wants.

!!! danger "Nothing in either environment authenticates a caller"
    There is no API key and no other credential. What restricts who can reach the service is the network it is published on. See [`SECURITY.md`](https://github.com/enerplanet/buem-gateway/blob/main/SECURITY.md).

## Try it out

Pre-built images from GHCR, so no Go toolchain, no conda and no local Caddy install:

```bash
cd environment/http
docker compose up -d
curl -s http://localhost:8080/buem/health
```

The port is published on loopback, so the service answers on your machine and nowhere else. Set `HOST_BIND=0.0.0.0` in a `.env` only where something upstream controls access and has to reach the container from another host.

For TLS instead:

```bash
cd environment/https
docker compose up -d
curl -sk https://localhost:8443/buem/health
```

!!! warning "The bundled certificate is not trusted"
    Caddy's certificate authority lives in a Docker-managed volume here, so it is never added to your operating system or browser trust store. `https://localhost:8443` shows an untrusted-certificate warning: expected, not a bug. Pass `-k` or `--no-check-certificate`, or click through it in the browser. [Deployment](#deployment) covers a real trust chain.

## Local dev (building from source)

To run a local code change to `buem-gateway`:

```bash
cd environment/http
docker compose -f docker-compose.build.yml up -d --build
```

This builds the connector from this source tree and pulls `buem-model` from `ghcr.io/enerplanet/buem-model`, which `enerplanet/buem` builds and publishes. Set `BUEM_IMAGE_TAG` to pin a version.

The dockerfile stays at `environment/gateway.dockerfile` rather than being copied into either directory. CI builds the published image from that one path, and the image is identical whichever transport it runs behind.

| Variable | File | Purpose |
|---|---|---|
| `HOST_BIND` | `http/.env` | Host interface the gateway is published on (default `127.0.0.1`) |
| `HOST_PORT` | `http/.env` | Host port the gateway is published on (default `8080`) |
| `HOST_HTTPS_PORT` | `https/.env` | Host port the reverse proxy publishes (default `8443`, not `443`, so it does not collide with ignis's own reverse proxy on the same host) |
| `APP_PORT` | either `.env` | Internal port `buem-gateway` listens on |
| `BUEM_SITE_ADDRESS` | `https/.env` | Domain Caddy serves and provisions a certificate for |
| `CADDY_DATA_DIR` | `https/.env` | Host path to Caddy's trusted local CA. Run `caddy trust` once on the host, then point this at where that created the CA |
| `ALLOWED_ORIGINS` | `env/common.env` | CORS origins accepted from browser pages. Shared by both environments |

!!! note "buem-model is never reachable from the host"
    It publishes no port in either environment and is called only by `buem-gateway`, by service name on the compose network. `buem-gateway` itself does publish a port in `environment/http`; that is the point of that directory.

## The `building-simulation` namespace

Every compose file here declares `name: building-simulation`, the same project name the standalone `ignis` repo's compose file uses. Bringing both stacks up, from their own repos and independently, puts every container on the same `building-simulation_default` Docker network. This is purely for co-location: grouping the two services conceptually and avoiding host port collisions. **Nothing on either side calls across it.** buem-gateway does not reach ignis, and ignis does not reach buem-gateway.

!!! warning "Do not share this project name with anything else"
    Compose tracks ownership by `(project name, service key)`, not `container_name`. Sharing `building-simulation` with a compose file that happens to reuse a service key, `buem-model` for instance, will cause `docker compose up` in one repo to silently recreate the other's container using its own definition. This happened once during development against `simulation-engine`'s bundled deployment; that deployment intentionally does **not** share this namespace as a result.

## Weather data

Weather is supplied per request in the payload's `buem.weather` block (`index` timestamps plus `T`/`GHI`/`DHI`/`DNI` variables), not read from a mounted archive. See [API reference: Weather is required](api.md#weather-is-required) for the exact shape and validation rules. buem-gateway rejects any request missing `buem.weather` with a `400` before it reaches BuEM (see `internal/buem/weather_validate.go`).

!!! info "No mounted archive, no weather service"
    Every compose file sets `BUEM_WEATHER_FALLBACK=false` on `buem-model`. That makes a request missing `buem.weather` fail loudly rather than have BuEM resolve its own, and it also skips the default-location timeseries `buem-model` would otherwise fetch when its config module loads. The container boots with no weather data of any kind.

`testdata/test_buem_buildings_request.json` is a two-building fixture for `POST /api/v1/buem/buildings` (Germany, one SFH, one MFH, full envelope and thermal data) usable as an envelope-structure template, but it does not include a `weather` block, so posting it as-is now gets every building its own `400`-equivalent `error` entry, not a result.

## Deployment

!!! danger "Neither service authenticates anything"
    Run the stack only on a network that already controls who can reach it. `buem-model` publishes no port in any environment; `buem-gateway` publishes one in `environment/http`, and Caddy publishes one in `environment/https`. Neither checks a credential.

For a shared host running both `ignis` and `buem-gateway`, bring each stack up from its own repo independently, as described in [The `building-simulation` namespace](#the-building-simulation-namespace). If both terminate TLS, `HOST_HTTPS_PORT` must differ between them.

### Behind an upstream proxy

If the platform's own reverse proxy or a firewall already terminates TLS, deploy `environment/http` and let that upstream handle transport security. Set `HOST_BIND` so the upstream can reach the container, and make sure nothing else on that network can.

Copy across `environment/http/docker-compose.yml`, a `.env` if you changed anything, and `environment/env/`, which sits one level up and is shared with the TLS environment.

### Terminating TLS here

Use `environment/https/docker-compose.prod.yml`, which pulls published images and needs no source tree on the target machine. Copy across: the compose file, `.env`, the `caddy/` directory, and `environment/env/`.

#### 1. Prepare the env files

!!! danger "Do not deploy the committed env file"
    Set `ALLOWED_ORIGINS` (`environment/env/common.env`) to the real caller origins, or leave it unset for server-to-server callers, which send no `Origin` header and ignore the response headers anyway.

#### 2. Prepare `.env`

`BUEM_SITE_ADDRESS` and `CADDY_DATA_DIR` are both required, and the compose file refuses to start without them rather than defaulting to something that would quietly serve the wrong thing. `APP_PORT` defaults to `8080`. Set `BUEM_IMAGE_TAG` to pin a release rather than tracking `latest`.

!!! warning "HOST_HTTPS_PORT must be 443 for a real domain"
    Caddy's default ACME challenge (TLS-ALPN-01) validates against port 443 specifically, so a real domain needs `HOST_HTTPS_PORT=443`, which is the production default. That means buem-gateway and ignis cannot both terminate publicly trusted TLS on the same host and IP: only one can hold port 443. Running them on separate hosts, or behind a single shared front proxy, avoids this; neither is set up here.

#### 3. Pull and start

```bash
cd environment/https
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

#### 4. Verify

```bash
curl -s -o /dev/null -w '%{http_code}\n' https://your-domain/buem/health
```

Expect `200`. A connection error usually means the certificate was not provisioned, which `docker compose logs buem-reverse-proxy` will show.
