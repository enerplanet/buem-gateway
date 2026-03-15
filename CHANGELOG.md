# BUEM–EnerPlanET API Schema Changelog

## v3 (2026-03)

Status: In development
Migration from v2: Breaking changes introduced

### 1. Separation of Concerns in Request Schema

`building_attributes` replaced by four dedicated nodes under `buem`:

- `building` — thematic and classification parameters (type, construction period, country, storeys, room height, floor area, attic/cellar/neighbour conditions)
- `envelope` — geometric description of building elements as a flat list (area, azimuth, tilt, surface reference)
- `thermal` — thermal performance parameters aligned with IEE TABULA definitions (U-values, ventilation rates, thermal mass, comfort setpoints, shading factors)
- `solver` — execution control flags (use_milp, parallel_thermal, use_chunked_processing)

### 2. Location Removed from buem

`building_attributes.latitude` and `building_attributes.longitude` removed. The GeoJSON `feature.geometry.coordinates [lon, lat]` is now the authoritative location source, used for weather station lookup. No duplication.

### 3. Flat Envelope Elements

`components` nested object (Walls/Roof/Floor/Windows/Doors/Ventilation) replaced by `envelope.elements[]` — a single flat list of elements each with an `id` and `type` enum. Geometry (area, azimuth, tilt) and parent surface reference live here.

### 4. Thermal Properties Decoupled from Geometry

U-values, `b_transmission`, `g_gl`, and `air_changes` moved out of the envelope and into `thermal.element_properties[]`, linked to envelope elements by `id`. This separates what a surface is from how it performs thermally.

### 5. TABULA-Aligned Thermal Parameters Exposed

Previously hardcoded implementation defaults are now first-class optional schema fields in the `thermal` node:

- `n_air_infiltration`, `n_air_use` (air change rates)
- `c_m`, `thermal_class` (thermal mass)
- `comfortT_lb`, `comfortT_ub` (comfort setpoints)
- `design_T_min` (outdoor design temperature)
- `F_sh_hor`, `F_sh_vert`, `F_f`, `F_w` (shading and window correction factors)

### 6. Legacy Formats Removed

- `child_components[]` flat array removed — no longer supported
- `building_attributes` object removed — replaced by `building`, `envelope`, `thermal`

### 7. Solver Flags Formalised

`use_milp`, `parallel_thermal`, and `use_chunked_processing` moved to a dedicated `solver` node. Previously `parallel_thermal` and `use_chunked_processing` were accepted by the implementation but absent from the schema.

### 8. Unit-Aware Measurement Types

All measurable quantities use `{ "value": number, "unit": string }` instead of bare numbers. A reusable `$defs` measurement library defines allowed units and SI defaults per quantity type:

- Geometric: `area_qty` (m2/ft2), `length_qty` (m/ft), `angle_qty` (deg/rad)
- Thermal: `u_value_qty` (W/(m2K)/BTU), `air_change_qty` (1/h), `heat_capacity_qty` (kJ/(m2K)), `temperature_qty` (degC/degF)
- Dimensionless: `dimensionless_qty` with `unit: "-"` as placeholder

The `unit` field defaults to SI when omitted. This enables frontend unit conversion and makes every field self-describing.

### 9. Response Measurement Types

Output quantities in `thermal_load_profile` also use `{ value, unit }`:

- `energy_qty` (kWh/MWh/Wh) — for `total` in energy summaries
- `power_qty` (kW/W/MW) — for `max`, `min`, `mean`, `median`, `std`
- `energy_intensity_qty` (kWh/m2) — for `energy_intensity`
- `duration_qty` (s/ms/min) — for `model_metadata.processing_time`

### 10. Response Energy Summary Field Names

Statistical fields renamed to drop the unit suffix (unit now carried in the measurement object):

| v2 field | v3 field |
|---|---|
| `total_kwh` | `total` |
| `max_kw` | `max` |
| `min_kw` | `min` |
| `mean_kw` | `mean` |
| `median_kw` | `median` |
| `std_kw` | `std` |

### 11. Timeseries Unit Declaration

`timeseries` arrays (`heating`, `cooling`, `electricity`) remain plain number arrays. A single `unit` field at the timeseries level declares the unit for all three arrays, avoiding per-value wrapping for 8760-entry arrays.

### 12. Response Metadata Formalised

`metadata` (top-level collection statistics) is now a required response field with required sub-fields `total_features`, `successful_features`, `failed_features`.

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
