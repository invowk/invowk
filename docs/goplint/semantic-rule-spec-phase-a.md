# goplint Semantic Rule Spec (Phase A)

This document defines the Phase A formalization layer for CFA-backed goplint categories.

Source of truth for machine-checkable semantics:

- `tools/goplint/spec/semantic-rules.v1.json`
- `tools/goplint/spec/schema/semantic-rules.schema.json`

## Scope

Phase A formalizes semantics for these categories:

- `unvalidated-cast`
- `unvalidated-cast-inconclusive`
- `use-before-validate-same-block`
- `use-before-validate-cross-block`
- `use-before-validate-inconclusive`
- `missing-constructor-validate`
- `missing-constructor-validate-inconclusive`

## Semantic Model

Each category is specified as a safety property over control-flow paths.

Common components:

1. State domain:
   - value requires validation
   - value validated
   - value escaped/consumed before validation

2. Outcome domain:
   - `safe`
   - `unsafe`
   - `inconclusive`

3. Inconclusive reason taxonomy:
   - `state-budget`
   - `depth-budget`
   - `recursion-cycle`
   - `unresolved-target`

4. Soundness default:
   - unresolved proof should not be upgraded to `safe`
   - inconclusive handling is explicit and policy-controlled (`error|warn|off`)

## Rule Families

### Cast Validation Family

Property:
- conversion from primitive input to validatable named type must be validated before externally meaningful use/return on all relevant paths.

Category mapping:
- unsafe proof -> `unvalidated-cast`
- uncertain proof -> `unvalidated-cast-inconclusive`

### Use-Before-Validate Family

Property:
- values requiring validation must not be consumed before validation on any relevant path.

Category mapping:
- same-block violation -> `use-before-validate-same-block`
- cross-block violation -> `use-before-validate-cross-block`
- uncertain proof -> `use-before-validate-inconclusive`

### Constructor-Validates Family

Property:
- constructors returning validatable values must ensure validation on all relevant return paths.

Category mapping:
- unsafe proof -> `missing-constructor-validate`
- uncertain proof -> `missing-constructor-validate-inconclusive`

## Contract Invariants

The semantic catalog must satisfy:

1. Category invariants:
   - category is known in goplint registry
   - baseline policy matches registry policy

2. Control-plane invariants:
   - enabling flags exist
   - run controls exist and are valid analyzer flags

3. Outcome invariants:
   - outcome values are from known domain
   - inconclusive reasons are from known reason taxonomy

4. Metadata invariants:
   - inconclusive categories declare required metadata keys
   - required keys are emitted by analyzer metadata helpers

5. Oracle invariants:
   - every CFA-backed category has must-report and must-not-report mappings
   - historical miss fixtures remain referenced by active tests

## Oracle Corpus Contract

The semantic catalog defines oracle mappings by fixture + symbol.

Rules:

- must-report mappings identify known violations or inconclusive outcomes.
- must-not-report mappings identify known-safe behavior for that category.
- historical fixtures are replayed via dedicated integration tests to prevent drift.

## Compatibility and Governance

Phase A does not change finding IDs or baseline format.

- stable IDs remain derived by existing ID functions.
- baseline suppressibility semantics remain registry-driven.
- Phase A adds validation of semantic intent, not suppression policy changes.
