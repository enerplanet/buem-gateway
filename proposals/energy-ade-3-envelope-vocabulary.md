# Adopt CityGML Energy ADE 3.0 for the envelope vocabulary

**Status:** proposal, not approved. Targets `schemas/v6-draft/` only, which is
inert (`docs/versioning.md`). Nothing in this document changes API contract v5.
Requires a matching change in the buem fork; the two must land together.

**Audience:** developer.

## Why

The envelope element vocabulary is our own invention. `envelope.elements[].type`
takes `wall`, `roof`, `floor`, `window`, `door`, `ventilation`, and nothing
outside this contract defines those terms.

Two consequences:

- `floor` is ambiguous. It is used for a slab on grade, for a floor over an
  unheated cellar and for a ceiling under an unheated attic. These are three
  different thermal situations, currently distinguished only by the
  `b_transmission` reduction factor.
- Every consumer needs a translation layer between this contract and the
  CityGML-derived data it starts from, and that layer has to be documented and
  justified against the standard rather than referring to it.

Energy ADE 3.0 is a published extension of CityGML for building energy
modelling. Adopting its vocabulary removes the invention and resolves the
ambiguity, and it does so in the direction the upstream data already points:
the Netherlands 3D source data is CityJSON from the same group that maintains
the extension.

## Version targeted

| Item | Value |
|---|---|
| Upstream | `github.com/tudelft3d/Energy_ADE` |
| Commit | `c3a97f7`, 2026-08-11 |
| Schema file | `FME/xsd/Energy_ADE_3.0_beta7.xsd`, last updated 2025-09-23 |
| Base standard | CityGML 2.0. Porting to CityGML 3.0 is stated upstream as future work |

The commit is pinned here deliberately. Energy ADE 3.0 is under active
development and this is a beta schema, so the version a field name was drawn
from must be recoverable. See Risks.

## Scope

In scope: the envelope, the surface vocabulary, per-surface geometry and
thermal properties, openings, and the three zone scalars that have standard
names.

Out of scope this round: the Schedules, Occupancy, Devices and Resources
modules, and every hourly series. Comfort temperatures, ventilation use rates
and setback profiles have conforming homes in the Schedules module and are left
as they are, named as extensions, rather than adopted half way.

## The surface vocabulary

In Energy ADE 3.0 a thermal boundary is a CityGML boundary surface;
`ThermalZone.thermalBoundary` is typed `bldg:BoundarySurfacePropertyType`.
There is no separate thermal boundary class and no type enumeration. Four
surface types are added by the extension for thermal topology CityGML cannot
express; each extends `bldg:AbstractBoundarySurfaceType` and carries no
properties of its own.

| Current `type` | Proposed | Source |
|---|---|---|
| `wall` | `WallSurface` | CityGML |
| `roof` | `RoofSurface` | CityGML |
| `floor` (on ground) | `GroundSurface` | CityGML |
| `floor` (over cellar) | `BasementCeilingSurface` | Energy ADE 3.0 |
| `floor` (under attic) | `AtticFloorSurface` | Energy ADE 3.0 |
| `window` | `Window` | CityGML |
| `door` | `Door` | CityGML |
| `ventilation` | removed, see below | |

`PartyWallSurface` and `IntermediateFloorSurface` are also available and are not
adopted: this contract carries no party walls (they are excluded upstream) and
no interior partitions.

The specification requires the CityGML classes for openings: "For the
opening(s), the corresponding CityGML classes (Window or Door) must be used."
`WindowSurface` and `DoorSurface` are not the right mapping; they are boundary
surfaces of an opening rather than the opening itself.

`ventilation` is not a surface and is removed from the surface vocabulary. It
has no successor, because it never had an effect. The element's `air_changes`
is read into a component and echoed back in the response, but ventilation heat
loss is computed from the zone-level infiltration and use rates alone; nothing
maps the element's value into either. Leakage is `infiltrationRate` on the
zone, which already exists and is the only leakage figure that has ever
counted. The use rate stays an extension, with the Schedules module out of
scope.

`AtticFloorSurface` must map to the transmission-only path in the model, not to
the roof path, which adds opaque solar gains.

### What populates the two interior surface types

