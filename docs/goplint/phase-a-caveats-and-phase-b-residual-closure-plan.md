# goplint Phase A Caveats and Conditional Phase B Residual Closure Plan

Date: 2026-03-05  
Correctness posture: **Soundness-first**  
Roadmap source: [State-of-the-Art Soundness Roadmap](./state-of-the-art-soundness-roadmap.md)

## Objective

Close the known minor caveats left after Phase A implementation, and include a closure plan for unresolved Phase B issues only if the remaining roadmap phases (C and D) do not already cover them.

## Conditional Inclusion Decision (Phase B)

Decision rule:
- Include Phase B residual plan in this document if and only if Phases C and D do not address the unresolved Phase B issues.

Evaluation against roadmap:
- Phase C focuses on SMT feasibility checks and CEGAR refinement.
- Phase D focuses on alias/precision tiers and optional relational domains.
- Neither phase introduces or completes a distinct IFDS/IDE interprocedural core for cast/UBV/constructor checks.

Result:
- **Condition satisfied**.
- This document includes both:
  - Phase A caveat remediation plan.
  - Phase B residual closure plan.

## Phase A Caveats Remediation Plan

## Scope

Known caveats to close:
1. Semantic spec tests parse schema and catalog but do not validate the catalog against the JSON Schema with a schema engine.
2. CFA category coverage sync relies on a hardcoded expected list.
3. Historical-miss replay asserts category presence but not symbol-level expectation quality.

## Workstream A1: Enforce JSON Schema Validation in Tests

Goal:
- Make schema validation explicit and executable, not just structural parsing.

Implementation plan:
1. Add schema-engine validation helper in `tools/goplint/goplint/semantic_spec.go`:
   - load raw catalog bytes;
   - load raw schema bytes;
   - validate catalog instance against `semantic-rules.schema.json`;
   - return structured validation errors with schema path + instance path.
2. Add test coverage in `tools/goplint/goplint/semantic_spec_registry_test.go`:
   - positive test for current catalog;
   - negative test fixture with known schema violation.
3. Keep existing semantic invariant checks as a second layer:
   - schema validation first;
   - semantic/business invariants second.

Acceptance criteria:
- `TestSemanticSpecSchemaAndCatalogParse` fails if the catalog breaks JSON Schema constraints.
- Negative fixture test fails for the right reason and points to the violating field path.

## Workstream A2: Replace Hardcoded CFA Category Sync List

Goal:
- Ensure new CFA-backed categories cannot be added without semantic contract coverage.

Implementation plan:
1. Add explicit category scope metadata in `tools/goplint/goplint/categories.go`:
   - add a scope marker (for example: `IsCFASemanticScoped`).
2. Mark CFA-backed categories in the registry with that scope marker:
   - cast, UBV, constructor-validate, and their inconclusive variants.
3. Update `semantic_spec_registry_test.go`:
   - replace hardcoded list helper with a registry-derived CFA category set;
   - assert one-to-one sync between registry CFA set and semantic catalog CFA set.

Acceptance criteria:
- Test fails when a registry CFA category is missing from semantic catalog.
- Test fails when semantic catalog contains CFA category not in registry scope.

## Workstream A3: Strengthen Historical-Miss Replay Oracles

Goal:
- Prevent weak regressions where category-only presence passes while symbol-level behavior drifts.

Implementation plan:
1. Extend semantic catalog with historical replay expectations:
   - add `historical_miss_oracles` keyed by fixture;
   - each entry defines expected category and symbol-level must-report set;
   - optional must-not-report set for nearby false-positive guardrails.
2. Update `tools/goplint/goplint/cfa_historical_ast_test.go`:
   - assert symbol-level matches using the same diagnostic-to-symbol mapping helper used by semantic oracles;
   - keep category presence assertion as a baseline check.
3. Add a small readme/update note in `tools/goplint/goplint/testdata/semantic_oracles/README.md` documenting historical oracle semantics.

Acceptance criteria:
- Historical replay fails if expected symbol is not reported.
- Historical replay fails if protected symbol is newly reported in forbidden category.

## Phase A Verification and Gates

Required checks:
- `./tools/goplint/scripts/check-semantic-spec.sh`
- `go test ./tools/goplint/goplint -run '^TestSemanticSpec|^TestSemanticSpecHistorical'`
- `make check-baseline`

Optional confidence checks:
- `go test -race ./tools/goplint/goplint`

## Conditional Phase B Residual Closure Plan

Rationale:
- Remaining roadmap phases (C, D) improve feasibility/refinement/precision, but do not complete the unresolved core Phase B requirement: a materially distinct IFDS/IDE interprocedural engine path.

## Residual Issues to Close

