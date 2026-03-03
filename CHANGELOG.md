# BUEM–EnerPlanET API Schema Changelog

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