`BasementCeilingSurface` and `AtticFloorSurface` cannot come from the 3D
geometry, and no improvement to that data will change it. An LoD2 exterior
shell contains no interior boundaries, and a basement ceiling and an attic
floor are interior boundaries by definition. LoD2 sources emit exactly
`WallSurface`, `RoofSurface` and `GroundSurface`.

The distinction comes from the TABULA archetype, which is already where the
unheated-cellar and unheated-attic situations live and what `b_transmission`
encodes today.

The archetype data carries two floor variants and two roof variants, each with
its own `A_`, `U_` and `b_Transmission_` column; this is the structure the
model already reads to derive a component's U-value and reduction factor, and
`b_Transmission_*` is cited as the source for `b_transmission` in the current
schema. Alongside them, the German archetypes carry a
`Code_ConstructionBorder_` column per variant naming what that variant borders.
In the data in use its values give floor variant 1 bordering a cellar, floor
variant 2 bordering soil, roof variant 1 exterior and roof variant 2 unheated,
which is a one-to-one match with `BasementCeilingSurface`, `GroundSurface`,
`RoofSurface` and `AtticFloorSurface` respectively.

Those column names are given so the mapping can be checked against whichever
TABULA edition a deployment loads, rather than taken on assertion. The edition
is not pinned here the way the upstream standard is, and the value set should
be confirmed against the loaded data before the mapping is implemented.

This is more than a rename, and less than it may look. The model selects one
variant per component, the largest by area with a non-zero transmission
factor, and discards the other, so a building with both a cellar ceiling and a
slab on grade is reduced to a single floor element. That reduction is unchanged
by this proposal, and carrying both as separate surfaces is deferred; see
Deriving the surface class and Open items.

What the vocabulary adds now is that the surviving element states its actual
class, `BasementCeilingSurface` or `GroundSurface`, rather than an ambiguous
`floor` qualified only by a reduction factor. On the floor side that is a
labelling gain with no change to computed results. The result change comes from
the roof side; see Migration impact.

The explicit `Code_ConstructionBorder_` label is populated for German
archetypes only. Two further columns, `Code_CellarCond` and `Code_AtticCond`,
are populated for every country and every archetype, and TABULA's own
calculator uses them to estimate the same distinction. That gives a rule that
works without the label.

### Deriving the surface class

This is a producer rule. It belongs to whatever turns geometry plus an
archetype into surfaces, not to this contract, which carries only the
resulting class. A producer that cannot apply it sends the class explicitly.
The class is never inferred from `b_transmission` at the request boundary, for
the reason at the end of this section.

1. Pick the variant as today: largest area with a non-zero transmission
   factor, falling back to variant 1. The U-value and transmission factor come
   from the picked variant, unchanged. The class is decided separately.
2. **Floor.** If `Code_ConstructionBorder_Floor_<picked>` is set, `Cellar` is
   `BasementCeilingSurface` and `Soil` is `GroundSurface`. Otherwise use
   TABULA's own estimate, which reads a cellar condition of `-` as soil and
   anything else as a cellar: `-` is `GroundSurface`, anything else is
   `BasementCeilingSurface`.
3. **Roof.** If `Code_ConstructionBorder_Roof_<picked>` is set, `Ext` is
   `RoofSurface` and `Unh` is `AtticFloorSurface`. Otherwise follow the
   calculator's roof formula, which drops the roof from the envelope when the
   attic condition is `N`: `N` is `AtticFloorSurface`, anything else is
   `RoofSurface`.
4. Where the explicit label exists it wins over the estimate. The two do not
   always agree, and the disagreements are real buildings rather than noise.
5. One surface per LoD2 polygon this round. Where both variants carry a
   non-zero factor, the larger is kept, as today.

One deliberate deviation from the formulas as written: an empty value and a
stray `0` are both treated as `-`. Across the 2,147 archetype rows of the
twenty country tables in the loaded edition, `Code_CellarCond` holds `0` in 114
rows, 108 of them Swedish, and `Code_AtticCond` in a further three. Those rows
carry a transmission factor and a ground-corrected U-value consistent with a
slab on ground rather than a cellar ceiling.

