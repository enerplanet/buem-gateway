# BUEM–EnerPlanET API Schema Changelog

------------------------------------------------------------------------

## v4.2.0 (2026-06) — Current

**Status:** Current version
**Compatible with v4.1.0:** Yes — new optional field only.

### What changed and why

#### 1. `model_id` field on the request `FeatureCollection`

Added an optional `model_id: string` field at the top level of the request FeatureCollection.

| Location | Field | Type | Required |
|---|---|---|---|
| `FeatureCollection` | `model_id` | `string` | No |

The BuEM solver does not use this value — it is forwarded as-is and ignored by the simulation engine. The EnerPlanET gateway reads it to isolate load profile CSV files per model: profiles are written to `{BUEM_DATA_DIR}/{model_id}/` rather than a shared flat directory. This prevents filename collisions between models and ties profile lifecycle to the model lifecycle (profiles are deleted when the model is deleted).

------------------------------------------------------------------------

## v4.1.0 (2026-04)

**Status:** Current version
**Compatible with v4.0.0:** Yes — new optional fields only.

### What changed and why

#### 1. `name` field on `building` and `envelope_element`

Added an optional `name: string` field to both `building` and each
`envelope_element`. The field is display-only and has no effect on the
simulation. It allows clients (e.g. EnerPlanET) to round-trip a
human-readable label alongside the technical `id`, making surfaces and
buildings easier to identify in UI and export files.

| Location | Field | Type | Required |
|---|---|---|---|
| `building` | `name` | `string` | No |
| `envelope_element` | `name` | `string` | No |

------------------------------------------------------------------------

## v4.0.0 (2026-03)

**Status:** Current version
**Compatible with v3:** No — response clients must handle optional `cooling` field.

------------------------------------------------------------------------

### What changed and why

#### 1. Cooling simulation is now opt-in (`solver.compute_cooling`)

A new boolean flag `compute_cooling` (default: `false`) has been added to the
`solver` section of the request.

When `false` (the default), the upper comfort temperature bound is not enforced in
the LP solver. The indoor temperature can rise freely, no cooling demand is
generated, and `cooling` is absent from the response summary and timeseries.

When `true`, the upper comfort bound (`building.thermal.comfortT_ub`) is enforced
and a cooling load profile is returned alongside heating.

**Why:** Most residential buildings in northern Europe have no active cooling. Running
the cooling simulation for every building wastes computation and produces a result
that has no physical meaning for that building. Making cooling opt-in aligns the
model output with the actual building configuration.

#### 2. User-provided electricity load profile (`buem.inputs`)

A new optional `inputs` section has been added to the `buem` node in the request.
Currently it holds one field:

```json
"inputs": {
  "electricity_load_profile": {
    "path": "/data/profiles/building_001_elec.csv",
    "unit": "kWh"
  }
}
```

The profile is referenced by **file path**, not inlined as an array. The file must
be accessible inside the model container via the shared Docker volume (`BUEM_DATA_DIR`).
Supported formats: CSV (single column of values), JSON array, or gzipped JSON (`.gz`).

When `electricity_load_profile` is provided, it is used directly as the internal
heat gain input (`elecLoad`) in the ISO 13790 energy balance. When absent, the
model generates a profile from its occupancy simulation.

**Why:** An 8760-value array inlined in the JSON payload is large and makes the
request unwieldy. File path referencing keeps the JSON slim and is consistent with
how the model already returns output timeseries — written to a shared volume and
referenced by path. Electricity consumption by appliances heats the building
interior and therefore affects both heating demand and cooling demand.

#### 3. `cooling` is no longer required in the response (breaking)

`thermal_load_profile.summary.cooling` and `timeseries.cooling` are now optional.
They are present only when `solver.compute_cooling` was `true` in the corresponding
request.

Clients that always expect `cooling` in the response must be updated to check for
its presence before reading it.

#### 4. `model_metadata` reports what was computed

Two new fields added to `model_metadata` in the response:

| Field | Type | Meaning |
|---|---|---|
| `simulations_run` | `string[]` | Which load types were computed (`heating`, `cooling`, `electricity`) |
| `electricity_source` | `string` | Whether electricity came from `model_generated` or `client_provided` |

------------------------------------------------------------------------

## v3.0.0 (2026-03) — Deprecated

**Status:** Deprecated
**Compatible with v2:** No — requests must be updated before sending to a v3 server.

------------------------------------------------------------------------

### What changed and why

#### 1. Building description has a clear two-level structure

In v2 all building parameters lived together in a single block called
`building_attributes`. In v3, `buem` contains two clearly separated concerns:

| Section | What it contains |
|---|---|
| `building` | The complete physical description of the building: classification fields (type, period, country, etc.), surface geometry and thermal properties (`envelope`), and building-wide thermal parameters (`thermal`) |
| `solver` | Computation options: which solver to use, whether to run in parallel |

`envelope` and `thermal` are nested inside `building` because they describe the
building — they are not independent concerns at the same level as `solver`.

**Why:** The old flat structure mixed building physics with computation settings.
The new structure makes it clear what belongs to the building description and what
belongs to how the model runs.

#### 2. Building location is taken from the map coordinates only

In v2, latitude and longitude were repeated inside `building_attributes`. In v3,
location comes exclusively from `feature.geometry.coordinates [longitude, latitude]`
— the standard GeoJSON location field. The duplicate fields are removed.