1. IFDS path delegates to legacy evaluators in key solver methods.
2. Supergraph/fact/edge-function artifacts are present but not used as the actual decision engine.
3. Runtime compare-mode downgrade checks do not propagate equivalent-unsafe evidence from adapters.
4. Rollout/default strategy drifted from staged legacy->compare->ifds progression.

## Workstream B1: Implement Distinct IFDS Propagation Engine

Goal:
- Make IFDS mode semantically distinct from legacy while preserving conservative outcomes.

Implementation plan:
1. Refactor `tools/goplint/goplint/cfa_ifds_solver.go`:
   - replace IFDS methods that call legacy directly with real fact propagation over supergraph;
   - compute outcomes from propagated fact states instead of legacy function return.
2. Keep legacy engine intact behind `cfg-interproc-engine=legacy`.
3. Keep compare mode dual-run behavior to detect regressions.

Acceptance criteria:
- IFDS methods no longer call legacy evaluators for main decision computation.
- Compare-mode tests still pass with no forbidden downgrades.

## Workstream B2: Wire Supergraph into Real Call/Return Analysis

Goal:
- Convert supergraph from placeholder artifact to solver substrate.

Implementation plan:
1. Upgrade `cfa_interproc_graph.go`:
   - add callee entry and return-exit linkage where target resolution succeeds;
   - keep unresolved call-to-return fallback with explicit reason metadata.
2. Thread graph nodes/edges into IFDS transfer flow:
   - intraprocedural transfer;
   - call flow transfer;
   - return flow transfer;
   - summary edge fallback.

Acceptance criteria:
- Graph construction tests cover resolved call edges and unresolved fallback edges.
- IFDS solver consumes graph edges in outcome synthesis.

## Workstream B3: Integrate IDE Edge Functions into Solver State Transitions

Goal:
- Use IDE transforms in real path-state updates instead of metadata-only tagging.

Implementation plan:
1. Connect `cfa_ide_edgefunc.go` operators to transfer steps:
   - validation transition;
   - escape-before-validate transition;
   - consume-before-validate transition;
   - conservative join semantics for merge points.
2. Ensure `edge_function_tags` in semantic spec remain aligned with runtime transitions.

Acceptance criteria:
- Integration tests demonstrate edge-function-driven state updates alter IFDS outcomes on targeted fixtures.
- Unit tests still validate compose/join algebra.

## Workstream B4: Fix Compare-Mode Equivalent-Unsafe Handling

Goal:
- Match runtime compare behavior to the documented no-silent-downgrade policy.

Implementation plan:
1. In adapters (`cfa_cast_validation.go`, `cfa_closure.go`, `analyzer_constructor_validates.go`):
   - compute and pass `hasEquivalentUnsafe` based on IFDS unsafe findings for the same normalized identity.
2. Keep classifier logic in `cfa_interproc_compat.go` unchanged except for any normalization helpers needed.
3. Expand compare tests for equivalent-unsafe allow case in end-to-end analyzer paths.

Acceptance criteria:
- Runtime compare mode allows `legacy=inconclusive -> ifds=safe` only when equivalent unsafe evidence exists.
- Forbidden downgrade cases still fail hard.

## Workstream B5: Restore Staged Rollout Contract

Goal:
- Re-align engine default progression with explicit rollout gates.

Implementation plan:
1. Temporarily set default back to `legacy` until IFDS distinctness and compatibility are proven.
2. Keep CI running:
   - semantic spec gate;
   - IFDS compatibility gate;
   - benchmark threshold gate.
3. Define promotion criteria to flip default to `ifds`:
   - at least one full CI cycle with compare-mode clean results;
   - no benchmark threshold regression;
   - no baseline churn caused by silent downgrades.

Acceptance criteria:
- Default and rollout state are explicitly documented in `tools/goplint/README.md`.
- Default flip is done only after documented promotion criteria are met.

## Phase B Verification and Gates

Required checks:
- `./tools/goplint/scripts/check-semantic-spec.sh`
- `./tools/goplint/scripts/check-ifds-compat.sh`
- `./tools/goplint/scripts/check-cfg-bench-thresholds.sh`
- `go test ./tools/goplint/goplint`
- `go test -race ./tools/goplint/goplint`
- `make check-baseline`

## Sequencing

Recommended execution order:
1. A1 -> A2 -> A3.
2. B4 (policy consistency) before B1/B2/B3 changes reach default path.
3. B1 -> B2 -> B3.
4. B5 rollout transition and default flip decision.

## Out of Scope

Still deferred to roadmap Phases C and D:
- SMT witness feasibility disambiguation.
- CEGAR refinement loops.
- Alias-tier and relational-domain precision upgrades.
