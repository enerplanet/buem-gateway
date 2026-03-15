# BUEM--EnerPlanET API Schema Versioning Policy

This document defines how schema versions are managed for the
BUEM--EnerPlanET integration.

The JSON schemas in this repository represent the authoritative contract
between EnerPlanET (client) and the BUEM microservice (server).

------------------------------------------------------------------------

## Versioning Principles

1. Each schema version is stored in its own folder: schemas/v1/
    schemas/v2/ schemas/v3/

2. Version folders are immutable once released.

3. Breaking changes require a new version folder.

4. Minor clarifications that do not affect validation rules may be
    updated within the same version but must be documented.

5. The CHANGELOG.md file must be updated for every version release.

------------------------------------------------------------------------

## What Counts as a Breaking Change

A new version is required if:

- Required fields are added or removed.
- Field types change.
- Field location changes (e.g., moved into a nested structure).
- Validation constraints become stricter.
- Semantic meaning of fields changes.
- Quantity representation changes (e.g., bare number → measurement object).

Examples from v2 → v3:

- `building_attributes` replaced by four separate nodes: `building`,
  `envelope`, `thermal`, `solver`.
- `latitude`/`longitude` removed from `buem` — location is now read
  from `feature.geometry.coordinates` only.
- All measurable quantities changed from bare numbers to
  `{ "value": number, "unit": string }` objects.
- `components` nested object replaced by flat `envelope.elements[]`
  list discriminated by a `type` field.
- `child_components` legacy format removed.
- Energy summary fields renamed: `total_kwh` → `total`, `max_kw` → `max`, etc.

------------------------------------------------------------------------

## Current Versions

### v3 (2026-03)

Status: In development\
Migration from v2: Breaking changes introduced

Key changes:

- Separation of concerns: `building`, `envelope`, `thermal`, `solver` nodes.
- Location sourced from GeoJSON geometry — no duplication in `buem`.
- Unit-aware measurement types for all measurable quantities.
- Flat `envelope.elements[]` with user-defined ids, unlimited per type.
- Thermal properties decoupled from geometry via `thermal.element_properties[]`.
- TABULA-aligned thermal parameters exposed as optional schema fields.
- `metadata` formalised as a required top-level response field.
- Timeseries unit declared once at array level.

See CHANGELOG.md for the full list of changes.

### v2 (2026-02)

Status: Deprecated\
Migration from v1: Breaking changes introduced

Key changes:

- Added `$id`, `title`, and `description`.
- Introduced structured `$defs`.
- Updated geometry to allow optional elevation (3D).
- Replaced loose `building_attributes` with structured schema.
- Added nested building components model.
- Added detailed component element definitions.
- Introduced `use_milp` control flag.
- Enforced stricter validation rules.

### v1 (2025-11)

Status: Deprecated

Characteristics:

- Minimal schema with loose typing.
- Flat child component model only.
- Strictly 2D geometry.
- No schema identification metadata.

------------------------------------------------------------------------

## Version Lifecycle

- Only one version is marked as Current.
- Older versions are marked as Deprecated.
- Deprecated versions remain available for reference but should not be
    used for new integrations.
- Every version must document all breaking and non-breaking changes
    in CHANGELOG.md.

------------------------------------------------------------------------

## Governance

EnerPlanET maintains this contract repository.

Any proposed schema change must:

1. Be documented in CHANGELOG.md
2. Be reviewed before merging
3. Be validated using JSON Schema validation tools
4. Increment the version if breaking changes are introduced

------------------------------------------------------------------------

This policy ensures stable integration and controlled evolution of the
BUEM--EnerPlanET API contract.