**Why:** Having location in two places risks them being inconsistent. The GeoJSON
geometry is the authoritative location, so the model uses that directly.

#### 3. Building surfaces listed as a flat list

In v2, surfaces were grouped into a nested structure by type (a `Walls` object
containing an `elements` list, a `Roof` object, and so on). In v3, all surfaces
— regardless of type — are in a single flat list called `building.envelope.elements`.
Each surface has an `id` that you assign and a `type` field (wall, roof, floor,
window, door, or ventilation).

There is no longer a limit on how many surfaces of each type you can include.

**Why:** The nested structure made it awkward to add more than one roof section or
an unusual wall configuration. A flat list with explicit types is simpler to build
and read.

#### 4. Thermal properties defined on each surface directly

Each surface element carries both its geometry (area, orientation, tilt) and its
own thermal properties (U-value, solar gain coefficient, transmission correction
factor) in a single entry. There is no separate list of thermal properties that
cross-references surfaces by id.

```json
{
  "id": "Wall_1", "type": "wall",
  "area": { "value": 30.0, "unit": "m2" },
  "azimuth": { "value": 0.0, "unit": "deg" },
  "tilt": { "value": 90.0, "unit": "deg" },
  "U": { "value": 1.6, "unit": "W/(m2K)" },
  "b_transmission": { "value": 1.0, "unit": "-" }
}
```

**Why:** Keeping geometry and thermal performance together in one entry avoids
having to match two separate lists by id. Each surface is self-contained and
readable on its own.

#### 5. Windows and doors reference their parent surface by id

Windows and doors have a `parent_id` field that holds the `id` of the wall (or
roof, for a skylight) they are embedded in. This replaces the previous `surface`
field whose name did not clearly describe its purpose.

#### 6. Building-wide thermal parameters available as inputs

Several thermal parameters that the model used internally with fixed default values
can now be set explicitly in the `building.thermal` section. They are all optional
— if omitted, the same defaults as before are used.

| Parameter | Physical meaning |
|---|---|
| `n_air_infiltration` | Uncontrolled air infiltration rate (how much cold air leaks in) |
| `n_air_use` | Ventilation air change rate during occupancy |
| `c_m` | Thermal mass of the building (how much heat the structure can store) |
| `thermal_class` | Thermal mass category (light / medium / heavy) |
| `comfortT_lb` | Minimum acceptable indoor temperature (heating setpoint) |
| `comfortT_ub` | Maximum acceptable indoor temperature (cooling setpoint) |
| `design_T_min` | Outdoor design temperature for peak load calculation |
| `F_sh_hor` / `F_sh_vert` | Shading reduction factors for horizontal and vertical surfaces |
| `F_f` | Window frame fraction (fraction of window area that is frame, not glass) |
| `F_w` | Window correction factor |

**Why:** These parameters significantly affect results. Exposing them allows
calibration against measured data and makes the model's assumptions explicit.

#### 7. Old input formats removed

Two input structures from v2 are no longer accepted:
- The `child_components` flat array (an early format from v1).
- The `building_attributes` block (replaced by the structure above).

#### 8. Every physical quantity now carries its unit

In v2, quantities were plain numbers (e.g. `"area": 85.0`). In v3, every
measurable quantity is an object with a value and a unit:

```json
"area": { "value": 85.0, "unit": "m2" }
```

The unit field uses standard SI notation. If omitted, SI is assumed. Imperial units
(ft², BTU, °F, etc.) are accepted where noted.

**Why:** A bare number is ambiguous — is area in m² or ft²? Is temperature in °C
or °F? Carrying the unit alongside the value eliminates this ambiguity and allows
the frontend to display values in the user's preferred unit.

#### 9. Output quantities also carry their unit

The same pattern applies to the response. All summary quantities in
`thermal_load_profile.summary` are `{value, unit}` objects.

The unit suffix is removed from field names now that the unit is carried explicitly:

| v2 field name | v3 field name |
|---|---|
| `total_kwh` | `total` |
| `max_kw` | `max` |
| `min_kw` | `min` |
| `mean_kw` | `mean` |
| `median_kw` | `median` |
| `std_kw` | `std` |

#### 10. Hourly result arrays have a single shared unit label

The hourly arrays (`heating`, `cooling`, `electricity`) remain plain number lists.
A single `unit` field at the top level of `timeseries` declares the unit for all
three arrays (avoiding the overhead of wrapping 8760 values individually).

#### 11. Response now always includes a processing summary

The response now always contains a `metadata` block at the top level:

```json
"metadata": {
  "total_features": 5,
  "successful_features": 4,
  "failed_features": 1
}
```

------------------------------------------------------------------------

## v2.0.0 (2026-02) — Deprecated

**Status:** Deprecated
**Compatible with v1:** No — requests must be updated.

### What changed

- Building parameters were organised into a structured `building_attributes` block,
  replacing the loosely typed object from v1.
- Building surfaces were introduced as a nested structure (Walls, Roof, Floor,
  Windows, Doors, Ventilation), each with geometry and thermal attributes.
- Elevation was added as an optional third coordinate in the geometry.
- The `use_milp` solver flag was introduced.
- Validation rules were tightened throughout.

------------------------------------------------------------------------

## v1.0.0 (2025-11) — Deprecated

**Status:** Deprecated

Initial release. Minimal structure with loose typing. Supported only a flat list of
child components. No detailed thermal or geometric parameters. Location was
specified as bare latitude/longitude fields. Strictly 2D geometry.
