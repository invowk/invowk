# goplint Semantic Rule Spec (Phase B)

Phase B extends the CFA semantic contract with an IFDS/IDE-style interprocedural core for:

- `unvalidated-cast`
- `unvalidated-cast-inconclusive`
- `use-before-validate-same-block`
- `use-before-validate-cross-block`
- `use-before-validate-inconclusive`
- `missing-constructor-validate`
- `missing-constructor-validate-inconclusive`

Normative machine-readable sources:

- `tools/goplint/spec/semantic-rules.v1.json`
- `tools/goplint/spec/schema/semantic-rules.schema.json`

## Phase B Additions

Each CFA rule now declares:

- `interproc_engine_modes`: allowed engine controls (`legacy`, `ifds`, `compare`)
- `fact_families`: IFDS fact families used by the rule
- `edge_function_tags`: IDE edge-function transitions used by the rule

## Runtime Control

Phase B introduces:

- `--cfg-interproc-engine=legacy|ifds|compare`

Compare mode is fail-closed for no-silent-downgrade policy:

- forbidden: `legacy=unsafe` and `ifds=safe`
- forbidden: `legacy=inconclusive` and `ifds=safe` without equivalent unsafe evidence

## Contract Invariants

- Category names and baseline policy remain unchanged.
- Inconclusive reason taxonomy remains unchanged (`state-budget`, `depth-budget`, `recursion-cycle`, `unresolved-target`).
- Required inconclusive metadata keys remain backward compatible.
