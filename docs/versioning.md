# BUEM–EnerPlanET API contract versioning

This repo is the single source of truth for the request and response JSON
exchanged between EnerPlanET and BUEM. The current contract is **API contract
v5**, defined by the files in `schemas/v5/`.

## Three version lines, none related to each other

| Line | Where it lives | Example |
|---|---|---|
| API contract version | `schemas/`, this file, `CHANGELOG.md`, `openapi.yaml` `info.version` | v5 (v5.0.0) |
| buem-gateway software version | git tags on this repo | v6.0.0 |
| buem-model software version | git tags on the `buem` repo | v5.0.x |

A match between any two of them is coincidence. Always write the contract
version qualified — "API contract v5", "schema v5" — never a bare "v5".

## Directory layout is the version status

| Path | Meaning |
|---|---|
| `schemas/v5/` | Current production contract. **Frozen** — never edited after it was cut. |
| `schemas/v6-draft/` | Next version, under development. Inert: nothing loads, embeds, or validates against it. Edit freely. |
| `schemas/v1/` … `schemas/v4/` | Archived past versions, read-only. |

There is no mutable "current" directory. To know which contract is current, read
this file or look for the `schemas/vN/` folder with no `-draft` suffix. To know
which contract a running buem-gateway enforces, call `GET /buem/health` — it
reports `contract_version`.

## How the contract is enforced

buem-gateway has no external Go dependencies and validates by hand, the same way
MEME does: `requireEnvelope` and `requireWeather` (`internal/buem/`) check the
parts of `schemas/v5/request_schema.json` a request must satisfy before BuEM is
called — `building.envelope.elements` non-empty, `buem.weather` present with an
`index` and at least one of T/GHI/DNI/DHI. A request that fails gets a 400 naming
the field; there is no separate "wrong version" error.

Schema-versus-code drift is caught two ways, not by making the schema the runtime
check:

- `python scripts/validate_schemas.py` validates `schemas/v5/example_request.json`
  against `schemas/v5/request_schema.json` (run in CI).
- A Go test runs `requireEnvelope`/`requireWeather` against that same example and
  against envelope-less / weather-less variants, so the hand-written checks stay
  tied to the frozen schema's own example.

When the schema changes, update both the schema file and the hand-written checks
in the same change; the tests above fail if they disagree.

`internal/buem/types.go`'s `APIVersion` constant records which contract version
those checks target, for the health response.

## How buem-model consumes the contract

buem-model keeps a pinned copy of `schemas/v5/{request,response}_schema.json`
with a header pointing back here, and a CI check that fails if the copy drifts
from this repo. buem-model does not maintain its own contract definition. See
the buem repo's own `json_schema/` notes.

## Promoting v6-draft to v6

A contract bump is text and file moves, no git tag, no GitHub release:

1. Breaking or not, `schemas/v6-draft/` becomes `schemas/v6/`.
2. Set `scripts/validate_schemas.py`'s `CURRENT_VERSION_DIR` to `v6` and
   `internal/buem/types.go` `APIVersion` to `v6`. Update `requireEnvelope` /
   `requireWeather` (and any other hand-written checks) to match the new shape.
3. Add the `CHANGELOG.md` entry; update the current-version line at the top of
   this file, the released-versions table below, and `openapi.yaml`
   `info.version`.
4. Run `python scripts/validate_schemas.py` and `go test ./...` — the first
   checks the schema against its examples and the version-string sync, the second
   runs `TestValidatorsMatchV5Example` (rename per version) tying the checks to
   the example.
5. Re-sync the buem-model pinned copy.
6. Commit. A code change that ships alongside (e.g. the connector reading a new
   field) gets its own buem-gateway software git tag as a normal release.

```bash
python scripts/validate_schemas.py
```

## Released versions

| Version | Date | Status | What changed |
|---|---|---|---|
| v5.0.0 | 2026-08 | Current | `envelope` required again (v4.x TABULA fallback removed); `weather` required as a pre-resolved inline timeseries; contract frozen into `schemas/v5/` and enforced by embedded-schema validation. See `CHANGELOG.md`. |
| v4.2.0 | 2026-06 | Unsupported | Optional `model_id` on the request `FeatureCollection`, used by the gateway to namespace CSV output per model |
| v4.1.0 | 2026-04 | Unsupported | Optional `name` field on `building` and `envelope_element`, display only |
| v4.0.0 | 2026-03 | Unsupported | `solver.compute_cooling` (opt-in cooling), file-path electricity input, `envelope` optional (TABULA fallback), `phi_int`/`q_w_nd` configurable |
| v3.0.0 | 2026-03 | Unsupported | `buem` split into `building` and `solver`; per-surface thermal properties; every quantity carries its unit |
| v2.0.0 | 2026-02 | Unsupported | Structured building attributes; nested component model |
| v1.0.0 | 2025-11 | Unsupported | Initial format |

See `CHANGELOG.md` for full detail on each version.
