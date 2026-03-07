# Phase C: Feasibility + CEGAR Refinement

Date: 2026-03-06  
Correctness posture: **Soundness-first**  
Roadmap source: [State-of-the-Art Soundness Roadmap](./state-of-the-art-soundness-roadmap.md)

## Objective

Implement Phase C by adding bounded witness-feasibility checking and counterexample-guided abstraction refinement for the same CFA-backed rule families carried by Phase B:

- `unvalidated-cast`
- `unvalidated-cast-inconclusive`
- `use-before-validate-same-block`
- `use-before-validate-cross-block`
- `use-before-validate-inconclusive`
- `missing-constructor-validate`
- `missing-constructor-validate-inconclusive`

Phase C must conservatively distinguish three cases for a candidate violating witness:

1. the witness is feasible and should remain `unsafe`;
2. the witness is infeasible and should trigger bounded refinement instead of becoming a permanent false alarm;
3. feasibility cannot be decided within supported abstractions, so the outcome stays conservatively `inconclusive`.

The main outcome is not a new rule family. It is a refinement layer over the Phase B IFDS/IDE path that measurably reduces persistent inconclusive findings, keeps false negatives from increasing, and emits explanation artifacts for every refined decision.

## Non-Goals (Phase D+)

Phase C does not implement:

- configurable alias-analysis tiers beyond the current Phase B target/slot matching and supergraph;
- relational numeric domains or broad arithmetic reasoning;
- a general-purpose whole-program symbolic executor;
- category renames, finding-ID redesign, or baseline format changes;
- retirement of the current legacy-pinned baseline workflow.

## Phase B Handoff and Preconditions

Phase C starts from the Phase B engine and contracts already in the repo:

1. Current interprocedural solver and compatibility model:
   - `tools/goplint/goplint/cfa_ifds_solver.go`
   - `tools/goplint/goplint/cfa_interproc_result.go`
   - `tools/goplint/goplint/cfa_interproc_compat.go`
2. Existing witness and inconclusive metadata:
   - `tools/goplint/goplint/cfa_outcome.go`
   - `tools/goplint/goplint/cfa_reporting.go`
   - call-chain metadata currently added by:
     - `tools/goplint/goplint/cfa_cast_validation.go`
     - `tools/goplint/goplint/cfa_closure.go`
     - `tools/goplint/goplint/analyzer_constructor_validates.go`
3. Existing run controls:
   - `--cfg-interproc-engine`
   - `--cfg-max-states`
   - `--cfg-max-depth`
   - `--cfg-inconclusive-policy`
   - `--cfg-witness-max-steps`
4. Existing semantic contract and oracle infrastructure:
   - `tools/goplint/spec/semantic-rules.v1.json`
   - `tools/goplint/spec/schema/semantic-rules.schema.json`
   - `tools/goplint/goplint/semantic_spec*.go`
5. Existing gates and current limitation:
   - `./tools/goplint/scripts/check-semantic-spec.sh`
   - `./tools/goplint/scripts/check-ifds-compat.sh`
   - `./tools/goplint/scripts/check-cfg-bench-thresholds.sh`
   - `make check-baseline`
   - `docs/goplint/current-techniques-and-semantics.md` currently documents that path feasibility is structural only and there is no SMT-backed discharge.

These Phase B contracts remain normative in Phase C. Refinement may add metadata and explanation traces, but category names, finding-ID derivation, and sound default behavior must remain stable.

Phase C may extend internal solver reason enums and trace payloads, but the externally visible `cfg_inconclusive_reason` taxonomy should remain backward compatible unless a later versioned contract explicitly broadens it.

## Current Gaps vs Phase C Deliverables

Phase C roadmap deliverables:

- SMT feasibility checker for witness paths.
- Refinement loop for recurring inconclusive classes.
- Diagnostics tagged with refinement outcome metadata.

Current gaps in `tools/goplint/goplint`:

