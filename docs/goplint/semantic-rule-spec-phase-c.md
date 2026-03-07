# goplint Semantic Rule Spec (Phase C)

Phase C extends the Phase B CFA contract with bounded witness feasibility and
counterexample-guided refinement for:

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

## Phase C Additions

Each CFA rule now declares:

- Phase C run controls:
  - `cfg-feasibility-engine`
  - `cfg-refinement-mode`
  - `cfg-refinement-max-iterations`
  - `cfg-feasibility-max-queries`
  - `cfg-feasibility-timeout-ms`
- `refinement_statuses`: allowed refined outcome classes for emitted findings.
- `required_meta_on_refinement`: additive metadata keys required on refined findings.

## Runtime Control

Phase C introduces these opt-in controls:

- `--cfg-feasibility-engine=off|smt`
- `--cfg-refinement-mode=off|once|cegar`
- `--cfg-refinement-max-iterations=<n>`
- `--cfg-feasibility-max-queries=<n>`
- `--cfg-feasibility-timeout-ms=<n>`

Current rollout contract:

- `off + off` preserves Phase B behavior.
- Phase C requires `--cfg-interproc-engine=ifds`.
- `unknown`, timeout, and unsupported predicates can never become `safe`.

## Emission Semantics

Refined findings keep the existing category names and stable finding IDs while
adding metadata:

- `cfg_feasibility_engine`
- `cfg_feasibility_result`
- `cfg_refinement_status`
- `cfg_refinement_iterations`
- `cfg_refinement_trigger`
- `cfg_refinement_witness_hash`

The internal findings stream also emits `kind=refinement-trace` JSONL records
for Phase C-evaluated non-safe witnesses, including refined and retained
unrefined outcomes, so gates can audit outcome changes even when no new
user-facing diagnostic is emitted.

## Contract Invariants

- Public category names and finding IDs remain unchanged.
- `cfg_inconclusive_reason` stays backward compatible with Phase B.
- Baseline generation remains pinned to legacy mode during Phase C rollout.
- `proven-safe` is an internal refinement status, not a new user-facing category.
