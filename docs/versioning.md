# BUEM–EnerPlanET Schema Versioning

This file explains how the data format between EnerPlanET and BUEM is versioned and
how to release a new version.

!!! note "This is the data-contract version, not buem-gateway's own release version"
    The number here (currently **v5.0.0**) tracks the request/response JSON shape in
    `schemas/` — it has its own number and its own history, separate from the
    buem-gateway *connector's* own software releases (git tags, which trigger
    `.github/workflows/release.yml` and publish Docker images). A schema version is
    never git-tagged; it lives only in this file, `CHANGELOG.md`, and
    `docs/openapi.yaml`'s `info.version` field. Only the connector's own releases get
    a git tag and a GitHub release.

    These two numbers are picked and bumped independently — they will land on the
    same digit sometimes and drift apart other times, purely by chance each time
    (see this file's `v4.0.0`/`v4.2.0` vs the software's earlier `v4.0.0` for one
    such coincidence, already resolved). Never infer a relationship from them
    matching or not matching.

------------------------------------------------------------------------

## How a version is communicated

Nothing in a request declares which schema version it was built against —
there is no `schema_version` field, no header, and no URL segment for it.
The version in this file is a label for what shape the current `schemas/`
describes, not something negotiated per request.

A request that does not match the current schema is not detected as "wrong
version" — it fails ordinary field validation. `requireEnvelope`,
`requireWeather`, and BuEM's own downstream checks reject it with a 400
naming the specific missing or invalid field. There is no separate
wrong-version error; to a caller it looks identical to any other validation
failure.

One caveat: `internal/buem/types.go` has its own `APIVersion = "v3"`
constant, describing what BuEM's own validator was last confirmed to
accept. It predates the v5.0.0 schema change (2026-08-06) and has not been
revisited since. Reconciling it is tracked separately, not covered here.

------------------------------------------------------------------------

## What is the schema?

The schema is a formal description of what the model accepts as input and what it
returns as output. It defines which fields are required, what units to use, and how
results are structured.

Both sides of the integration depend on this description:
- EnerPlanET (the client) uses it to build requests.
- BUEM (the model server) uses it to read and validate incoming data.

------------------------------------------------------------------------

## Why versioning?

The model evolves over time — new parameters are added, field names may change, or
the structure may be reorganised. Versioning gives every change a clear label so
that both sides always know which format they are working with.

A version number looks like this: **4.2.0**

The three numbers mean:

```
  4   .   2   .   0
  |       |       |
  |       |       +-- Documentation fix only (no data format change)
  |       +---------- New optional field added (old requests still work)
  +------------------ Breaking change (old requests no longer work)
```

------------------------------------------------------------------------

## Three types of change

### Breaking change (first number increases — e.g. 3.0.0 → 4.0.0)

The new format is not compatible with the old one. Any system sending requests in
the old format must update before it will work again.

This applies when:
- A required field is renamed or removed
- A field that accepted a plain number now requires a value with a unit (e.g. `150`
  becomes `{"value": 150, "unit": "kJ/(m2K)"}`)
- A section of the payload is reorganised or split

### New optional field (middle number increases — e.g. 4.1.0 → 4.2.0)

Something new is available, but nothing existing is removed or changed. Requests
that worked before still work. The new field is simply ignored if not provided.

This applies when:
- A new optional parameter is added (e.g. a new shading correction factor)
- A new unit option is allowed for an existing quantity

### Documentation fix only (last number increases — e.g. 4.2.0 → 4.2.1)

The data format is unchanged. Only text descriptions, example values, or formatting
are corrected.

------------------------------------------------------------------------

## How to release a new schema version

No git tag and no GitHub release — a schema bump is tracked entirely in text:

0. Breaking change only: copy the current `schemas/` files to `schemas/vN/` before
   editing anything, where N is the version you are leaving.
1. Edit the schema files directly in `schemas/` — this folder always holds the
   current version.
2. Update `CHANGELOG.md` with a plain-language description of what changed and why.
3. Update the version number in this file's table below and in
   `docs/openapi.yaml`'s `info.version` field, so all three stay in sync.
4. Run the validation check below. It confirms the schema and its example files are
   consistent, and that the version string in this file, `CHANGELOG.md`, and
   `docs/openapi.yaml` all agree.
5. Commit directly — no tag, no release. If the change also required a
   buem-gateway code change (e.g. a new field the connector now reads), that code
   change gets its own git tag as a normal software release — see
   [`docs/getting-started.md`](getting-started.md) for that process.

```bash
python scripts/validate_schemas.py
```

------------------------------------------------------------------------

## Where are old versions stored?

| Location | Contents |
|---|---|
| `schemas/` | Current version — edit here |
| `schemas/v1/`, `schemas/v2/`, `schemas/v3/`, `schemas/v4/` | Read-only snapshots of past major versions |

The `v5.x` line (`v5.0.0` → current) has no separate snapshot folder yet — it's the
line currently in `schemas/` at the repo root. A `schemas/v5/` snapshot gets cut only
when a future breaking change moves the current schema to `v6.0.0`.

------------------------------------------------------------------------

## Released versions

| Version | Date | Status | What changed |
|---|---|---|---|
| v5.0.0 | 2026-08 | Current | `envelope` required again — the v4.x TABULA-fallback-when-omitted behavior needed buem-gateway to call ignis, which broke its standalone claim and produced confusing errors; removed, see `CHANGELOG.md` |
| v4.2.0 | 2026-06 | Unsupported | Optional `model_id` on the request `FeatureCollection`, used by the gateway to namespace CSV output per model |
| v4.1.0 | 2026-04 | Unsupported | Optional `name` field on `building` and `envelope_element`, for display purposes only |
| v4.0.0 | 2026-03 | Unsupported | `solver.compute_cooling` (opt-in cooling), file-path electricity input, `envelope` now optional (TABULA fallback), `phi_int`/`q_w_nd` configurable |
| v3.0.0 | 2026-03 | Unsupported | `buem` split into `building` (classification, envelope, thermal) and `solver`; thermal properties on each surface element directly; every physical quantity carries its unit |
| v2.0.0 | 2026-02 | Unsupported | Structured building attributes; nested component model introduced |
| v1.0.0 | 2025-11 | Unsupported | Initial format — minimal structure |

See `CHANGELOG.md` for full detail on each version.

------------------------------------------------------------------------

## What must happen before a change is merged?

1. `CHANGELOG.md` is updated with a description of the change.
2. The validation check above passes without errors.
3. The change has been reviewed.
4. The version number follows the rules above.