1. Witnesses are encoded as bounded CFG block paths and call-chain hints, not as canonical path obligations that a feasibility engine can replay or discharge.
2. `interprocPathResult` tracks outcome, reason, fact family, and edge-function tag, but not feasibility verdicts, refinement status, or refinement history.
3. No bounded formula builder or backend abstraction exists for `SAT | UNSAT | UNKNOWN` feasibility decisions.
4. Current inconclusive reasons (`state-budget`, `depth-budget`, `recursion-cycle`, `unresolved-target`) are terminal reporting causes, not triggers for targeted re-analysis.
5. The internal findings stream (`-emit-findings-jsonl`) only records emitted findings; there is no explanation artifact path for discharged or refined-away witnesses.
6. CI gates validate semantic spec, IFDS compatibility, and benchmark thresholds, but no gate currently measures inconclusive reduction or requires refinement provenance.

## Implementation Touchpoint Map (Current Code)

Phase C must be anchored to these current seams:

1. Witness construction and outcome taxonomy:
   - `tools/goplint/goplint/cfa_outcome.go`
   - `tools/goplint/goplint/cfa_traversal.go`
   - `tools/goplint/goplint/cfa.go`
   - `tools/goplint/goplint/cfa_ubv.go`
   - `tools/goplint/goplint/analyzer_constructor_validates_cfa.go`
2. Phase B IFDS/IDE result surfaces:
   - `tools/goplint/goplint/cfa_ifds_solver.go`
   - `tools/goplint/goplint/cfa_interproc_result.go`
   - `tools/goplint/goplint/cfa_interproc_graph.go`
3. Category adapters and reporting:
   - `tools/goplint/goplint/cfa_cast_validation.go`
   - `tools/goplint/goplint/cfa_closure.go`
   - `tools/goplint/goplint/analyzer_constructor_validates.go`
   - `tools/goplint/goplint/cfa_reporting.go`
4. Run-control validation and analyzer orchestration:
   - `tools/goplint/goplint/flags.go`
   - `tools/goplint/goplint/analyzer_run.go`
5. Governance and machine-readable contracts:
   - `tools/goplint/goplint/categories.go`
   - `tools/goplint/goplint/semantic_spec.go`
   - `tools/goplint/spec/semantic-rules.v1.json`
   - `tools/goplint/spec/schema/semantic-rules.schema.json`
   - `tools/goplint/goplint/finding_sink.go`
6. Existing docs, scripts, and CI:
   - `tools/goplint/README.md`
   - `Makefile`
   - `.github/workflows/lint.yml`

## Target Architecture (Phase C)

### 1. Canonical witness explanation record

Normalize the current CFG-path and call-chain hints into a single explanation record that can be consumed by both the reporting layer and a feasibility backend.

Each record should carry:

- category + stable finding ID;
- origin anchors already used by current adapters;
- fact family, fact key, and edge-function tag from the Phase B solver;
- normalized CFG path and call chain;
- trigger reason (`state-budget`, `depth-budget`, `recursion-cycle`, `unresolved-target`, or `unsafe-candidate`);
- stable witness hash for de-duplication, caching, and trace comparison.

This record becomes the contract between the solver, the refinement loop, and the explanation artifact path.

### 2. Feasibility discharge layer

Add a bounded feasibility abstraction with a backend interface that returns:

- `sat` for a feasible violating witness,
- `unsat` for a disproven witness,
- `unknown` when the backend cannot soundly decide.

The first supported predicate vocabulary should stay intentionally narrow:

- branch polarity already recoverable from CFG successor choices;
- simple nil/equality-to-constant/type-test guards when they are directly extractable;
- call/return reachability facts already represented in the Phase B interprocedural graph.

Anything outside this vocabulary must degrade to `unknown`, not `safe`.

### 3. CEGAR refinement loop

Use infeasible or underconstrained witnesses as targeted refinement triggers instead of final answers.

The loop is:

1. Phase B emits a candidate `unsafe` or `inconclusive` witness.
2. Feasibility checks that witness.
3. If the witness is `unsat`, classify the abstraction gap and choose a bounded refinement action.
4. Re-run only the affected analysis slice.
5. Stop when a feasible violating witness is found, the result becomes conservatively `inconclusive`, or the bounded refinement budget is exhausted.

