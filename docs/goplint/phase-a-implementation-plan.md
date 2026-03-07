# Phase A: Semantic Formalization and Oracle Hardening

## Objective

Implement Phase A from the goplint soundness roadmap by adding machine-checkable semantic contracts and a property-oriented oracle corpus for CFA-backed categories.

This plan operationalizes the roadmap deliverables into concrete repository artifacts, tests, and CI checks.

## Non-Goals (Phase B+)

This phase does not implement:

- IFDS/IDE interprocedural solver replacement.
- SMT-based feasibility checks.
- CEGAR refinement loops.
- Alias-tier engine upgrades.

## Current Gaps vs Required Deliverables

Phase A requirements from the roadmap:

- Rule-spec document mapping each category to formal state and transfer rules.
- Property-focused test corpus with must-report and must-not-report proofs.
- Explicit soundness contracts for `inconclusive` handling.

Current gaps in `tools/goplint`:

1. No machine-readable semantic contract currently tied to category registry.
2. No contract tests ensuring rule semantics stay in sync with flags and outcome metadata.
3. No single oracle index mapping CFA categories to must-report and must-not-report fixtures.
4. Legacy historical fixtures (`*_nocfa_*`) are present but not referenced by integration tests.

## Artifact Plan

Phase A adds the following artifacts:

1. Normative semantic document:
   - `docs/goplint/semantic-rule-spec-phase-a.md`

2. Machine-readable semantic contract:
   - `tools/goplint/spec/semantic-rules.v1.json`
   - `tools/goplint/spec/schema/semantic-rules.schema.json`

3. Contract loader/validator:
   - `tools/goplint/goplint/semantic_spec.go`

4. Contract tests:
   - `tools/goplint/goplint/semantic_spec_registry_test.go`
   - `tools/goplint/goplint/semantic_spec_runconfig_test.go`
   - `tools/goplint/goplint/semantic_spec_outcome_test.go`

5. Oracle index:
   - `tools/goplint/goplint/testdata/semantic_oracles/README.md`

6. Historical-miss replay and AST-compat fixture coverage:
   - `tools/goplint/goplint/cfa_historical_ast_test.go`

7. CI/script gate:
   - `tools/goplint/scripts/check-semantic-spec.sh`

## Machine-Checkable Semantic Contract

The semantic catalog must be versioned (`v1`) and include, per rule category:

- `category`
- `family`
- `entrypoints`
- `enabled_by_flags`
- `run_controls`
- `traversal_mode`
- `state_domain`
- `outcome_domain`
- `inconclusive_reasons` (for inconclusive categories)
- `required_meta_on_inconclusive` (for inconclusive categories)
- `baseline_policy`

The contract test suite must enforce:

1. Rule categories are known in the canonical registry.
2. Rule baseline policy matches registry baseline policy.
3. Rule controls and enabling flags exist in analyzer flags.
4. Inconclusive reasons use known reason tags.
5. Required inconclusive metadata keys are emitted by current reporting helpers.

## Oracle Hardening Matrix

CFA-backed categories covered in Phase A:

- `unvalidated-cast`
- `unvalidated-cast-inconclusive`
- `use-before-validate-same-block`
- `use-before-validate-cross-block`
- `use-before-validate-inconclusive`
- `missing-constructor-validate`
- `missing-constructor-validate-inconclusive`

For each category, maintain at least:

- one must-report fixture/symbol
- one must-not-report fixture/symbol

Historical fixtures must be replayed via integration tests and listed in the semantic catalog `historical_miss_fixtures` set.

## Acceptance Gates

`OracleCoverage`
- Every CFA-backed category has at least one must-report and one must-not-report mapping.

`HistoricalMissReplay`
- Every fixture listed in `historical_miss_fixtures` is referenced by at least one integration test.

`InconclusiveContract`
- Inconclusive categories declare reason tags and required metadata keys.
- Contract tests confirm reason and metadata compatibility with analyzer helpers.

`RegistrySync`
- CFA category set in semantic catalog remains synchronized with category registry and baseline policy.

`PerformanceEnvelope`
- Existing CFG benchmark thresholds remain green (`check-cfg-bench-thresholds.sh`).

## Rollout Sequence

1. Add semantic docs + machine-readable catalog/schema.
2. Add semantic spec loader + validation logic.
3. Add registry/run-config/outcome contract tests.
4. Add oracle index and historical fixture replay tests.
5. Add script and Makefile target for semantic-spec gate.
6. Update goplint README with Phase A references and command.

## Risks and Mitigations

Risk: semantic drift between contract and runtime behavior.
- Mitigation: strict contract tests and registry-policy sync checks.

Risk: false confidence from sparse oracle mappings.
- Mitigation: enforce must-report and must-not-report mappings per CFA category.

Risk: historical regressions hidden by orphan fixtures.
- Mitigation: replay historical fixtures in active integration tests.

Risk: policy metadata drift for inconclusive findings.
- Mitigation: required metadata-key assertions tied to helper outputs.

## Commands and Verification Checklist

Primary checks:

```bash
cd tools/goplint && go test ./goplint
cd tools/goplint && go test -race ./goplint
./tools/goplint/scripts/check-semantic-spec.sh
./tools/goplint/scripts/check-cfg-bench-thresholds.sh
```

Repository-level regression gate (recommended after goplint semantic changes):

```bash
make check-baseline
```
