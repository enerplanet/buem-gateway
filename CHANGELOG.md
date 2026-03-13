# BUEM–EnerPlanET API Schema Changelog

## v2 (2026-02) — Structural Refinements

Status: Current version
Migration from v1: Breaking changes introduced

### Structural Refinements (2026-03)

The following improvements were applied to v2 to strengthen validation and
reduce ambiguity before v3 is introduced. No modelling logic changes were
made.

#### 1. Removed Duplicate Spatial Authority

- Removed `latitude` and `longitude` fields from `building_attributes`.
- GeoJSON geometry coordinates are now the single authoritative spatial
  source of truth.
- `required: ["latitude", "longitude"]` constraint removed from
  `building_attributes`.

#### 2. Formalised `_external_format_fields`

- `_external_format_fields` was previously invalid JSON Schema — it
  declared sub-fields directly inside the property object rather than
  inside `properties`.
- Now formalised as a proper nested object with `"type": "object"` and
  `"properties"`.
- `"additionalProperties": false` added to prevent undeclared fields.

#### 3. Resolution Field Changed to Integer

- `resolution` changed from `"type": "string"` to `"type": "integer"`
  with `"minimum": 1` in both request and response schemas, and in the
  `thermal_load_profile` block.
- Example payloads updated accordingly (`"60"` → `60`).

#### 4. Restricted `additionalProperties` in Components

- `components` previously allowed any property name via
  `"additionalProperties": { "$ref": "#/$defs/component" }`.
- Now set to `"additionalProperties": false` to restrict component names
  to the defined set: Walls, Roof, Floor, Windows, Doors, Ventilation.

#### 5. Deprecated `child_components`

- `child_components` (flat format) is officially deprecated.
- The nested `components` structure inside `building_attributes` is the
  preferred format going forward.
- `child_components` is retained for external compatibility only and will
  be removed in v3.

#### 6. Clarified Informational Defaults

- `default` keyword values in JSON Schema are informational only; they
  are not applied automatically.
- Descriptions for all fields carrying a `default` value now explicitly
  state: "informational default only; runtime must enforce this value".
  Fields affected: `resolution`, `resolution_unit`, `use_milp`,
  `b_transmission`, `g_gl`.

#### 7. Explicit Echo Rule for `building_attributes` in Response

- Response schema description for `building_attributes` now explicitly
  states: the response must contain exactly the same object as submitted
  in the request without modification.
- Example response updated to include a complete `building_attributes`
  echo matching the example request.

---

## v2 (2026-02)

Status: Current version
Migration from v1: Breaking changes introduced

### 1. Schema Structure Improvements

- Added `$id`, `title`, and `description` for schema identification and tracking.
- Introduced structured `$defs` replacing generic v1 definitions.
- Improved inline documentation.

### 2. Geometry Definition Updated

- v1 restricted geometry to 2D coordinates.
- v2 allows optional elevation (`maxItems: 3`).

### 3. Introduction of `building_attributes`

- v1 used an untyped object.
- v2 provides explicit structured fields:
  - latitude, longitude
  - A_ref, h_room
  - nested components
  - external compatibility fields

### 4. Structured Building Components Model

- v1 supported only flat `child_components`.
- v2 adds a nested component model:
  - Walls, Roof, Floor, Windows, Doors, Ventilation
  - each with detailed thermal/geometry attributes.

### 5. Redefined Component Elements

- Added area, tilt, azimuth, U-value override.
- Specialised element types introduced:
  - window_element
  - ventilation_element

### 6. New Model Control Field

- Added `use_milp` boolean with default `false`.

### 7. Improved Validation Requirements

- `minItems: 1` for features.
- Stricter typing for all attribute groups.

### 8. Deprecated or Changed Field Interpretations

- Lat/lon moved into `building_attributes`.
- Replaced generic `building_attributes` with structured schema.

---

## v1 (2025-11)

Status: Deprecated

### Notes

- Minimal schema with loose typing.
- Only flat child component model.
- No detailed modelling attributes.
- No explicit schema identification.
- Strictly 2D geometry.
