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

## Choose an environment

`environment/` holds two. Neither needs a `.env` file and neither checks a credential.

| Directory | What runs | Reach it at |
|---|---|---|
| `environment/http` | `buem-gateway` and `buem-model` | `http://localhost:8081` |
| `environment/https` | the same, plus Caddy terminating TLS | `https://localhost:8443` |

TLS is a deployment choice, not a property of the service. Where transport security and access control sit upstream, use `environment/http`; it is also the usual choice for local development.

!!! danger "Nothing in either environment authenticates a caller"
    There is no API key and no other credential. What restricts who can reach the service is the network it is published on. See [`SECURITY.md`](https://github.com/enerplanet/buem-gateway/blob/main/SECURITY.md).

## Try it out

Pre-built images from GHCR, so no Go toolchain, no conda and no local Caddy install:

```bash
cd environment/http
docker compose up -d
curl -s http://localhost:8081/buem/health
```

The port is published on loopback, so the service answers only on the local machine. Set `HOST_BIND=0.0.0.0` in a `.env` only where something upstream controls access and has to reach the container from another host.

For TLS instead:

```bash
cd environment/https
docker compose up -d
curl -sk https://localhost:8443/buem/health
```

!!! warning "The bundled certificate is not trusted"
    Caddy's certificate authority lives in a Docker-managed volume and is not added to the operating system or browser trust store. `https://localhost:8443` shows an untrusted-certificate warning. Pass `-k` or `--no-check-certificate`, or click through it in the browser. [Deployment](#deployment) covers a real trust chain.

## Local dev (building from source)

To run a local code change to `buem-gateway`:

```bash
cd environment/http
docker compose -f docker-compose.build.yml up -d --build
```

This builds the connector from this source tree and pulls `buem-model` from `ghcr.io/enerplanet/buem-model`, which `enerplanet/buem` builds and publishes. The compose files default to `buem-model` 6.7.0, which supports output selection and applies an 18 to 21 degC comfort band when a request omits `comfortT_lb` and `comfortT_ub`; set `BUEM_MODEL_IMAGE_TAG` to pin another version.

!!! warning "buem-model 6.4.0 or later"
    Older `buem-model` images write a `.json.gz` file inside their container for every building whose hourly series is requested, and nothing removes it. buem-gateway no longer shares a volume with `buem-model` to delete those files, so pair it with 6.4.0 or later.

The dockerfile stays at `environment/gateway.dockerfile` rather than being copied into either directory. CI builds the published image from that one path, and the image is identical whichever transport it runs behind.

| Variable | File | Purpose |
|---|---|---|
| `HOST_BIND` | `http/.env` | Host interface the gateway is published on (default `127.0.0.1`) |
| `HOST_PORT` | `http/.env` | Host port the gateway is published on (default `8081`. Not `8080`: the platform's Keycloak holds that, and ignis's own HTTP environment holds `8088`) |
| `HOST_HTTPS_PORT` | `https/.env` | Host port the reverse proxy publishes (default `8443`, not `443`, so it does not collide with ignis's own reverse proxy on the same host) |
| `APP_PORT` | either `.env` | Internal port `buem-gateway` listens on |
| `BUEM_SITE_ADDRESS` | `https/.env` | Domain Caddy serves and provisions a certificate for |
| `CADDY_DATA_DIR` | `https/.env` | Host path to Caddy's trusted local CA. Run `caddy trust` once on the host, then point this at where that created the CA |
| `ALLOWED_ORIGINS` | `environment/env/common.env` | CORS origins accepted from browser pages. Shared by both environments |

!!! note "buem-model is never reachable from the host"
    It publishes no port in either environment and is called only by `buem-gateway`, by service name on the compose network. `buem-gateway` itself does publish a port in `environment/http`; that is the point of that directory.

## Compose project names

Each directory is its own compose project: `buem-gateway-http` and `buem-gateway-https`.

A compose project is the unit compose acts on destructively, for example with `--remove-orphans`. Sharing a project name with another repository would let a command run here remove that repository's containers, so these projects share a name with nothing else. The two transports are separate projects for the same reason: their containers are alternatives, and neither may act on the other's.

A compose project's default network is named after the project. A service outside this repository that needs to reach buem-gateway attaches to a purpose-named network declared for that job, not to `buem-gateway-http_default` or `buem-gateway-https_default`, which change if the project is renamed.

Compose names the containers after the project, for example `buem-gateway-http-buem-gateway-1`, so both transports can run at the same time on one host: they are separate projects on different host ports (8081 and 8443). Address a container through compose from its directory, for example `docker compose logs buem-gateway` or `docker compose stop buem-gateway`, rather than by container name. Other containers on a shared network reach the connector by its service name `buem-gateway`, or by the alias `buem-gateway-http` on `tentacron-net`.

!!! warning "One HTTP stack per host on tentacron-net"
    Every HTTP stack started from this repository joins `tentacron-net` under the same alias, `buem-gateway-http`. Two of them on one host would split requests between the two stacks without any error. Run a second HTTP stack, for example for testing, under its own project name and without `tentacron-net`.

!!! info "Containers from earlier versions of these compose files"
    Earlier versions of these compose files fixed the container names to `buem-gateway`, `buem-model` and `buem-reverse-proxy`. A stack started from this repository under its current project name is replaced by the next `docker compose up`. A stack started under the old `building-simulation` project name is invisible to compose here. Remove its containers by name:

    ```bash
    docker stop buem-gateway buem-model buem-reverse-proxy
    docker rm   buem-gateway buem-model buem-reverse-proxy
    ```

    `stop` before `rm`, not `rm -f`, so the connector finishes the requests in flight instead of being killed mid-run.

    Do **not** use `docker compose -p building-simulation down`. That project was shared with [ignis](https://github.com/THD-Spatial-AI/ignis), so it also removes `ignis-app`, `ignis-db` and `ignis-reverse-proxy`. Volumes are untouched either way.

!!! info "Volumes from earlier releases"
    buem-gateway and `buem-model` share no volume. Earlier releases mounted `building-simulation_buem-csv-data` (CSV output, before v7.0.0) and `building-simulation_buem-results-data` (intermediate files, before v7.1.0). Nothing mounts either any more; remove them with `docker volume rm building-simulation_buem-csv-data building-simulation_buem-results-data` once you no longer need their contents.

    `caddy-data` is not pinned. It holds a self-signed authority that is never added to a trust store, so losing it costs one click through a certificate warning.

!!! warning "Compose warns about the pinned volumes. Ignore the suggested fix"
    `docker compose up` reports that each pinned volume was created for a different project and suggests `external: true`. Do not apply it: an external volume must exist before `up`, so a clean checkout would fail to start. The warning appears whenever one volume serves two projects and is expected after the first transport switch.

## Weather data

Weather is supplied per request in the payload's `buem.weather` block (`index` timestamps plus `T`/`GHI`/`DHI`/`DNI` variables), not read from a mounted archive. See [API reference: Weather is required](api.md#weather-is-required) for the exact shape and validation rules. buem-gateway rejects any request missing `buem.weather` with a `400` before it reaches BuEM (see `internal/buem/weather_validate.go`).

!!! info "No mounted archive, no weather service"
    Every compose file sets `BUEM_WEATHER_FALLBACK=false` on `buem-model`. That makes a request missing `buem.weather` fail loudly rather than have BuEM resolve its own, and it also skips the default-location timeseries `buem-model` would otherwise fetch when its config module loads. The container boots with no weather data of any kind.

`testdata/test_buem_buildings_request.json` is a two-building fixture for `POST /api/v1/buem/buildings` (Germany, one SFH, one MFH, full envelope and thermal data) usable as an envelope-structure template. It has no `weather` block, so posting it unchanged returns an `error` entry for every building.

## Deployment

!!! danger "Neither service authenticates anything"
    Run the stack only on a network that already controls who can reach it. `buem-model` publishes no port in any environment; `buem-gateway` publishes one in `environment/http`, and Caddy publishes one in `environment/https`. Neither checks a credential.

For a shared host running both `ignis` and `buem-gateway`, bring each stack up from its own repository independently. They share nothing but the host, so the only thing that has to differ is the ports they publish. If both terminate TLS, `HOST_HTTPS_PORT` must differ between them.

### Behind an upstream proxy

If the platform's own reverse proxy or a firewall already terminates TLS, deploy `environment/http` and let that upstream handle transport security. Set `HOST_BIND` so the upstream can reach the container, and make sure nothing else on that network can.

Copy across `environment/http/docker-compose.yml`, a `.env` if you changed anything, and `environment/env/`, which sits one level up and is shared with the TLS environment.

### Terminating TLS here

Use `environment/https/docker-compose.prod.yml`, which pulls published images and needs no source tree on the target machine. Copy across: the compose file, `.env`, the `caddy/` directory, and `environment/env/`.

#### 1. Prepare the env files

!!! danger "Do not deploy the committed env file"
    Set `ALLOWED_ORIGINS` (`environment/env/common.env`) to the real caller origins, or leave it unset for server-to-server callers, which send no `Origin` header and ignore the response headers anyway.

#### 2. Prepare `.env`

`BUEM_SITE_ADDRESS` and `CADDY_DATA_DIR` are both required, and the compose file refuses to start without them rather than defaulting to something that would quietly serve the wrong thing. `APP_PORT` defaults to `8080`. Set `BUEM_GATEWAY_IMAGE_TAG` and `BUEM_MODEL_IMAGE_TAG` to pin releases rather than tracking `latest`. The two images are versioned independently, so pin each to its own release.

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
