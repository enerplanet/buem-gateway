# buem fork (enerplanet branch) — consume the contract instead of redefining it

**Status:** internal migration task. The EnerPlanET team owns the `enerplanet`
branch of the `buem` fork and the buem-gateway side, so both move together — no
external sign-off. Not yet started in the `buem` repo.

**Audience:** developer.

## Why

The BUEM-EnerPlanET request/response contract lived as two independently edited
copies — `buem/src/buem/integration/json_schema/versions/` and
`buem-gateway/schemas/` — kept in step by hand. They drifted: as of August 2026
they disagreed on the version number (the buem fork called the shipped contract
v4, buem-gateway called it v5), on whether `weather` is in the schema at all
(buem-gateway's runtime required it; neither repo's schema modelled it), and on
several field-level details.

buem-gateway is now the single source of truth: `buem-gateway/schemas/v5/` is the
frozen definition of **API contract v5**, and its versioning is directory-based
(`schemas/v5/` frozen current, `schemas/v6-draft/` in development). See
`buem-gateway/docs/versioning.md`.

This brings the buem fork's `enerplanet` branch onto the same contract without it
maintaining a parallel definition.

## What changes in the buem fork (`enerplanet` branch)

### 1. Stop maintaining a contract definition

- Delete `src/buem/integration/json_schema/versions/v1/` … `v4/`. Their history
  is in git and mirrored in `buem-gateway/schemas/v1..v4/`.
- Delete `src/buem/integration/json_schema/VERSIONING.md` and
  `SCHEMA_OVERVIEW.md`. Replace `README.md` in that folder with a short pointer
  (see step 4).

### 2. Keep one pinned copy, checked by CI

- Add `src/buem/integration/json_schema/request_schema.json` and
  `response_schema.json` as verbatim copies of
  `buem-gateway@<tag>:schemas/v5/{request,response}_schema.json`, each with a
  leading `"$comment"`:

  ```json
  "$comment": "Pinned copy of enerplanet/buem-gateway@<tag> schemas/v5/request_schema.json. Do not edit here. Re-sync to update."
  ```

- Add a CI job that fetches the same file from buem-gateway at the pinned tag and
  fails if it differs from the local copy. Pin the tag in one place (a
  `CONTRACT_TAG` variable or a small `contract.txt`), so re-syncing is: bump the
  tag, re-copy, done.

### 3. Runtime validation against the pinned schema, retire the marshmallow validator

`src/buem/integration/scripts/geojson_validator.py` does two jobs. Split them:

- **Validation** — `BuildingAttributesSchema`, `BuemSchema.require_building_envelope`,
  the `building_type` enum, the per-provider `weather.year` range check, the
  "reject unknown v4 fields" logic, and the marshmallow schemas feeding them.
  **Delete.** Replace with a `jsonschema.Draft202012Validator` over the pinned
  `request_schema.json`. `schema_validator.py` + `schema_manager.py` already do
  exactly this against `json_schema/versions/`; repoint them at the pinned copy
  and make `geojson_processor.py` call that instead of `GeoJsonValidator`.
  (`jsonschema` is already a buem dependency, so this adds nothing. buem-gateway
  keeps its checks hand-written and stdlib-only to match MEME and stay
  dependency-free — a Python service validating against the schema file at
  runtime is a reasonable difference, not a contradiction.)
- **Conversion** — `_convert_v3_to_v2`, `_convert_components_format`,
  `_child_to_nested_components`, `_weather_from_payload`,
  `_check_weather_profile_ranges`. **Keep.** These transform a validated request
  into the model's internal `building_attributes` shape; they are not validation
  and get simpler when they can assume schema-valid input. Move them to a
  `geojson_transform.py` (or keep the file, minus the marshmallow classes).

### 4. Shrink the folder's own docs

`src/buem/integration/json_schema/README.md` becomes:

> The BUEM-EnerPlanET API contract is owned by `enerplanet/buem-gateway`
> (`schemas/v5/`, `docs/versioning.md`). The files here are a pinned copy of
> **API contract v5**; do not edit them. Re-sync from buem-gateway to update.

### 5. Fix the stale Guardrail in `buem/CLAUDE.md`

The "Guardrails" section still describes a three-tier model (live version /
`versions/v4/` draft / promotion) and says never edit `geojson_validator.py`
without checking in. After this change there is no `versions/` tree and no
marshmallow validator to guard. Replace that section with: the contract is a
pinned copy from buem-gateway, changed only by re-syncing; propose contract
changes as a `schemas/v6-draft/` PR on buem-gateway.

## The `weather` selector is out of contract

buem-model's `geojson_validator.py` accepts `buem.weather` as a `{provider, year}`
selector and fetches the timeseries itself. **API contract v5 does not.** `weather`
must be a pre-resolved inline `{index, variables}` timeseries; buem-gateway rejects
a selector-only block. buem-model may keep reading `provider`/`year` for its own
standalone CLI runs, but on the EnerPlanET path the caller supplies resolved
weather and BuEM must not self-fetch (matches buem-gateway's `#37` and buem's
`0a42545`).

If buem-model needs the selector for standalone use, gate it behind an explicit
"standalone" mode that is off for requests arriving via buem-gateway, and keep it
out of the shared schema.

## `upstream/feature/schema-folder-refactor` is stale, ignore it

That branch ("Add schema validator module", "Refactor API schema folder
structure") is 4 commits based ~73 commits behind `enerplanet`, proposes a
different layout (`integration/schemas/v1/`), and adds a `schema_validator.py`
that `enerplanet` already has its own version of. Nothing to reconcile.

## Known blocker in the buem fork

`buem/CLAUDE.md` records that `tests/test_energy.py`,
`tests/test_geojson_integration.py`, and others fail to collect
(`ModuleNotFoundError: No module named 'buem.results'`). Fix or confirm that
before relying on the buem test suite to verify this migration.

## Not covered

- The buem-gateway side (schema freeze, schema/runtime alignment, mutable-file
  removal, drift-gate test) is already done — see `buem-gateway` `CHANGELOG.md`
  v5.0.0 §0.
- The `/validate` response envelope (`{valid, error}`) is a separate piece of
  work on both services.
- The upstream (`upstream/main`, UU-BUEM) contract. These changes are for the
  `enerplanet` branch only; upstream merges are reconciled separately.
