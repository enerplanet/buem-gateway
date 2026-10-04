---
audience: developer
---

# Building types

How buem-model 6.6.0 (with occupancy 6.1.0+enerplanet.1) models each `building_type`: what to send, how the building is simulated, and what to tell a user about the result. Field rules follow the v6-draft request contract (`schemas/v6-draft/request_schema.json`).

The types fall into two groups:

| Group | Types | Occupancy comes from |
|---|---|---|
| Residential | `SFH` single-family, `TH` terraced house, `MFH` multi-family, `AB` apartment block | a household model: persons, household archetype, appliances |
| Service | `bakery`, `clinic`, `hotel`, `office`, `restaurant`, `school`, `supermarket`, `warehouse` | an opening-hours schedule scaled by capacity |

## One building, many dwellings

Each entry in a request is one building with one envelope. A dwelling is one self-contained living space occupied by one household. `residential_units` is the number of dwellings in the building:

- An apartment block with 12 flats is one entry with `residential_units: 12`.
- A row of twelve single-family houses is twelve entries, each with `residential_units: 1`.

`residential_units` never counts separate buildings.

## Profiles and what they need

Each request selects profiles in `buem.outputs` (`none`, `summary` or `series`).

| Profile | Needs `envelope` and `weather` | Source |
|---|---|---|
| `heating`, `cooling` | yes | thermal model (one zone), driven by the envelope, weather and internal gains |
| `electricity` | no | occupancy model |
| `hot_water` | no | occupancy model; residential only |
| `kitchen` | no | occupancy model; residential with `cooking_carrier: gas` only |

A request with `heating` and `cooling` both `none` needs no envelope and no weather. Its year comes from `properties.start_time`.

## Residential buildings

### What to send