Initial refinement actions should be keyed to the current reason taxonomy:

- `state-budget`: rerun with a localized state budget increase for the exact witness slice;
- `depth-budget`: extend only the witness-local search depth, not the full package traversal;
- `recursion-cycle`: refine call-string or summary precision for the current call chain only;
- `unresolved-target`: retry call-target resolution with narrowed receiver/argument context and explicit fallback if still unresolved.

### 4. Provenance and emission semantics

Phase C must preserve existing category names and finding IDs while adding refinement provenance.

For emitted findings, add metadata such as:

- `cfg_feasibility_engine`
- `cfg_feasibility_result`
- `cfg_refinement_status`
- `cfg_refinement_iterations`
- `cfg_refinement_trigger`
- `cfg_refinement_witness_hash`

For decisions that become safe only because all candidate violating witnesses were discharged within budget, do not emit a new user-facing diagnostic. Instead, write explanation artifacts through the existing internal findings-stream plumbing or a closely related trace sink so tests and gates can still audit what changed.

### 5. Governance and measurement layer

Extend the semantic contract and CI gates so Phase C is measurable and explainable:

- semantic spec declares new run controls and required refinement metadata;
- curated oracle fixtures distinguish real violations from infeasible/spurious witnesses;
- a dedicated refinement gate reports inconclusive-reduction metrics while preserving must-report behavior;
- benchmark and compatibility gates remain active so Phase C never bypasses Phase A/B protections.

## Proposed Artifact Plan

Planned new files:

1. Feasibility and refinement core:
   - `tools/goplint/goplint/cfa_feasibility.go`
   - `tools/goplint/goplint/cfa_feasibility_smt.go`
   - `tools/goplint/goplint/cfa_refinement.go`
   - `tools/goplint/goplint/cfa_refinement_cache.go`
   - `tools/goplint/goplint/cfa_refinement_trace.go`
2. Optional refinement diff helper:
   - `tools/goplint/goplint/cfa_refinement_compat.go`
3. Tests for Phase C:
   - `tools/goplint/goplint/cfa_feasibility_test.go`
   - `tools/goplint/goplint/cfa_refinement_test.go`
   - `tools/goplint/goplint/cfa_refinement_trace_test.go`
   - `tools/goplint/goplint/cfa_refinement_compat_test.go`
4. Scripts and docs:
   - `tools/goplint/scripts/check-cfg-refinement.sh`
   - `docs/goplint/semantic-rule-spec-phase-c.md`

Planned modified files:

1. Control-plane and validation:
   - `tools/goplint/goplint/flags.go`
   - `tools/goplint/goplint/analyzer_run.go`
2. Solver/result integration:
   - `tools/goplint/goplint/cfa_ifds_solver.go`
   - `tools/goplint/goplint/cfa_interproc_result.go`
   - `tools/goplint/goplint/cfa_outcome.go`
3. Check adapters and reporting:
   - `tools/goplint/goplint/cfa_cast_validation.go`
   - `tools/goplint/goplint/cfa_closure.go`
   - `tools/goplint/goplint/analyzer_constructor_validates.go`
   - `tools/goplint/goplint/cfa_reporting.go`
   - `tools/goplint/goplint/finding_sink.go`
4. Semantic contracts and registry tests:
   - `tools/goplint/spec/semantic-rules.v1.json`
   - `tools/goplint/spec/schema/semantic-rules.schema.json`
   - `tools/goplint/goplint/semantic_spec.go`
   - relevant `semantic_spec*_test.go`
5. Repo gates/docs:
   - `Makefile`
   - `tools/goplint/README.md`
   - `.github/workflows/lint.yml`

## Control-Plane Design (Feasibility + Refinement)

Introduce Phase C run controls behind explicit opt-in flags first:

- `--cfg-feasibility-engine=off|smt`
- `--cfg-refinement-mode=off|once|cegar`
- `--cfg-refinement-max-iterations=<n>`
- `--cfg-feasibility-max-queries=<n>`
- `--cfg-feasibility-timeout-ms=<n>`

