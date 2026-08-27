# buem-gateway

[![Go](https://github.com/enerplanet/buem-gateway/actions/workflows/go.yml/badge.svg)](https://github.com/enerplanet/buem-gateway/actions/workflows/go.yml)
[![CI](https://github.com/enerplanet/buem-gateway/actions/workflows/ci.yml/badge.svg)](https://github.com/enerplanet/buem-gateway/actions/workflows/ci.yml)
[![MkDocs](https://github.com/enerplanet/buem-gateway/actions/workflows/docs.yml/badge.svg)](https://enerplanet.github.io/buem-gateway)
[![codecov](https://codecov.io/gh/enerplanet/buem-gateway/branch/main/graph/badge.svg)](https://codecov.io/gh/enerplanet/buem-gateway)
[![Release](https://img.shields.io/github/v/release/enerplanet/buem-gateway?include_prereleases&label=release&logo=github)](https://github.com/enerplanet/buem-gateway/releases)

Go connector between EnerPlanET and [BuEM](https://github.com/UU-BUEM/buem), the ISO 52016-1 thermal building model.

buem-gateway accepts a single building or a flat list of buildings, forwards each to BuEM, writes the resulting heating, cooling, and electricity load profiles to CSV, and returns one result per building.

It is a standalone service with its own container and reverse proxy. It does not require `enerplanet/simulation-engine`.

## Scope

This repository holds two things:

- The connector itself.
- The authoritative JSON schema contract defining BuEM's request and response format (`schemas/`) and the CSV output naming convention ([`docs/naming.md`](docs/naming.md)).

The schema-contract version and buem-gateway's release version are numbered independently. See [`docs/versioning.md`](docs/versioning.md) and [`CHANGELOG.md`](CHANGELOG.md).

## Architecture

```mermaid
sequenceDiagram
    autonumber
    participant Caller as Caller<br/>e.g. EnerPlanET backend
    participant Proxy as buem-reverse-proxy<br/>Caddy, X-Api-Key auth
    participant App as buem-gateway<br/>Go connector
    participant Model as buem-model<br/>BuEM Flask
    participant Vol as shared volume

    Caller->>Proxy: POST /api/v1/buem/buildings<br/>buildings list + shared weather
    Proxy->>App: Forward request
    App->>Model: POST /api/process<br/>one call per building
    Model-->>App: thermal_load_profile
    App->>Vol: write heating/cooling/electricity CSVs
    App-->>Proxy: results list<br/>buem or error per building
    Proxy-->>Caller: results list<br/>buem or error per building
```

Each building carries its own `building` block containing its envelope. One `weather` block is shared across the request. Buildings run concurrently, bounded by `MAX_CONCURRENT_SIMS`, and each returns its own `buem` result or `error` independently of the others.

buem-gateway has no concept of a grid or topology. A caller holding one resolves it to a flat list before calling.

## Endpoints

## Endpoints

| Method | Path | Request | Response |
| --- | --- | --- | --- |
| `GET` | `/buem/health` | None | Service status |
| `POST` | `/api/v1/buem/building` | One building with geometry, envelope, and weather | `buem` block with load profile and model metadata |
| `POST` | `/api/v1/buem/buildings` | Building list, each with geometry and envelope, plus one shared weather block | One result per building, in request order |
| `POST` | `/api/v1/buem/validate` | Same body as `/building` | Whether the request is well-formed. BuEM is not called |

All routes except `/buem/health` require the `X-Api-Key` header, checked by the reverse proxy.

Full reference: [`docs/api.md`](docs/api.md).

## Compatibility

| Dependency | Version |
| --- | --- |
| Go | 1.26+ |
| Docker with Compose plugin | any recent |

## Quick start

Pre-built images, no Go toolchain and no local Caddy install required:

```bash
cd environment
docker compose -f docker-compose.quickstart.yml up -d
curl -sk https://localhost:8443/buem/health
```

No `.env` file is needed; every value has a default. Without `caddy trust`, `https://localhost:8443` presents an untrusted certificate, so `curl -k` is required. See [`docs/getting-started.md`](docs/getting-started.md#try-it-out-no-caddy-setup) for how this differs from a real deployment.

## Building from source

| Step | Command | Description |
| --- | --- | --- |
| 1 | `cd environment && cp .env.example .env` | Configure `CADDY_DATA_DIR`, ports, weather data path |
| 2 | `docker compose up -d --build` | Start `buem-model`, `buem-gateway`, and `buem-reverse-proxy` |
| 3 | `curl -sk https://localhost:8443/buem/health -H "X-Api-Key: <BUEM_API_KEY>"` | Confirm the stack is up |

Full setup and deployment details: [`docs/getting-started.md`](docs/getting-started.md).

## Testing

```bash
go build ./...
go vet ./...
go test ./...
```

With coverage:

```bash
go test ./... -coverprofile=coverage.out -covermode=atomic
go tool cover -html=coverage.out
```

## Local docs

```bash
python -m venv .venv
```

```bash
.venv/bin/pip install -r docs/requirements.txt
.venv/bin/mkdocs serve
```

## Licence

MIT Licence, Copyright 2026 BigGeoData & Spatial AI, Technische Hochschule Deggendorf. See [LICENSE](LICENSE) for the full text and [ATTRIBUTIONS.md](ATTRIBUTIONS.md) for third-party attributions.

Security issues: see [SECURITY.md](SECURITY.md) for private reporting.

## Acknowledgements

Developed in the context of the RENvolveIT research project (<https://projekte.ffg.at/projekt/5127011>), funded by CETPartnership under the 2023 joint call for research proposals, co-funded by the European Commission (GA N°101069750).

<img src="docs/assets/sponsors/CETP-logo.svg" alt="CETPartnership" width="144" height="72">
<img src="docs/assets/sponsors/EN_Co-fundedbytheEU_RGB_POS.png" alt="EU" width="180" height="40">