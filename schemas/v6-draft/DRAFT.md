# API contract v6 (draft)

This folder is the **in-development** next version of the BUEM-EnerPlanET
request/response contract. It is inert: nothing in buem-gateway reads or
validates against it. `schemas/v5/` is the current production contract.

Edit these files freely while v6 is being designed. Promotion steps are in
[`docs/versioning.md`](../../docs/versioning.md#promoting-v6-draft-to-v6): rename
to `schemas/v6/`, bump `CURRENT_VERSION_DIR` and `APIVersion`, update the
hand-written `requireEnvelope`/`requireWeather` checks (v6 requires all four
weather variables, and requires envelope and weather only when heating or
cooling is selected in `outputs`), update the version docs,
run `python scripts/validate_schemas.py` and `go test ./...`, re-sync the
buem-model copy.

Started as a copy of v5. Record every intended v6 change in `CHANGELOG.md` under
an `## Unreleased` heading as it is made.

buem-model pins this draft, not v5, while output selection is being built, so
that it can accept requests without envelope or weather. That pin is
temporary and moves to `schemas/v6/` on promotion.