Phase C rollout behavior:

1. `off + off`
   - exact Phase B behavior, no feasibility discharge, no refinement.
2. `smt + once`
   - one bounded feasibility/refinement pass for targeted oracle and fixture runs.
3. `smt + cegar`
   - bounded iterative refinement for full Phase C gates after oracle confidence is proven.

Normalization contract:

- `unknown` or timeout can never suppress a finding.
- `unsat` disproves only the current witness, not the entire obligation.
- a result may become safe only when all candidate violating witnesses have been discharged within the allowed abstraction and budgets;
- every suppressed/refined-away candidate must still leave an explanation artifact.

Baseline contract during rollout:

- keep `make check-baseline` and `make update-baseline` pinned to the current legacy workflow while Phase C matures;
- Phase C gates operate against the IFDS path and curated oracle suites first, not baseline drift.

## Detailed Work Breakdown

### Workstream 1: Canonical witness and trace substrate

Goal:
- Turn current CFG witness fragments into a stable refinement/explanation contract.

Implementation plan:

1. Introduce a normalized witness record carrying:
   - finding identity;
   - normalized CFG path;
   - call chain;
   - fact family/fact key;
   - edge-function tag;
   - trigger reason.
2. Extend internal result/trace payloads with refinement-specific backend reason details (for example timeout, unsupported predicate, discharged contradiction) while keeping the public inconclusive reason contract stable.
3. Compute a stable witness hash from normalized fields so repeated runs de-duplicate the same refinement target.
4. Keep current URL metadata keys (`witness_cfg_path`, `cfg_witness_*`) as backward-compatible public output while the new record is used internally.
5. Extend the internal findings stream or adjacent trace sink to persist discharged/refined decisions for tests and gates.

Done criteria:

- repeated runs with identical inputs produce byte-stable witness hashes and trace ordering;
- existing inconclusive metadata keys remain available on emitted diagnostics.

### Workstream 2: Feasibility backend and bounded formula builder

Goal:
- Add a conservative feasibility checker that can prove a witness infeasible without inventing unsupported facts.

Implementation plan:

1. Define a feasibility backend interface returning:
   - result (`sat`, `unsat`, `unknown`);
   - backend reason (`unsupported-predicate`, `timeout`, `solver-error`, or empty);
   - optional explanation payload.
2. Build a bounded path-formula encoder from the canonical witness record using only supported guard vocabulary.
3. Add an SMT-backed backend behind `--cfg-feasibility-engine=smt`.
4. Treat unsupported guards and backend failures as `unknown`, never as proof of safety.

Done criteria:

- unit tests cover feasible, infeasible, timeout, and unsupported cases;
- backend failures cannot cause a category to disappear.

### Workstream 3: Bounded CEGAR refinement loop

Goal:
- Convert infeasible or underconstrained witnesses into targeted re-analysis instead of permanent uncertainty.

Implementation plan:

1. Add a refinement planner keyed by witness hash, outcome class, and trigger reason.
2. Invoke refinement at the last safe interception points before the current solver paths would return final `unsafe` or `inconclusive` results:
   - `dfsUnvalidatedBlocksOutcomeWithWitness`
   - `dfsUseBeforeValidateModeWithSummaryStackWithWitness`
   - `constructorReturnPathOutcomeWithWitness`
   - IFDS terminal synthesis in `runIFDSPropagation`
3. Map current reasons to targeted actions:
   - `state-budget` -> localized budget increase;
   - `depth-budget` -> localized depth extension;
   - `recursion-cycle` -> more precise call-context for the current chain;
   - `unresolved-target` -> narrowed target-resolution retry.
4. Cache refinement attempts per witness hash so repeated failures do not explode query volume.
5. Bound the loop by both iteration count and feasibility-query count.
6. Return:
   - `unsafe` if a feasible violating witness remains;
   - `inconclusive-refined` if refinement ran but could not prove safety;
   - `inconclusive-unrefined` if no refinement applied or budgets prevented it;
   - internal `proven-safe` only when all candidate violating witnesses are discharged within budget.