**The transmission factor cannot substitute for the class.** Over those same
2,147 rows, the picked floor variant carries a factor of 0.5 in 669 archetypes
with no cellar and in 865 with an unheated cellar. The factor is the quantity;
the class comes from the adjacency columns. That is why step 1 decides the two
separately, and why the request boundary does not guess.

A caller not working from an archetype supplies the class itself, as it already
must supply `b_transmission`.

## Element and zone attributes

| Current | Proposed | Note |
|---|---|---|
| `area` | `opaqueSurfaceArea` or `totalSurfaceArea` | See Area convention |
| `azimuth` | `azimuth` | Unchanged name, convention now stated explicitly |
| `tilt` | `inclination` | Same convention, standard's name |
| `U` | `uValue` | On the construction |
| `g_gl` | `gValue` | **Different quantity and different number.** See Numeric hazards |
| `parent_id` | removed | Replaced by nesting |
| `window_to_wall_ratio` | extension | `openingToSurfaceRatio` is its named successor; see Extensions |
| `A_ref` | zone `area`, type `energyReferenceArea` | |
| `h_room` | zone `volume`, type `netVolume` | No height attribute exists. Height is derived as volume over area |
| `c_m` | zone `heatCapacity` | **Different quantity and different number.** See Numeric hazards |
| `n_air_infiltration` | zone `infiltrationRate` | Exact match |
| `residential_units` | `numberOfBuildingUnits` | Exact match |
| `building_type` (residential) | `bdgType` | Codelist literals replace the four residential codes |
| `building_type` (service) | extension | No faithful codelist target; see Extensions |

Zone volume replaces `h_room` rather than sitting beside it. Nothing in the
contract carries a volume today, and the standard has no height attribute, so
`h_room` would otherwise be derived from a value the caller does not send.
Volume is also what the 3D source data has directly, with height being the
proxy.

`bdgType` is adopted for residential buildings only. The codelist
(`xsd/codelists/BuildingTypeValue.xml`) holds ten values, and all four
residential types in use have faithful targets: "single-family house",
"terraced house", "multi-family house", "apartment block". Adopting the
attribute name while continuing to carry abbreviated codes would be naming what
we do not populate, so those four are mapped to literals in the contract and
back to internal codes in the model.

The service types are not adopted, and this is the one place where a faithful
mapping does not exist. Eight service types are accepted today; the codelist
offers six generic categories, of which only "office building" is an exact
match. Three would collapse onto "commercial building", "clinic" would become
"hospital", which is a different category of building, and "school" has no
target at all. Those eight values select occupancy and equipment profiles, so
collapsing them changes results rather than only names. The service path
therefore keeps `building_type` as a declared extension.

## Area convention

The standard names both areas. `totalSurfaceArea` is "the total, gross value of
a surface area, including the area of openings". `opaqueSurfaceArea` is "the
area excluding the openings". Where opening areas are unknown,
`openingToSurfaceRatio` derives them, and the specification notes it
"corresponds to the more commonly known window-to-wall ratio".

Our current rule requires net opaque areas when explicit openings are listed,
because the model does not subtract an opening from its parent surface. That
rule conforms; it is one of the two named options rather than a deviation.

A surface carries exactly one of `opaqueSurfaceArea` or `totalSurfaceArea`:

- `opaqueSurfaceArea` permits nested openings, and is required when any are
  present.
- `totalSurfaceArea` permits none. The model synthesises the openings, from the
  building-level opening ratio extension when one is given and from the
  archetype ratios otherwise.

`totalSurfaceArea` combined with explicit nested openings is rejected, not
silently subtracted. Both forms are expressed in the schema as a `oneOf` with a
conditional, so this is enforced by validation rather than by hand-written
checks on either side of the contract.

`openingToSurfaceRatio` is not part of either form. The standard defines it per
surface, nothing produces per-surface ratios today, and the building-level
figure remains an extension. It is recorded there as the named successor.

## Numeric hazards

Two adopted attributes share a name-like relationship with the field they
replace while meaning a different quantity. Both produce a plausible wrong
answer rather than an error, so both are stated as failure modes first.

The general rule that follows from them: rename whenever a meaning changes,
never redefine a name in place. Because neither old spelling is accepted as an
alias, and the element rejects unknown properties, a payload that carries the
old name fails validation naming the field. What validation cannot catch is a
caller who renames the key and carries the number across unchanged, which is
why the conversion belongs in the migration note at the field.

