# buem-gateway

[![Go](https://github.com/enerplanet/buem-gateway/actions/workflows/go.yml/badge.svg)](https://github.com/enerplanet/buem-gateway/actions/workflows/go.yml)&nbsp;&nbsp;&nbsp;[![codecov](https://codecov.io/gh/enerplanet/buem-gateway/branch/main/graph/badge.svg)](https://codecov.io/gh/enerplanet/buem-gateway)

Standalone connector between EnerPlanET and [BuEM](https://github.com/enerplanet/buem), the ISO 52016-1 thermal building model.

It also carries the JSON schema contract that defines BuEM's request and response shape (`schemas/`, `CHANGELOG.md`, `VERSIONING.md`), so this repository is both the connector and the authoritative source for that contract.

!!! info "Not the same thing as simulation-engine"
    `enerplanet/simulation-engine` bundles its own BuEM deployment. This is a separate repo with its own container and its own reverse proxy, and nothing here requires simulation-engine to be installed or running.

## Sequence Diagram

Following sequence diagram illustrates the interaction between the components:

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
    App->>Caller: one result per building
    App->>Vol: write heating/cooling/electricity CSVs
```

A request is a flat list of buildings, each carrying its own `building` block (envelope etc.), plus one `weather` block shared across the whole request. Buildings are run through BuEM concurrently, bounded by `MAX_CONCURRENT_SIMS`; each comes back with its own `buem` result or `error`, independent of the others. buem-gateway has no concept of a grid or topology — a caller with one resolves it down to this flat list itself.

## Documentation

| Section | Description |
|---|---|
| [Getting started](getting-started.md) | Local dev, deployment, reproducibility check |
| [API reference](api.md) | Input and output contract, auth, example request and response |

## Repository

[github.com/enerplanet/buem-gateway](https://github.com/enerplanet/buem-gateway) ·
[Issue tracker](https://github.com/enerplanet/buem-gateway/issues)