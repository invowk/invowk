## MODIFIED Requirements

### Requirement: Goplint exception governance is enforced
Invowk SHALL keep `tools/goplint` baseline and exception governance aligned with its lint, type-system, canonical semantic-analysis, routed soundness-assurance profile, and exhaustive completion quality gates.

#### Scenario: Goplint lint and the required routed profile run together
- **WHEN** repository lint gates run
- **THEN** `tools/goplint` golangci-lint checks MUST run in addition to the custom goplint analyzer gates
- **THEN** automation MUST select the conservatively classified goplint assurance profile without a legacy, alternate, fallback, or weakened semantic path
- **THEN** explicit semantic, completion, release, and scheduled gates MUST retain every population required by their canonical manifests
- **THEN** neither custom analyzer nor soundness gates MUST be treated as a replacement for the module's golangci-lint config

#### Scenario: Accepted goplint exceptions are reviewable
- **WHEN** a goplint exception is kept in `tools/goplint/exceptions.toml`
- **THEN** it MUST include a reason that explains why the exception remains acceptable
- **THEN** long-lived or broad exceptions MUST include a review date or equivalent review mechanism

#### Scenario: Stale goplint exception audit is part of quality gates
- **WHEN** repository quality gates run locally or in CI
- **THEN** stale, overdue, malformed, or unsupported goplint exceptions MUST be reported
- **THEN** stale matching MUST consume the exact canonical repository-audit result when package analysis has already run
- **THEN** review-date validation MUST NOT repeat package loading or analyzer traversal
- **THEN** the gate MUST fail for stale or malformed exceptions unless the design explicitly marks a temporary advisory transition inside this same change

#### Scenario: Baseline uses the only production semantics
- **WHEN** `make check-baseline` or `make update-baseline` invokes goplint
- **THEN** it MUST use the same canonical production analysis and fail-closed aggregation as the blocking full repository scan
- **THEN** a read-only baseline check MUST reuse an exact-tree canonical repository-audit result when one exists in the same execution plan
- **THEN** it MUST NOT retain a legacy fact reader, AST fallback, alternate evaluator, hidden selector, or mode-specific stable-ID path
- **THEN** stable finding ID changes MUST be reported and reviewed before the baseline is accepted

#### Scenario: Goplint baseline wording matches behavior
- **WHEN** baseline tooling, goplint documentation, or agent guidance describes baseline behavior
- **THEN** it MUST distinguish baseline-suppressed categories from always-visible hard-blocking categories
- **THEN** stale statements about accepted counts, alternate semantics, or advisory soundness scans MUST be removed

#### Scenario: Canonical full scan is blocking
- **WHEN** the repository goplint full scan runs locally, in pre-commit, or in CI
- **THEN** violations, blocking inconclusive outcomes, malformed evidence, incomplete required evidence for the selected profile, surviving or non-causal required mutants, legacy-path detections, and analyzer failures MUST fail the gate
- **THEN** the workflow MUST NOT downgrade or mask those outcomes

## ADDED Requirements

### Requirement: Automatic goplint assurance routing is conservative and change aware
Repository automation SHALL select the least expensive goplint profile that completely covers the changed ownership surface, and SHALL fail closed to a stronger profile whenever it cannot prove that a weaker profile is sufficient.

#### Scenario: Consumer code changes without analyzer ownership changes
- **WHEN** a pull request changes only root-module code that consumes goplint and no analyzer, gate, evidence, governance, workflow, threshold, or governing-specification path
- **THEN** automation MUST run one blocking canonical repository audit with baseline and exception governance
- **THEN** it MUST NOT rerun unchanged analyzer soundness subgates solely because `cmd`, `internal`, or `pkg` changed

#### Scenario: Analyzer or assurance ownership changes
- **WHEN** a change touches goplint production semantics, tests, evidence producers, manifests, schemas, scripts, threshold manifests, workflow orchestration, or governing goplint specifications
- **THEN** automation MUST select the semantic profile
- **THEN** the profile MUST retain every causal core population required before this optimization

#### Scenario: Completion event requires exhaustive evidence
- **WHEN** a completion proof, release, scheduled certification, or explicit exhaustive dispatch runs
- **THEN** automation MUST select the completion profile regardless of changed paths
- **THEN** clean-tree freshness and statistically stable performance certification MUST be blocking

#### Scenario: Change context is missing or ambiguous
- **WHEN** the merge base, changed-path census, ownership manifest, or event context is missing, malformed, stale, or ambiguous
- **THEN** routing MUST select the applicable semantic or completion profile
- **THEN** it MUST NOT silently select the consumer profile

### Requirement: One exact-tree repository analysis serves all read-only audit consumers
The goplint quality gate SHALL execute at most one canonical package/analyzer traversal per exact execution plan for full-scan enforcement, baseline comparison, and stale-exception matching.

#### Scenario: Multiple audit verdicts are required
- **WHEN** full-scan, baseline, and stale-exception verdicts are required for the same tree
- **THEN** one canonical superset analysis MUST emit a machine-readable repository-audit result
- **THEN** each verdict MUST be derived from that result without loading or analyzing the packages again