### Solar gain

**Failure mode.** `g_gl: 0.60` rewritten as `gValue: 0.60` gives roughly 25 per
cent more window solar gain.

The standard defines `gValue` as "the coefficient used to measure the
transmittance of solar gain through glazing, considering also the frame".
`g_gl` is glazing only, with the frame applied separately as a frame fraction.
So `gValue` equals `g_gl` multiplied by one minus the frame fraction.

The model applies no frame fraction to a window that carries `gValue`, because
the standard's value already includes the frame. The frame fraction applies
only to synthesised openings, where the synthesis emits a frame-inclusive
value itself. A caller sending both `gValue` and a frame fraction should not
expect both to be applied to that window.

The building-level glazing default keeps its current glazing-only meaning and
is **not** renamed. It differs from the element-level `gValue` by the frame
fraction despite the similar spelling, so a value migrated from it must not be
converted.

### Thermal capacity

**Failure mode.** `c_m: 175` rewritten as `heatCapacity: 175` gives a building
with roughly one hundredth of its thermal mass, for a hundred-square-metre
building. The resulting hourly profile is plausible, not obviously wrong.

`c_m` is internal thermal capacity per square metre of floor area, in
kJ/(m2K). `heatCapacity` is the zone total. So `heatCapacity` equals `c_m`
multiplied by the reference area, and the unit changes to kJ/K.

## Openings nest inside their parent surface

Openings become nested within the surface that contains them, replacing the
flat list with `parent_id`.

This is the only shape in which the model's existing behaviour needs no
documented override. The model forces every opening to inherit its parent
surface's azimuth and tilt, correcting a caller-supplied value rather than
rejecting it. A nested opening has nowhere to carry an orientation, so a value
that would be discarded cannot be supplied.

It also removes rather than checks three error classes: a parent reference that
resolves to nothing, a parent that is not a wall or roof, and an opening
parented to another opening. Under a flat list these need a new validation
rule, and a dangling reference currently surfaces as an error from inside the
solver after the run has started rather than at the request boundary.

Consequently `bdgOpnAzimuth` and `bdgOpnInclination` are not adopted: the
orientation comes from the enclosing surface. A skylight nested in a
`RoofSurface` inherits the roof inclination, which is already how the model
behaves.

## Declared as extensions

These have no name in Energy ADE 3.0 at this scope and are documented as
extensions to it rather than mapped onto approximate standard names. The
declaration is what keeps the conformance claim true.

| Extension | Why it has no standard name |
|---|---|
| `b_transmission` | The standard expresses an unheated adjacent space structurally, by surface class and by building-level thermal status. The reduction factor quantifying it is ours |
| `window_to_wall_ratio` | Building-level. The per-surface `openingToSurfaceRatio` is its named successor; nothing produces per-surface ratios today |
| `F_sh_vert`, `F_sh_hor` | Sky and ground view factors are geometric obstruction, not ISO 13790 shading reduction factors, and neither is computed from the other |
| `F_f` | The standard's glazing ratio is the per-window equivalent; one building-level value is held instead. Applies to synthesised openings only, since an element-level `gValue` is already frame-inclusive |
| `F_w`, `F_red_htr`, `design_T_min`, `ventControl` | ISO constants and solver switches |
| `comfortT_lb`, `comfortT_ub`, `setback_profile`, `n_air_use` | Conforming homes in the Schedules module, out of scope this round |
| `window_U`, `window_g_gl`, `door_U` | Synthesis defaults; a default construction for synthesised openings is the conforming form. `window_g_gl` stays glazing-only and is not renamed, so it differs from element-level `gValue` by the frame fraction |
| `building_type` (service buildings) | Eight service types select occupancy and equipment profiles. The codelist offers six generic categories with one exact match, and no target for "school", so mapping onto it would change results rather than names |
| `include_dhw`, `cooking_carrier`, `elec_load_as_gain`, `region_code`, `country`, `construction_period` | Occupancy, Devices and Resources modules, out of scope |

## Not adopted

