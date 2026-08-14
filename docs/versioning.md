# BUEM–EnerPlanET Schema Versioning

This file tracks the version of the request and response JSON exchanged between
EnerPlanET and BUEM. Current version: v5.0.0.

This number is unrelated to buem-gateway's own software releases, which are
git-tagged. The two are chosen independently and any match between them is
coincidence.

------------------------------------------------------------------------

## How a version is communicated

Nothing in a request declares which schema version it was built against.
There is no `schema_version` field, no header, and no URL segment for it.
The version in this file is a label for what shape the current `schemas/`
describes, not something negotiated per request.

A request that does not match the current schema is not detected as a
distinct "wrong version" case. It fails ordinary field validation instead:
`requireEnvelope`, `requireWeather`, and BuEM's own downstream checks reject
it with a 400 naming the specific missing or invalid field. There is no
separate wrong-version error, so to a caller it looks identical to any other
validation failure.

One caveat: `internal/buem/types.go` has its own `APIVersion` constant,
recording what BuEM's own validator was last confirmed to accept. It is
updated manually when someone re-runs that confirmation, not automatically
alongside this file.

------------------------------------------------------------------------

## How to release a new schema version

No git tag and no GitHub release. A schema bump is tracked entirely in text:

0. Breaking change only: copy the current `schemas/` files to `schemas/vN/` before
   editing anything, where N is the version you are leaving.
1. Edit the schema files directly in `schemas/` (this folder always holds the
   current version).
2. Update `CHANGELOG.md` with a plain-language description of what changed and why.
3. Update the version number in this file's table below and in
   `docs/api/openapi.yaml`'s `info.version` field, so all three stay in sync.
4. Run the validation check below. It confirms the schema and its example files are
   consistent, and that the version string in this file, `CHANGELOG.md`, and
   `docs/api/openapi.yaml` all agree.
5. Commit directly. If the change also required a buem-gateway code change (e.g.
   a new field the connector now reads), that code change gets its own git tag as
   a normal software release. See [`docs/getting-started.md`](getting-started.md)
   for that process.

```bash
python scripts/validate_schemas.py
```

------------------------------------------------------------------------

## Where are old versions stored?

| Location | Contents |
|---|---|
| `schemas/` | Current version, edit here |
| `schemas/v1/`, `schemas/v2/`, `schemas/v3/`, `schemas/v4/` | Read-only snapshots of past major versions |

The `v5.x` line (`v5.0.0` → current) has no separate snapshot folder yet. It's the
line currently in `schemas/` at the repo root. A `schemas/v5/` snapshot gets cut only
when a future breaking change moves the current schema to `v6.0.0`.

------------------------------------------------------------------------

## Released versions

| Version | Date | Status | What changed |
|---|---|---|---|
| v5.0.0 | 2026-08 | Current | `envelope` required again, v4.x TABULA fallback removed (see `CHANGELOG.md`) |
| v4.2.0 | 2026-06 | Unsupported | Optional `model_id` on the request `FeatureCollection`, used by the gateway to namespace CSV output per model |
| v4.1.0 | 2026-04 | Unsupported | Optional `name` field on `building` and `envelope_element`, for display purposes only |
| v4.0.0 | 2026-03 | Unsupported | `solver.compute_cooling` (opt-in cooling), file-path electricity input, `envelope` now optional (TABULA fallback), `phi_int`/`q_w_nd` configurable |
| v3.0.0 | 2026-03 | Unsupported | `buem` split into `building` (classification, envelope, thermal) and `solver`; thermal properties on each surface element directly; every physical quantity carries its unit |
| v2.0.0 | 2026-02 | Unsupported | Structured building attributes; nested component model introduced |
| v1.0.0 | 2025-11 | Unsupported | Initial format, minimal structure |

See `CHANGELOG.md` for full detail on each version.
