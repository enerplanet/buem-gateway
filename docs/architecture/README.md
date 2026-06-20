# Architecture Documentation

This directory describes the BUEM ecosystem at four levels of detail,
following **ISO/IEC/IEEE 42010** (architecture description) and the **C4 model** hierarchy.

Each level answers a different question and is written for a different audience.

---

## Documentation Map

| Level | Document | Audience | Concern |
|-------|----------|----------|---------|
| L1 — System Context | [L1-system-context.md](L1-system-context.md) | Managers, external integrators | Who uses the system; external boundaries |
| L2 — Containers | [L2-containers.md](L2-containers.md) | Architects, DevOps | Services, tech stacks, network topology |
| L3 — BUEM Components | [L3-buem-components.md](L3-buem-components.md) | BUEM developers | Internal Flask modules, thermal pipeline |
| L3 — Gateway Components | [L3-gateway-components.md](L3-gateway-components.md) | Gateway / platform developers | Simulation routing, data transformation |
| L4 — Endpoint Reference | [L4-endpoints.md](L4-endpoints.md) | API integrators, frontend developers | All HTTP endpoints, request / response shapes |

---

## Reading Order

```
L1  (blackbox)
 └── L2  (system whitebox — services)
      ├── L3-buem    (BUEM whitebox — modules)
      ├── L3-gateway (gateway whitebox — routing)
      └── L4  (interface — HTTP endpoints)
```

Start at L1 for a plain-language overview. Go deeper only for the service you are working on.

---

## Related Documents

| Document | Location | Purpose |
|----------|----------|---------|
| Integration architecture | [../integration-architecture.md](../integration-architecture.md) | Request flow, data transformation, concurrency model |
| Schema overview | [../../SCHEMA_OVERVIEW.md](../../SCHEMA_OVERVIEW.md) | Request / response field reference |
| Versioning policy | [../../VERSIONING.md](../../VERSIONING.md) | How schema versions are managed |
| Changelog | [../../CHANGELOG.md](../../CHANGELOG.md) | History of schema changes |