Done criteria:

- refinement terminates deterministically under fixed flags;
- `unknown` feasibility results never become `safe`;
- localized refinement measurably reduces inconclusives on a curated corpus.

### Workstream 4: Adapter integration and reporting semantics

Goal:
- Thread Phase C through cast, UBV, and constructor adapters without destabilizing current public contracts.

Implementation plan:

1. Integrate refinement decisions into:
   - `cfa_cast_validation.go`
   - `cfa_closure.go`
   - `analyzer_constructor_validates.go`
2. Preserve current category names, message text, and finding-ID components unless a future versioned contract explicitly changes them.
3. Add refinement metadata to emitted findings and explanation artifacts to discharged decisions.
4. Reuse the compare-mode discipline from Phase B to add a refinement diff view between unrefined IFDS and refined IFDS outcomes for tests/gates.

Done criteria:

- must-report fixtures still report under refined mode unless a fixture is explicitly modeled as infeasible;
- refined findings include the required metadata keys on every path.

### Workstream 5: Semantic contracts, oracle corpus, and gates

Goal:
- Make Phase C behavior machine-checkable and CI-enforced.

Implementation plan:

1. Extend semantic spec/schema with Phase C fields, for example:
   - `refinement_statuses`
   - `required_meta_on_refinement`
   - new `run_controls` entries for Phase C flags
2. Add `docs/goplint/semantic-rule-spec-phase-c.md` documenting the normative contract.
3. Add targeted fixtures for:
   - contradictory branch guards producing infeasible witnesses;
   - unresolved-target cases that stay inconclusive after narrowed resolution;
   - recursion-cycle cases improved by bounded context refinement;
   - real violations that must remain `unsafe`;
   - unsupported predicates that must stay non-safe.
4. Add `check-cfg-refinement.sh` and wire it into `Makefile` and `.github/workflows/lint.yml`.

Done criteria:

- semantic spec tests fail if Phase C metadata or controls drift from implementation;
- refinement gate fails on must-report loss, missing explanation metadata, or forbidden optimistic suppression.

## Oracle and Regression Matrix (Phase C Extensions)

Retain Phase A/B oracles and add Phase C-focused fixtures:

1. Infeasible witness fixture:
   - contradictory guard path should be discharged as `unsat`;
   - result should either become `proven-safe` internally or remain `inconclusive`, but never silently disappear without trace.
2. Real-violation fixture:
   - refinement must still end at `unsafe`.
3. Persistent-unknown fixture:
   - unsupported predicates or backend timeouts stay `inconclusive` with refinement metadata.
4. Localized-budget fixture:
   - a witness initially hitting `state-budget` or `depth-budget` becomes conclusive after bounded localized rerun.
5. Recursion/refined-context fixture:
   - call-context refinement improves precision without broad runtime explosion.

Required matrix invariants:

- Phase B must-report symbols for cast/UBV/constructor remain reported when the violation is real;
- no `unknown` or timeout path is downgraded to `safe`;
- every refined emitted finding carries refinement metadata;
- every discharged witness has a trace artifact visible to tests/gates;
- repeated runs under identical flags produce deterministic results.

## Acceptance Gates

`RefinementSoundness`

- Curated real-violation fixtures remain reported under refined mode.

`InconclusiveReduction`

- The curated persistent-inconclusive corpus shrinks relative to unrefined IFDS mode.

`NoOptimisticUnknown`

- `unknown`, timeout, and unsupported predicates never classify as `safe`.

`ExplanationCompleteness`

- Every refined decision has either emitted metadata or a trace artifact explaining why the result changed.

`RegistrySync`

- Category registry, semantic contract, and required refinement metadata remain aligned.

`Determinism`

- Same package set plus same flags yields stable witness hashes and stable refined outcomes.

`PerformanceEnvelope`

- Existing CFG benchmark threshold gate stays green.
- Phase C overhead stays within a declared query/time budget envelope on representative suites.

## Rollout Sequence

