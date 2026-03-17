# BUEM–EnerPlanET Schema Versioning

This file explains how the data format between EnerPlanET and BUEM is versioned and
how to release a new version.

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

A version number looks like this: **3.1.0**

The three numbers mean:

```
  3   .   1   .   0
  |       |       |
  |       |       +-- Documentation fix only (no data format change)
  |       +---------- New optional field added (old requests still work)
  +------------------ Breaking change (old requests no longer work)
```

------------------------------------------------------------------------

## Three types of change

### Breaking change (first number increases — e.g. 2.0.0 → 3.0.0)

The new format is not compatible with the old one. Any system sending requests in
the old format must update before it will work again.

This applies when:
- A required field is renamed or removed
- A field that accepted a plain number now requires a value with a unit (e.g. `150`
  becomes `{"value": 150, "unit": "kJ/(m2K)"}`)
- A section of the payload is reorganised or split

### New optional field (middle number increases — e.g. 3.0.0 → 3.1.0)

Something new is available, but nothing existing is removed or changed. Requests
that worked before still work. The new field is simply ignored if not provided.

This applies when:
- A new optional parameter is added (e.g. a new shading correction factor)
- A new unit option is allowed for an existing quantity

### Documentation fix only (last number increases — e.g. 3.1.0 → 3.1.1)

The data format is unchanged. Only text descriptions, example values, or formatting
are corrected.

------------------------------------------------------------------------

## How to release a new version

1. Edit the schema files directly in `schemas/` — this folder always holds the
   current version.
2. Run the validation check below to confirm the schema and its example files are
   consistent.
3. Update `CHANGELOG.md` with a plain-language description of what changed and why.
4. Commit and create a Git release with the appropriate version tag.

```bash
# Validation check — run this before every release
python -c "
import json
from jsonschema import Draft202012Validator
for name in ['request', 'response']:
    schema  = json.load(open(f'schemas/{name}_schema.json'))
    example = json.load(open(f'schemas/example_{name}.json'))
    errs = list(Draft202012Validator(schema).iter_errors(example))
    print(f'{name}: OK' if not errs else [e.message for e in errs])
"

# Create the release (adjust tag and notes)
gh release create v3.1.0 --title "v3.1.0" --notes "Short description of change" \
  schemas/request_schema.json \
  schemas/response_schema.json \
  schemas/example_request.json \
  schemas/example_response.json
```

------------------------------------------------------------------------

## Where are old versions stored?

| Location | Contents |
|---|---|
| `schemas/` | Current version — edit here |
| `schemas/v1/`, `schemas/v2/`, `schemas/v3/` | Read-only snapshots of past versions |

Any past version can also be retrieved from Git using the release tags
(`v1.0.0`, `v2.0.0`, etc.).

------------------------------------------------------------------------

## Released versions

| Version | Date | Status | What changed |
|---|---|---|---|
| v3.0.0 | 2026-03 | Current | `buem` split into `building` (full physical description: classification, envelope, thermal) and `solver`; thermal properties on each surface element directly; every physical quantity carries its unit |
| v2.0.0 | 2026-02 | Deprecated | Structured building attributes; nested component model introduced |
| v1.0.0 | 2025-11 | Deprecated | Initial format — minimal structure |

See `CHANGELOG.md` for full detail on each version.

------------------------------------------------------------------------

## What must happen before a change is merged?

1. `CHANGELOG.md` is updated with a description of the change.
2. The validation check above passes without errors.
3. The change has been reviewed.
4. The version number follows the rules above.