#### Scenario: Audit input identity differs
- **WHEN** the workspace, analyzer binary, toolchain, package census, flags, baseline, exception configuration, or semantic manifest differs from the repository-audit binding
- **THEN** the consumer MUST reject the result
- **THEN** a fresh canonical analysis MUST run before any verdict can pass

#### Scenario: Review dates are audited
- **WHEN** exception review dates are checked
- **THEN** the audit MUST parse and validate configuration without loading Go packages
- **THEN** malformed or overdue entries MUST remain blocking

### Requirement: Local goplint execution is resource aware and bounded
The aggregate goplint executor SHALL discover the effective local CPU and available-memory budgets, schedule independent work concurrently within explicit resource reservations, and provide deterministic overrides and conservative fallbacks.

#### Scenario: High-capacity local machine runs the semantic profile
- **WHEN** auto mode runs on a machine exposing 24 effective CPUs and sufficient available memory
- **THEN** the plan MUST admit materially more concurrent independent work than the 4-vCPU hosted-runner plan
- **THEN** it MUST reserve at least 75 percent of the effective CPU budget while sufficient runnable work exists
- **THEN** child-process parallelism MUST remain bounded by the resources reserved to each work unit

#### Scenario: Memory budget would be exceeded
- **WHEN** a ready set of subgates would exceed the detected or configured available-memory budget
- **THEN** the executor MUST defer work until the required memory tokens are available
- **THEN** it MUST NOT rely on swap exhaustion or an operating-system kill as ordinary flow control

#### Scenario: Resource discovery is unavailable
- **WHEN** effective CPU or available memory cannot be determined reliably
- **THEN** the executor MUST use a documented conservative fallback
- **THEN** the normalized execution result MUST remain semantically identical to a larger-budget run

#### Scenario: Contributor overrides resources
- **WHEN** a contributor supplies valid CPU, memory, or worker limits
- **THEN** the planner MUST record and enforce those limits deterministically
- **THEN** invalid or impossible limits MUST fail with an actionable error before any subgate runs

### Requirement: Race and repeat execution uses exhaustive balanced work units
The goplint race/repeat gate SHALL assign the live top-level test census to deterministic duration-weighted work units, compile each required test-binary mode once per plan, and prove that every required test execution occurred exactly once per configured iteration.

#### Scenario: Shards are planned from timing metadata
- **WHEN** the analyzer test census is divided into shards
- **THEN** the planner MUST validate every live `Test`, `Fuzz`, and `Example` against versioned timing metadata
- **THEN** it MUST assign every top-level entry exactly once using a deterministic longest-processing-time policy
- **THEN** an entry without timing data MUST receive a conservative weight and remain visible for metadata refresh

#### Scenario: Heavy nested family dominates one top-level test
- **WHEN** one top-level entry contains independently executable cases whose weight prevents balanced shards
- **THEN** the implementation MUST expose a stable machine-validated case census for sharding or split the cases into stable top-level entries
- **THEN** no case MAY be skipped, duplicated, or accepted through a name marker alone

#### Scenario: Race and repeat binaries are prepared
- **WHEN** a race/repeat plan executes
- **THEN** the normal analyzer test binary MUST be compiled at most once for that plan
- **THEN** the race-instrumented analyzer test binary MUST be compiled at most once for that plan
- **THEN** every shard MUST bind to the expected binary digest, test census, mode, and iteration

#### Scenario: A shard times out or is absent
- **WHEN** a required shard times out, crashes, reports the wrong census, or produces no bound result
- **THEN** race/repeat aggregation MUST fail
- **THEN** increasing a workflow-level timeout MUST NOT make the missing population pass

### Requirement: Goplint gate performance is observable and regression bounded
Every goplint execution plan SHALL retain enough machine-readable timing and resource evidence to explain its critical path, prove utilization, detect shard imbalance, and enforce reviewed wall-time objectives without changing semantic verdicts.

#### Scenario: Work unit completes
- **WHEN** a local or distributed work unit completes
- **THEN** its result MUST record queue time, wall time, reserved resources, peak memory when available, exit or timeout cause, and population counts
- **THEN** the aggregate MUST report critical path, maximum concurrent reservations, and shard imbalance in deterministic normalized form

#### Scenario: Optimized executor is compared with the serial reference
- **WHEN** the optimized executor becomes authoritative
- **THEN** normalized findings, observations, required populations, and verdicts MUST match the serial reference byte-for-byte after permitted timing fields are removed
- **THEN** semantic-profile median wall time on the same reviewed runner class MUST be at least 50 percent lower than the recorded serial baseline

#### Scenario: Consumer profile performance is accepted
- **WHEN** the consumer profile is measured on the reviewed warm-cache 4-vCPU CI runner class
- **THEN** its blocking goplint feedback objective MUST be no more than 10 minutes
- **THEN** failure to meet the objective MUST remain visible in telemetry and block removal of the legacy topology during migration