1. Add Phase C flags and canonical witness trace plumbing with defaults disabled.
2. Land explanation artifacts and witness hashing with no behavior change.
3. Add the feasibility backend behind `--cfg-feasibility-engine=smt`.
4. Enable one-pass refinement on targeted fixtures and semantic-oracle tests.
5. Add bounded `cegar` mode for reason-specific localized refinements.
6. Wire `check-cfg-refinement` into CI while keeping IFDS compatibility and benchmark gates active.
7. After at least one stable CI window, consider enabling Phase C by default on the IFDS path.
8. Keep the legacy-pinned baseline workflow unchanged until Phase C defaulting is proven stable and separately approved.

## Dependency-Ordered Task List

This section turns the Phase C plan into an execution sequence with explicit dependencies. Task IDs are ordered by critical-path dependency, not by desirability alone.

### C0: Freeze the Phase C contract surface

Depends on:
- none

Deliverables:
- confirm the initial flag surface for feasibility/refinement;
- confirm that public category names and finding IDs remain unchanged;
- add a short implementation note clarifying that new refinement details are additive metadata only.

Exit criteria:
- there is a single agreed Phase C flag/control set to implement first;
- no task below relies on an ambiguous public contract.

### C1: Add canonical witness record and stable witness hashing

Depends on:
- C0

Deliverables:
- normalized witness/trace record type;
- stable witness hash algorithm;
- deterministic serialization/ordering tests.

Exit criteria:
- repeated runs over the same fixture produce identical witness hashes;
- the witness record can represent current CFG path + call-chain metadata without loss.

### C2: Extend internal trace/output plumbing for refined decisions

Depends on:
- C1

Deliverables:
- trace sink or findings-stream extension for discharged/refined-away witnesses;
- stable record shape for non-emitted explanation artifacts;
- tests covering sink determinism and backward compatibility.

Exit criteria:
- refined-away decisions are auditable even when no user-facing diagnostic is emitted;
- current emitted finding behavior remains unchanged when Phase C is disabled.

### C3: Add Phase C run controls and analyzer validation

Depends on:
- C0

Deliverables:
- new flags in `flags.go`;
- validation/defaulting in `analyzer_run.go`;
- integration of new controls into run config plumbing with defaults disabled.

Exit criteria:
- invalid Phase C flag combinations fail fast;
- default execution still behaves exactly like Phase B.

### C4: Build feasibility backend abstraction and narrow formula encoder

Depends on:
- C1
- C3

Deliverables:
- feasibility backend interface;
- bounded formula/path encoder over supported predicates only;
- unit tests for `sat`, `unsat`, `unknown`, timeout, and unsupported-predicate cases.

Exit criteria:
- the encoder never invents unsupported facts;
- backend failures and unsupported predicates always degrade to `unknown`.

### C5: Implement initial SMT feasibility backend

Depends on:
- C4

Deliverables:
- `smt` feasibility backend implementation;
- backend timeout/query-budget controls;
- deterministic fixture tests for feasible and infeasible witnesses.

Exit criteria:
- `--cfg-feasibility-engine=smt` works on curated fixtures;
- timeout and backend failure behavior is fail-closed.

### C6: Insert refinement interception points into solver/traversal exits

Depends on:
- C1
- C3
- C4

Deliverables:
- refinement hook points at:
  - `dfsUnvalidatedBlocksOutcomeWithWitness`
  - `dfsUseBeforeValidateModeWithSummaryStackWithWitness`
  - `constructorReturnPathOutcomeWithWitness`
  - IFDS terminal synthesis in `runIFDSPropagation`
- internal result carriers extended with refinement state.

Exit criteria:
- current Phase B outcomes are unchanged when refinement mode is `off`;
- candidate `unsafe` and `inconclusive` witnesses are available to the refinement layer before final emission.

### C7: Implement bounded one-pass refinement actions by reason class

Depends on:
- C5
- C6

Deliverables:
- localized rerun strategy for `state-budget`, `depth-budget`, `recursion-cycle`, and `unresolved-target`;
- refinement attempt cache keyed by witness hash;
- one-pass refinement mode (`once`) with deterministic termination.