| Attribute | Reason |
|---|---|
| `surfaceGeometry` | Per-surface polygons are not carried anywhere in this pipeline. The only geometry in the contract is a building centroid |
| `isHeated`, `isCooled` | The model computes and returns both loads unconditionally. The contract already rejects a request asking for conditional cooling. Adopting these would mean either contradicting the standard's defaults on day one or claiming behaviour that does not exist. Revisit when conditional cooling is implemented |
| `PartyWallSurface`, `IntermediateFloorSurface` | No party walls or interior partitions in this contract |
| Multi-zone relations | The model is single zone. `coincidesWithLod2Hull` is the standard's own flag for one zone equal to the building hull, which is this case |

## Conventions stated locally

Energy ADE 3.0 does not restate the angle conventions; the schema types them as
plain angles with no documentation. The only written definition is in the 1.0
feature catalogue: azimuth of the surface normal with zero at north and
horizontal surfaces at zero, and inclination zero for horizontal planes.

This matches the convention the model already uses, so no conversion is
introduced. Because the definition lives only in a superseded version, both
conventions are stated in the field descriptions here rather than referenced.
Units are likewise not fixed upstream for heat capacity or infiltration rate
and are stated in this contract.

## Migration impact

- Every caller edits the surface vocabulary. Two field migrations need a value
  converted as well as a key renamed, `g_gl` to `gValue` and `c_m` to
  `heatCapacity`; see Numeric hazards. The building-level glazing default is
  not renamed and its value must not be converted.
- The nesting change reaches only callers that send explicit openings. A caller
  relying on synthesised openings is unaffected by it, and a client whose
  surface picking is keyed on the geometry source rather than on these elements
  is unaffected structurally.
- Clients mapping surface types should note that reading the type verbatim,
  with a fallback for an unrecognised value, needs no change beyond new labels.
- In the model, nesting is confined to the request converter, which walks one
  nested loop instead of copying a parent reference; the internal component
  shape does not change, so the solver and the synthesis are untouched by it.
  `gValue` is separate and does touch both the window loop and the synthesis.
- The model's element type map and this schema must change in the same release.
  The schema enum is currently the only thing preventing an unrecognised type
  from being dropped with a warning and the building running with an empty
  envelope.
- **`AtticFloorSurface` changes results, and by an unmeasured amount.** This is
  a correction rather than a preference, and it is listed here so it is not
  mistaken for part of the rename. Today the picked roof variant's U-value and
  transmission factor are applied to every LoD2 roof polygon, so a building
  with an unheated attic is computed as a sloped, sun-exposed roof carrying the
  attic ceiling's U-value and receiving opaque solar gain through it. Under the
  class the surface takes zero inclination, the ground-surface area, and the
  transmission-only path with no opaque solar gain. Roughly a third of
  archetypes across all countries carry the attic condition that triggers this.
  The magnitude has not been measured and should be before the change ships.

## Risks

- **Beta schema under active development.** The specification and the schema
  already disagree in at least one place, on the capitalisation of the
  hull-coincidence flag, and an attribute was renamed between 1.0 and 3.0.
  The pinned commit above is the mitigation, not a guarantee.
- **Built on CityGML 2.0.** Porting to CityGML 3.0 is upstream future work.
- **Licensing.** Upstream software is Apache 2.0; the documentation and UML
  diagrams are CC BY-NC-SA 4.0. Using attribute names is not redistribution of
  those documents, but the split should be a known factor rather than a
  discovery.

## Open items

- Whether the model subtracts explicit openings from a parent surface, which
  would allow gross areas on both paths and remove the rejected combination
  under Area convention. Not required by this proposal.
- Whether a single LoD2 polygon should be split by the archetype's own area
  ratio when both variants carry a non-zero transmission factor, so that a
  cellar ceiling and a slab on grade, or an exterior roof and an attic ceiling,
  can both be carried. The rule above keeps the larger, as today. Splitting is
  possible and is deferred.
- Measuring the `AtticFloorSurface` result change before it ships.
- Whether the missing service-building categories are worth raising upstream.
  The codelist is actively developed and has no target for several types in
  use, so the extension declared here may be reducible later rather than
  permanently.
- Promotion sequencing against `proposals/buem-fork-contract-migration.md`. The
  contract is currently enforced by hand-written checks here and by validation
  against a pinned copy in the fork. Two copies enforced by two mechanisms is a
  larger liability under this change than under a field rename.
