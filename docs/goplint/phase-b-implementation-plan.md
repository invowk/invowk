# Phase B: IFDS/IDE-Style Interprocedural Core

Date: 2026-03-05  
Correctness posture: **Soundness-first**  
Roadmap source: [State-of-the-Art Soundness Roadmap](./state-of-the-art-soundness-roadmap.md#phase-b-ifdside-style-interprocedural-core)

## Objective

Implement Phase B by introducing an IFDS/IDE-style interprocedural analysis core for these CFA-backed categories:

- `unvalidated-cast`
- `unvalidated-cast-inconclusive`
- `use-before-validate-same-block`
- `use-before-validate-cross-block`
- `use-before-validate-inconclusive`
- `missing-constructor-validate`
- `missing-constructor-validate-inconclusive`

The new core must support compatibility comparison with the current engine and enforce no-silent-downgrade behavior under deeper recursion and call complexity.

## Non-Goals (Phase C+)

Phase B does not implement:

- SMT feasibility checking of witness paths.
- CEGAR refinement loops.
- Alias-tier upgrades beyond the current target/slot matching and summary semantics.
- Relational numeric domains.

## Phase A Handoff and Preconditions

Phase B starts from Phase A artifacts and contracts already present:

1. Semantic contract and schema:
   - `tools/goplint/spec/semantic-rules.v1.json`
   - `tools/goplint/spec/schema/semantic-rules.schema.json`
2. Semantic contract tests:
   - `tools/goplint/goplint/semantic_spec_*_test.go`
3. Existing CFA outcome and witness taxonomy:
   - `pathOutcome`, `pathOutcomeReason*`, witness metadata helpers in `cfa_outcome.go`
4. Existing policy controls:
   - `--cfg-inconclusive-policy`
   - `--cfg-witness-max-steps`
5. Existing benchmark gate:
   - `tools/goplint/scripts/check-cfg-bench-thresholds.sh`

These contracts remain normative in Phase B; the solver architecture changes, but governance semantics and category naming stay stable.

## Current Gaps vs Phase B Deliverables

Phase B roadmap deliverables:

- New interprocedural solver path for cast/UBV/constructor-validates.
- Fact domain definitions and edge-function semantics.
- Compatibility mode for comparing old/new engines.

Current gaps in `tools/goplint/goplint`:

1. Interprocedural reasoning is selective and summary-based (`cfa_summary.go`), not a unified IFDS/IDE solver.
2. Cast/UBV/constructor flows have partially duplicated traversal logic (`cfa_cast_validation.go`, `cfa_ubv.go`, `analyzer_constructor_validates_cfa.go`).
3. No runtime compatibility mode exists for engine-level old/new parity checks.
4. No explicit engine-level downgrade detector guards against `legacy=unsafe` becoming `new=safe`.
5. Semantic contract lacks explicit interprocedural solver metadata and edge-function model.

## Implementation Touchpoint Map (Current Code)

Phase B must be anchored to these current seams:

1. Cast-validation CFA flow:
   - entrypoint `inspectUnvalidatedCastsCFA` in `cfa_cast_validation.go`
   - cast collection in `cfa_collect.go`
   - shared path engine calls in `cfa.go`
2. UBV flow:
   - in-block and cross-block outcome logic in `cfa_ubv.go`
   - UBV mode control and auto-enable behavior in `flags.go` and `analyzer_run.go`
3. Constructor-validates CFA flow:
   - CFA path outcomes in `analyzer_constructor_validates_cfa.go`
   - post-traversal integration in `analyzer_constructor_validates.go`
4. Shared traversal core:
   - `cfgTraversalContext` and SCC memoization in `cfa_traversal.go`
   - DFS outcome kernel + budget controls in `cfa.go` and `cfa_budget.go`
5. Existing interprocedural hooks:
   - callee summary derivation/cache in `cfa_summary.go`
6. Outcome/reporting contracts:
   - outcome reason and witness helpers in `cfa_outcome.go`
   - policy-aware reporting in `cfa_reporting.go`
7. Governance/baseline contracts:
   - category policy registry in `categories.go`
   - stable finding identity and sink behavior in `finding.go` and `finding_sink.go`

## Target Architecture (Phase B)

### 1. Unified interprocedural supergraph

Build a normalized analysis graph that represents:

- intra-procedural flow edges for CFG blocks/nodes,
- call edges from callsite to callee entry with slot mapping,
- return edges from callee exits back to caller return sites,
- call-to-return summary edges for unresolved/external calls.

The graph must reuse existing call-target discovery helpers (receiver/arg slot matching) and preserve current conservative behavior when target resolution is incomplete.

### 2. IFDS fact domain (finite facts)

Define a shared fact domain with check-family tags:

1. Cast-validation facts:
   - `CastNeedsValidate(origin, target, typeKey)`
2. UBV facts:
   - `UBVNeedsValidateBeforeUse(origin, target, typeKey, mode)`
3. Constructor facts:
   - `CtorReturnNeedsValidate(ctor, returnTypeKey)`
4. Special zero fact:
   - `ZeroFact` for IFDS reachability plumbing.

Fact identity must remain deterministic and compatible with existing finding-ID components (`PackageScopedFindingID` + stable source anchors).

### 3. IDE edge-function layer (environment transforms)

Add edge functions that model state transforms over facts:

- `NeedsValidate -> Validated` when Validate semantics are proven.
- `NeedsValidate -> EscapedBeforeValidate` for UBV escape semantics.
- `NeedsValidate -> ConsumedBeforeValidate` for UBV use-before-validate.
- `NeedsValidate` preserved across unknown call effects unless safely discharged.

Join semantics must be conservative:

- conflicting transforms join to the least optimistic state,
- unknown/unresolved effects cannot produce `safe` by default.

### 4. Outcome synthesis and reporting

Translate IFDS/IDE results into existing category emissions and metadata:

- `unsafe` categories remain unchanged.
- `inconclusive` categories keep reason taxonomy (`state-budget`, `depth-budget`, `recursion-cycle`, `unresolved-target`).
- witness metadata keys stay compatible with existing consumers and baseline workflows.

## Proposed Artifact Plan

Planned new files:

1. Interprocedural core:
   - `tools/goplint/goplint/cfa_interproc_graph.go`
   - `tools/goplint/goplint/cfa_ifds_domain.go`
   - `tools/goplint/goplint/cfa_ifds_solver.go`
   - `tools/goplint/goplint/cfa_ide_edgefunc.go`
   - `tools/goplint/goplint/cfa_interproc_result.go`
2. Compatibility and diffing:
   - `tools/goplint/goplint/cfa_interproc_compat.go`
3. Tests for the new core:
   - `tools/goplint/goplint/cfa_ifds_solver_test.go`
   - `tools/goplint/goplint/cfa_ide_edgefunc_test.go`
   - `tools/goplint/goplint/cfa_interproc_compat_test.go`

Planned modified files:

1. Control-plane and flags:
   - `tools/goplint/goplint/flags.go`
   - `tools/goplint/goplint/analyzer_run.go`
2. Check adapters:
   - `tools/goplint/goplint/cfa_cast_validation.go`
   - `tools/goplint/goplint/cfa_ubv.go`
   - `tools/goplint/goplint/analyzer_constructor_validates_cfa.go`
3. Semantic contracts/docs:
   - `tools/goplint/spec/semantic-rules.v1.json`
   - `tools/goplint/spec/schema/semantic-rules.schema.json`
   - `docs/goplint/semantic-rule-spec-phase-b.md` (new)
   - `tools/goplint/README.md` (soundness docs and flags)
4. CI/scripts:
   - `tools/goplint/scripts/check-ifds-compat.sh` (new)
   - `Makefile` (`check-ifds-compat` target)

## Control-Plane Design (Compatibility Mode)

Introduce an engine selector run control:

- `--cfg-interproc-engine=legacy|ifds|compare`

Phase-B rollout behavior:

1. `legacy`:
   - fallback engine and baseline compatibility mode.
   - `make check-baseline`/`make update-baseline` pin `-cfg-interproc-engine=legacy` for deterministic suppression behavior.
2. `ifds`:
   - run only the new solver path (current default).
3. `compare`:
   - run both engines, compute normalized diff, and fail on forbidden downgrade classes (`check-ifds-compat` gate).

Normalization contract for `compare`:

- compare by `(category, finding-id)` plus normalized outcome class,
- treat `legacy=unsafe` and `ifds=safe` as hard downgrade,
- treat `legacy=inconclusive` and `ifds=safe` as downgrade unless there is an equivalent `unsafe` finding proving stronger signal,
- allow `ifds` to produce additional `unsafe`/`inconclusive` findings (soundness-first).

## Detailed Work Breakdown

### Workstream 1: Supergraph and call/return modeling

1. Define graph node IDs for `(function, block, node-index)` and synthetic call/return nodes.
2. Build call edges from `ast.CallExpr` using existing target-slot matchers in `cfa_summary.go`.
3. Model unresolved calls with conservative call-to-return edges tagged `unresolved-target`.
4. Encode recursion-cycle detection with explicit call-stack keys (replace ad hoc summary stack usage in solver path).

Done criteria:

- supergraph builder handles direct calls, method calls, method values, and closure calls currently covered by CFA fixtures.

### Workstream 2: IFDS fact propagation core

1. Implement finite fact set and lattice-neutral IFDS propagation.
2. Seed facts at cast sites, UBV cast definitions, and constructor return obligations.
3. Implement kill/gen transfer rules for Validate, use/escape, and return nodes.
4. Integrate block/state budgets with explicit inconclusive cause tagging.

Done criteria:

- deterministic propagation for identical input package order and analyzer flags.

### Workstream 3: IDE edge functions

1. Add value-environment transforms for validation, escape, and consumption states.
2. Compose edge functions across call and return edges.
3. Join edge environments conservatively; no optimistic downgrade on conflict.

Done criteria:

- edge-function composition unit tests cover recursion, unresolved callee, and mixed-path validation.

### Workstream 4: Check-family adapters

1. Cast adapter maps IFDS/IDE outcomes to `unvalidated-cast*` categories.
2. UBV adapter maps same-block/cross-block unsafe and inconclusive with existing `ubv_mode` metadata.
3. Constructor adapter maps return-path obligations to `missing-constructor-validate*` categories.
4. Keep current message text and finding-ID strategy stable unless explicitly versioned.

Done criteria:

- existing fixture expectations remain valid or are updated only where Phase-B correctness intentionally tightens outcomes.

### Workstream 5: Compatibility diff harness

1. Implement legacy-vs-ifds normalization and diff classifier.
2. Expose compare result in tests and script output.
3. Add hard fail on forbidden downgrade classes.

Done criteria:

- compare mode can be executed in CI and blocks merges on silent downgrade.

### Workstream 6: Contracts, docs, and gates

1. Extend semantic spec schema with interprocedural fields (engine, fact families, edge-function tags).
2. Add Phase B semantic spec document.
3. Add script + Make target for compatibility gate.
4. Update README soundness-doc index and Phase-B controls.

Done criteria:

- semantic spec tests and compatibility gate are both wired to CI command set.

## Oracle and Regression Matrix (Phase B Extensions)

Retain Phase A oracles and add Phase B-focused fixtures:

1. Deep helper chain (cast):
   - ensure no missed unsafe when validation occurs only in some callees.
2. Recursive helper cycle (UBV escape/order):
   - ensure recursion leads to unsafe/inconclusive, never optimistic safe.
3. Constructor transitive helper graph (cross-package):
   - ensure return-type validation obligations survive call depth.
4. Method-value and closure-heavy flows:
   - ensure call/return matching preserves current synchronous/asynchronous semantics.
5. Unresolved dynamic target scenarios:
   - ensure explicit inconclusive reason and witness metadata.

Required matrix invariants:

- each Phase-B category retains must-report and must-not-report coverage,
- recursion and high-depth fixtures are represented in `must_report`,
- historical-miss fixtures remain replayed by active tests.

## Acceptance Gates

`SolverParity`

- In compare mode, no case where legacy reports `unsafe` and IFDS reports `safe`.

`OracleMonotonicity`

- Curated must-report symbols for cast/UBV/constructor remain reported by IFDS path.

`NoSilentDowngrade`

- Recursion-depth and call-complexity fixtures do not silently downgrade to `safe`.

`OutcomeContract`

- Inconclusive outcomes preserve reason taxonomy and required metadata keys.

`RegistrySync`

- Category registry, semantic contract, and baseline policy remain aligned.

`PerformanceEnvelope`

- Existing CFG benchmark threshold gate stays green.
- New interprocedural overhead remains within declared budget envelope for representative suites.

## Rollout Sequence

1. Add engine flag and compare-mode plumbing (`legacy|ifds|compare`).
2. Land supergraph + IFDS core with cast family only behind `ifds`.
3. Add IDE edge functions and UBV family adapter.
4. Add constructor family adapter.
5. Enable compare mode in CI gate (`check-ifds-compat`) and keep it active post-flip.
6. Close parity/downgrade gaps until compare gate is stable.
7. Flip default to `ifds` after at least one full CI cycle with clean compare results (completed on 2026-03-05).
8. Keep `legacy` as temporary fallback and baseline compatibility mode until fallback retirement criteria are met.

## Risks and Mitigations

Risk: semantic drift between legacy and IFDS adapters.
- Mitigation: mandatory compare gate with downgrade-failure policy and per-category diff reports.

Risk: recursion/context blow-up degrades runtime.
- Mitigation: explicit budgets, memoization, SCC-aware cycle handling, and benchmark thresholds.

Risk: false confidence from parity-only checks.
- Mitigation: maintain must-report/must-not-report oracle matrix plus seeded stress fixtures.

Risk: metadata/governance regressions break baseline workflows.
- Mitigation: preserve category names, finding-ID derivation, and required inconclusive metadata contracts.

Risk: rollout regressions after default switch.
- Mitigation: keep `check-ifds-compat` and benchmark gates active, with `legacy` fallback and legacy-pinned baseline targets until fallback retirement.

## Commands and Verification Checklist (After Implementation)

Primary goplint checks:

```bash
cd tools/goplint && GOCACHE=/tmp/go-build go test ./goplint
cd tools/goplint && GOCACHE=/tmp/go-build go test -race ./goplint
./tools/goplint/scripts/check-semantic-spec.sh
./tools/goplint/scripts/check-cfg-bench-thresholds.sh
./tools/goplint/scripts/check-ifds-compat.sh
```

Repository-level regression gate:

```bash
make check-baseline
```