Exit criteria:
- one-pass refinement can improve curated inconclusive cases without broad package-wide budget expansion;
- repeated identical witnesses do not trigger unbounded retries.

### C8: Implement iterative `cegar` mode and termination guards

Depends on:
- C7

Deliverables:
- bounded iterative refinement loop;
- iteration/query-budget enforcement;
- stable status classification (`unsafe`, `inconclusive-refined`, `inconclusive-unrefined`, internal `proven-safe`).

Exit criteria:
- iterative mode terminates deterministically under fixed flags;
- no `unknown` or timeout result can become `safe`.

### C9: Integrate adapter/reporting metadata for cast, UBV, and constructor families

Depends on:
- C2
- C6
- C7

Deliverables:
- adapter wiring in cast, closure/UBV, and constructor reporters;
- additive metadata keys for feasibility/refinement;
- diff/reporting tests proving existing category/message/finding-ID continuity.

Exit criteria:
- refined findings expose the required metadata;
- real must-report violations still emit under refined mode.

### C10: Extend semantic contracts, docs, and oracle fixtures

Depends on:
- C3
- C7
- C9

Deliverables:
- schema/catalog updates for Phase C fields;
- `docs/goplint/semantic-rule-spec-phase-c.md`;
- Phase C oracle fixtures for infeasible, real-violation, persistent-unknown, localized-budget, and recursion-refined cases.

Exit criteria:
- semantic spec tests cover the new metadata and controls;
- the doc/spec corpus matches the implemented Phase C contract.

### C11: Add dedicated refinement gate and CI wiring

Depends on:
- C8
- C9
- C10

Deliverables:
- `tools/goplint/scripts/check-cfg-refinement.sh`;
- `Makefile` target;
- `.github/workflows/lint.yml` job wiring;
- benchmark and no-optimistic-suppression assertions for refined mode.

Exit criteria:
- CI can fail on must-report loss, missing refinement provenance, optimistic `unknown -> safe`, or unacceptable overhead;
- existing Phase A/B gates remain active alongside the new gate.

### C12: Run rollout-readiness audit before any default flip

Depends on:
- C11

Deliverables:
- evidence summary for inconclusive reduction, no-silent-downgrade behavior, and runtime overhead;
- explicit recommendation on whether refined mode should stay opt-in or become default on the IFDS path.

Exit criteria:
- at least one stable CI window completes with the new refinement gate enabled;
- default enablement is treated as a separate approval decision, not implied by implementation completion.

## Risks and Mitigations

Risk: unsound suppression if unsupported predicates are treated as proof of safety.  
Mitigation: `unknown` is fail-closed and can never produce `safe`.

Risk: query explosion or runtime blow-up on large packages.  
Mitigation: witness hashing, localized refinement only, bounded iterations, bounded query counts, and explicit timeout controls.

Risk: solver/backend instability creates nondeterministic CI behavior.  
Mitigation: stable witness normalization, pinned backend configuration, deterministic trace ordering, and timeout-to-unknown fallback.

Risk: metadata churn breaks baseline and machine consumers.  
Mitigation: preserve category names and finding IDs, add metadata only, and keep new keys explicitly versioned in semantic spec.

Risk: refined-away findings become un-auditable.  
Mitigation: write explanation artifacts through the internal findings stream or a dedicated companion trace sink and make gates consume them.

Risk: Phase C quietly bypasses Phase B protections.  
Mitigation: keep `check-ifds-compat` active and add a refined-vs-unrefined diff gate for must-report fixtures.

## Commands and Verification Checklist (After Implementation)

Primary goplint checks:

```bash
cd tools/goplint && GOCACHE=/tmp/go-build go test ./goplint
cd tools/goplint && GOCACHE=/tmp/go-build go test -race ./goplint
./tools/goplint/scripts/check-semantic-spec.sh
./tools/goplint/scripts/check-ifds-compat.sh
./tools/goplint/scripts/check-cfg-refinement.sh
./tools/goplint/scripts/check-cfg-bench-thresholds.sh
```

Repository-level regression gate:

```bash
make check-baseline
```