| Field | Required | Default when omitted | Notes |
|---|---|---|---|
| `building_type` | yes | | `SFH`, `TH`, `MFH`, `AB` |
| `country` | yes | | household-size figures exist for `NL`; other countries use generic European figures |
| `A_ref` | yes | | floor area of the whole building, never of one dwelling |
| `residential_units` | for `MFH` and `AB` | 1 | the number of dwellings in the building, an integer |
| `num_persons` | no | statistical household size per dwelling (table below) | occupants per dwelling, not per building |
| `archetype` | no | by type (table below) | `generic`, `working_couple`, `family_with_children`, `retired_single`, `student_shared` |
| `region_code` | no | country figures | CBS municipality code, e.g. `GM0200` |
| `equipment` | no | default ownership probabilities | see [Appliances](#appliances) |
| `cooking_carrier` | no | `electric` | `electric`, `gas` or `none` |
| `include_dhw` | no | `true` | reports hot water |
| `setback_profile` | no | `none` | lowers the heating set point while occupants sleep or are away |

Defaults per type:

| Type | Household size, NL | Household size, other countries | Default archetype |
|---|---|---|---|
| `SFH` | 2.55 | 2.6 | `family_with_children` |
| `TH` | 2.41 | 2.4 | `working_couple` |
| `MFH` | 1.54 | 1.9 | `generic` |
| `AB` | 1.54 | 1.7 | `generic` |

Municipal NL figures: `GM0200` (Apeldoorn) 1.37 for `MFH` and `AB`; `GM0177` (Raalte) 2.47 `SFH`, 2.33 `TH`, 1.28 `MFH` and `AB`.

### How the building is simulated

buem-model simulates **one representative dwelling** and multiplies it by `residential_units`:

1. The household size for one dwelling is resolved (`num_persons`, otherwise the table above).
2. A fractional size is simulated as two households, the sizes below and above, blended by weight. 1.54 persons is 46% of a 1-person and 54% of a 2-person household.
3. Occupant presence, appliance use, hot water draws and cooking are generated for that dwelling.
4. Electricity, internal gains, hot water and cooking are multiplied by `residential_units`.
5. The thermal model sees the whole building: `A_ref`, the envelope and `h_room` describe the building, and the internal gains are the multiplied dwelling.

!!! warning "Every dwelling in a building is identical"
    A 20-dwelling block is 20 copies of one dwelling: the same household, appliances and hour-by-hour profile. Annual totals are representative, but the building's peak electricity, hot water and cooking demand is the single-dwelling peak times the number of dwellings, with no allowance for dwellings peaking at different times. For `MFH` and `AB` this overstates peaks.

!!! warning "A_ref must describe the whole building"
    Sending one dwelling's floor area with `residential_units` above 1 packs all dwellings' internal gains into one dwelling's volume. Nothing rejects that combination.

### Appliances

`building.equipment` switches individual appliances for the representative dwelling, so it applies to every dwelling in the building.

- `true` installs the appliance, `false` removes it.
- An omitted appliance keeps its default ownership probability. The household draws once whether it owns one, with a fixed seed, so the same inputs give the same result.
- Each id is one appliance; counts are not expressible. Up to three televisions are `tv_1`, `tv_2` and `tv_3`.
- Appliance power and usage patterns are fixed in the occupancy model and cannot be set through the contract.

| Group | Appliance ids |
|---|---|
| Cooking | `hob`, `oven`, `microwave`, `kettle`, `small_cooking_group` |
| Cold | `fridge_freezer`, `refrigerator`, `chest_freezer`, `upright_freezer` |
| Laundry and cleaning | `washing_machine`, `tumble_dryer`, `washer_dryer`, `dish_washer`, `iron`, `vacuum` |
| Electronics | `tv_1`, `tv_2`, `tv_3`, `tv_receiver_box`, `personal_computer`, `printer`, `hi_fi`, `cassette_cd_player`, `vcr_dvd` |
| Other | `lighting`, `clock`, `answer_machine`, `cordless_telephone`, `fax` |

Usage scales with the number of occupants for laundry, cooking, lighting and computing, but not for cold appliances.

### Hot water and cooking

- Hot water is generated as tapping events per fixture (basin, kitchen sink, shower, bath), scaled by `num_persons`, and priced at a cold-water temperature of 11.2 °C with delivery at 38 to 50 °C. It does not depend on weather and includes no storage or distribution losses. It is reported as useful heat, not as electricity.
- With `cooking_carrier: electric` cooking stays inside electricity and `kitchen` is zero. With `gas` the cooking energy moves out of electricity into `kitchen`, in kWh of gas. With `none` nothing is reported separately and electric cooking stays inside electricity.

### Terraced houses

A wall shared with a neighbour counts as shared only when it is sent with `b_transmission: 0`, or left out of the envelope. A party wall listed as an ordinary wall is modelled as an outside wall. `neighbour_status` has no effect.

## Service buildings

### What to send

| Field | Required | Default when omitted | Notes |
|---|---|---|---|
| `building_type` | yes | | one of the eight service types |
| `country` | yes | | |
| `A_ref` | yes | | whole building |
| `capacity` | no | `A_ref` divided by the floor area per occupant (table below) | number of occupants |
| `residential_units` | | 1 | must be 1 |

`equipment`, `num_persons`, `archetype` and `region_code` are rejected for service types. `include_dhw` and `cooking_carrier` are accepted and have no effect.

### How the building is simulated

Occupants are present during opening hours, as a share of `capacity` with some random variation, and absent outside them. Hotels follow a 24-hour occupancy curve instead. Electricity comes from the type's own equipment set (for example lighting, plug loads, refrigeration, kitchen), switched by occupancy. There are no hot water or cooking models, so `hot_water` and `kitchen` come back as zero. Internal gains are occupant heat plus a floor-area term, and are not taken from electricity.

| Type | Opening hours (weekday / weekend) | Floor area per occupant (m²) |
|---|---|---|
| `bakery` | 6-19 / 7-17 | 10 |
| `clinic` | 7-19 / 8-13 | 15 |
| `hotel` | 24-hour curve | 20 |
| `office` | 8-18 / closed | 15 |
| `restaurant` | 11-23 every day | 3 |
| `school` | 8-16 / closed, closed July and August | 5 |
| `supermarket` | 8-21 / 9-18 | 10 |
| `warehouse` | 7-18 / closed | 100 |

Public holidays are not modelled.

## Limitations to tell users

- Dwellings within a building are identical, and peaks scale linearly with the number of dwellings.
- Two buildings with the same type, household size, archetype and year produce identical profiles.
- Household-size statistics are specific to the Netherlands; other countries use generic figures.
- The occupancy and appliance data combine Dutch schedules with UK appliance statistics and are described by the occupancy package as a first pass.
- Presence is redrawn independently each hour for every household archetype except `working_couple`.
- `neighbour_status`, `attic_condition`, `cellar_condition` and `n_storeys` have no effect.
