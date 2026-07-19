# lint-tooling-quality-gates Specification

## Purpose
Define the repository-wide lint tooling, module coverage, and blocking quality-gate contracts used by local development, pre-commit, and CI, including goplint baseline, exception, semantic, and soundness assurance.
## Requirements
### Requirement: Golangci-lint version is normalized
Invowk SHALL resolve golangci-lint through one exact repository-governed version contract used by local Make targets, pre-commit hooks, and GitHub Actions.

#### Scenario: Latest release is checked before changing the pin
- **WHEN** maintainers implement or update the golangci-lint pin
- **THEN** they MUST check the current upstream golangci-lint release
- **THEN** the selected version MUST be an exact version
- **THEN** the implementation notes or updated version-pinning documentation MUST record the selected version and where it is enforced

#### Scenario: Local lint does not use an unverified ambient binary
- **WHEN** `make lint` or any lint subtarget runs locally
- **THEN** it MUST invoke the repository-normalized golangci-lint binary or fail before linting with an actionable version mismatch message
- **THEN** it MUST NOT silently use an arbitrary `golangci-lint` binary from `PATH`

#### Scenario: CI and pre-commit use the same golangci-lint version
- **WHEN** GitHub Actions or pre-commit runs golangci-lint
- **THEN** the resolved golangci-lint version MUST match the repository-normalized version
- **THEN** a version mismatch MUST fail before lint results are trusted

#### Scenario: Version documentation remains synchronized
- **WHEN** the golangci-lint version changes
- **THEN** `AGENTS.md`, `.agents/rules/version-pinning.md`, workflow configuration, pre-commit configuration, Make targets, and any wrapper or tool-pin files that mention golangci-lint MUST describe the same version source and current version

### Requirement: Lint automation covers every Go module
Invowk SHALL lint the root Go module and the nested `tools/goplint` Go module anywhere the repository advertises full lint coverage.

#### Scenario: Make lint covers both modules
- **WHEN** maintainers run `make lint`
- **THEN** linting MUST run against the root module with the root golangci-lint config
- **THEN** linting MUST run against `tools/goplint` from the `tools/goplint` module root with that module's golangci-lint config

#### Scenario: CI lint coverage matches Make lint coverage
- **WHEN** the lint workflow runs in GitHub Actions
- **THEN** it MUST lint the root module and `tools/goplint`
- **THEN** it MUST fail if either module's golangci-lint config fails validation or produces lint findings
- **THEN** workflow comments and job names MUST NOT imply coverage that is not actually executed

#### Scenario: Pre-commit lint coverage matches changed Go module surfaces
- **WHEN** pre-commit runs golangci-lint hooks
- **THEN** it MUST run the root-module lint gate when root-module Go files or shared lint configuration changes are staged
- **THEN** it MUST run the `tools/goplint` lint gate when `tools/goplint` Go files or its lint configuration changes are staged
- **THEN** it MUST provide a supported path to run both module lint gates together

#### Scenario: Nested module boundaries are explicit
- **WHEN** maintainers inspect lint automation
- **THEN** the automation MUST make clear that root `go list ./...` does not include `tools/goplint`
- **THEN** the nested-module lint invocation MUST NOT depend on accidental traversal from the root module

### Requirement: Golangci-lint formatter policy is enforced
Invowk SHALL enforce every configured golangci-lint v2 formatter policy instead of leaving formatter sections as documentation-only configuration.

#### Scenario: Formatter check runs for root module
- **WHEN** the repository formatter check runs
- **THEN** it MUST run golangci-lint formatting in diff or check mode for the root module using `.golangci.toml`
- **THEN** it MUST fail when formatting changes would be produced

#### Scenario: Formatter check runs for tools/goplint module
- **WHEN** the repository formatter check runs
- **THEN** it MUST run golangci-lint formatting in diff or check mode for `tools/goplint` using `tools/goplint/.golangci.toml`
- **THEN** it MUST fail when formatting changes would be produced

#### Scenario: Formatter automation is part of the regular quality gate
- **WHEN** `make lint`, CI linting, or pre-commit quality gates run
- **THEN** formatter checks MUST run as part of the same quality gate or through an explicitly documented companion target/hook required for completion
- **THEN** generated-file exclusions MUST match the formatter config rather than separate ad hoc skip logic

### Requirement: Linter configs are deterministic and non-contradictory
Invowk SHALL keep golangci-lint configuration deterministic, internally consistent, and truthful about what each linter and exclusion enforces.

#### Scenario: Enabled linters are explicit
- **WHEN** maintainers inspect root or `tools/goplint` golangci-lint config
- **THEN** each blocking linter MUST be explicitly listed
- **THEN** the config MUST NOT rely on an implicit default set whose contents can change without visible config changes unless the design documents that trade-off and all tooling verifies the effective linter set

#### Scenario: Config verification passes
- **WHEN** maintainers run the lint validation target
- **THEN** golangci-lint config verification MUST pass for the root config
- **THEN** golangci-lint config verification MUST pass for `tools/goplint/.golangci.toml`

#### Scenario: Effective linter sets are inspectable
- **WHEN** maintainers audit lint coverage
- **THEN** they MUST be able to inspect the effective enabled linter set for each module
- **THEN** differences between the root and `tools/goplint` linter sets MUST be intentional, documented in config comments or design notes, and based on module-specific behavior rather than drift

#### Scenario: Comments match enforcement scope
- **WHEN** a linter setting, exclusion, or workflow comment explains a policy
- **THEN** the described scope MUST match the actual golangci-lint scope
- **THEN** a test-only or fixture-only rationale MUST NOT be attached to a global production-code exclusion

### Requirement: Lint exclusions and nolints are narrow and auditable
Invowk SHALL prefer fixing code over broad suppression and SHALL keep every remaining lint suppression scoped, specific, and reviewable.

#### Scenario: Global exclusions are not test-only suppressions
- **WHEN** golangci-lint excludes a function, linter, or path globally
- **THEN** the exclusion rationale MUST apply to production code as well as tests
- **THEN** test-only or fixture-only exclusions MUST be moved to path-scoped exclusion rules or removed

#### Scenario: Nolints identify linter and rationale
- **WHEN** source code uses `//nolint`
- **THEN** the directive MUST name the specific linter or linters being suppressed
- **THEN** the directive MUST include or be adjacent to a human-readable rationale unless the linter config explicitly documents a narrower acceptable pattern

#### Scenario: Stale suppressions fail lint
- **WHEN** a `//nolint` directive no longer suppresses an active finding
- **THEN** golangci-lint or a companion lint validation step MUST fail
- **THEN** the stale suppression MUST be removed before the quality gate is considered passing

### Requirement: High-signal Go linters are enabled after cleanup
Invowk SHALL expand lint coverage for readability, maintainability, modern Go idioms, and subtle correctness gotchas without accepting unresolved findings.

#### Scenario: Exported documentation linting is enabled
- **WHEN** golangci-lint runs on the root module or `tools/goplint`
- **THEN** exported-symbol documentation linting MUST be enabled where supported by golangci-lint
- **THEN** existing findings MUST be fixed or suppressed with local, specific rationale

#### Scenario: Modern integer range idioms are enforced
- **WHEN** golangci-lint runs on the root module or `tools/goplint`
- **THEN** integer range modernization checks MUST be enabled where supported by the selected golangci-lint version
- **THEN** existing findings MUST be fixed unless a local suppression documents why the older loop form is clearer or required

#### Scenario: Missing test parallelism is enforced
- **WHEN** golangci-lint runs on Go tests in the root module or `tools/goplint`
- **THEN** the lint gate MUST enforce that eligible tests and subtests call `t.Parallel()`
- **THEN** tests that cannot run in parallel because they mutate process-global state, rely on serial filesystem state, require exclusive external resources, or exercise order-sensitive integration behavior MUST be locally excluded with rationale

#### Scenario: Context propagation findings are enforced or locally justified
- **WHEN** golangci-lint runs on production Go code
- **THEN** context propagation linting MUST be enabled where it produces actionable findings
- **THEN** every remaining context propagation exception MUST be local and explain why a new or fallback context is intentional

#### Scenario: Unchecked type assertions are hardened
- **WHEN** golangci-lint runs on production Go code
- **THEN** unchecked type assertions MUST be reported by lint tooling
- **THEN** existing unchecked type assertions MUST be converted to checked assertions or locally suppressed with a rationale that explains the invariant

#### Scenario: Noisy linters are not enabled as broad contradictory gates
- **WHEN** maintainers consider complexity, duplication, preallocation, contained-context, or highly subjective style linters
- **THEN** they MUST NOT enable them as blocking gates unless existing findings are cleaned or narrowly excluded
- **THEN** any excluded linter MUST be documented as intentionally omitted rather than accidentally forgotten

### Requirement: Test parallelism policy is truthful
Invowk SHALL align agent-facing test parallelism rules with the actual linters and exceptions used by the repository.

#### Scenario: Rules name the correct linter responsibilities
- **WHEN** `.agents/rules/testing.md` or related agent guidance describes test parallelism enforcement
- **THEN** it MUST distinguish missing-`t.Parallel()` enforcement from `tparallel` placement and subtest checks
- **THEN** it MUST name the enabled linter or validation step responsible for each behavior

#### Scenario: Parallelism exceptions are intentional
- **WHEN** a test is excluded from missing-`t.Parallel()` enforcement
- **THEN** the exclusion MUST identify the specific shared state, timing, platform, external resource, or integration behavior that prevents safe parallel execution
- **THEN** broad package-wide exclusions MUST be avoided unless every test in the package shares the same constraint

### Requirement: Goplint exception governance is enforced
Invowk SHALL keep `tools/goplint` baseline and exception governance aligned with its lint, type-system, canonical semantic-analysis, and soundness-assurance quality gates.

#### Scenario: Goplint lint and soundness gates run together
- **WHEN** repository lint gates run
- **THEN** `tools/goplint` golangci-lint checks MUST run in addition to the custom goplint analyzer gates
- **THEN** the canonical goplint soundness-assurance gate MUST run without a legacy, alternate, fallback, or weakened semantic path
- **THEN** neither custom analyzer nor soundness gates MUST be treated as a replacement for the module's golangci-lint config

#### Scenario: Accepted goplint exceptions are reviewable
- **WHEN** a goplint exception is kept in `tools/goplint/exceptions.toml`
- **THEN** it MUST include a reason that explains why the exception remains acceptable
- **THEN** long-lived or broad exceptions MUST include a review date or equivalent review mechanism

#### Scenario: Stale goplint exception audit is part of quality gates
- **WHEN** repository quality gates run locally or in CI
- **THEN** stale, overdue, malformed, or unsupported goplint exceptions MUST be reported
- **THEN** the gate MUST fail for stale or malformed exceptions unless the design explicitly marks a temporary advisory transition inside this same change

#### Scenario: Baseline uses the only production semantics
- **WHEN** `make check-baseline` or `make update-baseline` invokes goplint
- **THEN** it MUST use the same canonical production analysis and fail-closed aggregation as the blocking full repository scan
- **THEN** it MUST NOT retain a legacy fact reader, AST fallback, alternate evaluator, hidden selector, or mode-specific stable-ID path
- **THEN** stable finding ID changes MUST be reported and reviewed before the baseline is accepted

#### Scenario: Goplint baseline wording matches behavior
- **WHEN** baseline tooling, goplint documentation, or agent guidance describes baseline behavior
- **THEN** it MUST distinguish baseline-suppressed categories from always-visible hard-blocking categories
- **THEN** stale statements about accepted counts, alternate semantics, or advisory soundness scans MUST be removed

#### Scenario: Canonical full scan is blocking
- **WHEN** the repository goplint full scan runs locally, in pre-commit, or in CI
- **THEN** violations, blocking inconclusive outcomes, malformed evidence, incomplete catalog/oracle coverage, surviving or non-causal soundness mutants, legacy-path detections, and analyzer failures MUST fail the gate
- **THEN** the workflow MUST NOT downgrade or mask those outcomes

### Requirement: Documentation and verification remain synchronized
Invowk SHALL update documentation and validation so contributors can run, understand, and trust the lint and canonical goplint soundness-assurance gates, including generation and verification of the retained exact-tree evidence bundle used for completion claims.

#### Scenario: Command documentation lists complete lint workflow
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint documentation
- **THEN** they MUST see how to run root lint, `tools/goplint` lint, formatter and config checks, exception governance, full scan, the aggregate soundness-assurance gate, and both generation and verification of the retained exact-tree evidence bundle used by `make check-goplint-soundness-complete`
- **THEN** documented commands and guarantee claims MUST match implemented targets, CI jobs, production code paths, and retained evidence

#### Scenario: Agent documentation sync check passes
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/`
- **THEN** `make check-agent-docs` MUST pass before the change is complete

#### Scenario: Final validation proves production semantics and evidence integrity
- **WHEN** this change is complete
- **THEN** maintainers MUST run both-module lint, formatter, config, test, race, repeat, baseline, exception, full-scan, performance, and agent-document gates
- **THEN** maintainers MUST run real-analyzer counterexamples, catalog completeness, bounded independent oracle, meaningful deterministic fuzz seeds, real package-order determinism, causal targeted mutation, legacy-path absence, and strict OpenSpec validation
- **THEN** every required gate MUST pass in the recorded clean synthetic-tree worktree without optimistic uncertainty, hidden compatibility behavior, missing evidence, or unreviewed baseline drift

#### Scenario: Documented completion commands match the implementation
- **WHEN** contributors read `.agents/rules/commands.md`, `AGENTS.md`, Make help, or goplint documentation
- **THEN** the documented generation and verification commands for retained exact-tree evidence MUST match the implemented targets and CI jobs
- **AND** removed rollout phases and semantic-mode flags MUST NOT remain documented as supported behavior

### Requirement: Soundness evidence is category-specific and causally executed
Every soundness evidence layer SHALL identify the exact goplint category and semantic feature it exercises, SHALL execute through its declared production or independent boundary, and SHALL emit a machine-verifiable observation consumed by the blocking gate. Semantic-kind predicates, source-file existence, test-name markers, nonempty fuzz seeds, or shared generic evidence MUST NOT award category coverage by themselves.

#### Scenario: New protocol category has no inherited evidence
- **WHEN** a protocol category is added without category-specific production, independent-model, metamorphic, fuzz, mutation, and determinism observations
- **THEN** the semantic census and aggregate soundness gate MUST fail
- **AND** generic evidence registered for other protocol categories MUST NOT satisfy the missing layers

#### Scenario: Category evidence reaches extraction and reporting
- **WHEN** a protocol category claims production-boundary coverage
- **THEN** its evidence MUST exercise applicable source extraction, identity and graph construction, propagation, refinement, aggregation, and diagnostic reporting
- **AND** a direct component call or marker-only artifact MUST be labeled supporting evidence rather than end-to-end proof

#### Scenario: Historical fuzz seed proves its declared feature
- **WHEN** the audit matrix maps a historical counterexample to a committed fuzz seed
- **THEN** the gate MUST decode that seed, observe the exact declared semantic structure, and demonstrate the independent property that detects the counterexample
- **AND** nonempty input or an unrelated shared graph shape MUST NOT count as coverage

### Requirement: Aggregate soundness orchestration rejects vacuous subgates
The aggregate goplint soundness gate SHALL execute every required subgate through a canonical machine-readable manifest and SHALL validate the expected evidence from that execution. Target names, dependency declarations, recipe text, test definitions, or marker strings alone MUST NOT prove that a subgate ran.

#### Scenario: Required recipe is replaced by a no-op
- **WHEN** an adversarial gate test replaces any required subgate command with a successful no-op
- **THEN** the gate contract MUST fail because the required evidence was not produced by the declared command

#### Scenario: Empty evidence population cannot pass
- **WHEN** a subgate executes with zero admitted programs, categories, seeds, mutants, deterministic reorderings, benchmarks, or counterexamples where a nonzero population is required
- **THEN** the subgate and aggregate gate MUST fail

#### Scenario: Unrelated failure is not causal evidence
- **WHEN** a mutation or adversarial run fails for compilation, timeout, crash, unrelated test failure, or a pre-existing failing control
- **THEN** the gate MUST reject the result as non-causal
- **AND** it MUST NOT report the intended semantic guard as proven

### Requirement: Independent evidence exercises integrated production semantics
Generated comparison, fuzzing, perturbation, scheduled profiles, and analyzer benchmarks SHALL exercise the integrated production analyzer dimensions named by their manifests. The independent reference model MUST represent the corresponding facts, aliases, constraints, call sites, and realizable call/return behavior without calling production semantic helpers.

#### Scenario: Evidence corruption enters before production validation
- **WHEN** an end-to-end perturbation corrupts witness, refinement, reason, or summary evidence
- **THEN** corruption MUST be injected before production evidence checking and aggregation
- **AND** editing the analyzer result after execution MUST NOT satisfy the perturbation requirement

#### Scenario: Differential fuzzing couples solver dimensions
- **WHEN** a fuzz program declares aliases, constraints, procedures, call sites, or return edges
- **THEN** both the production analyzer and independent interpreter MUST use those dimensions in the compared outcome
- **AND** realizability, alias, and constraint properties MUST NOT be checked only as disconnected component laws

#### Scenario: Scheduled profile compares the real analyzer
- **WHEN** the scheduled oracle profile runs
- **THEN** it MUST enumerate a manifest-derived strict superset of the blocking corpus and compare every admitted case with the production analyzer
- **AND** a documented Make or CI surface MUST invoke it with a derived, self-checked case count

#### Scenario: Generated-analysis benchmark measures the analyzer
- **WHEN** a benchmark is reported as generated analyzer performance
- **THEN** it MUST include parsing, typing, SSA extraction, graph construction, propagation, aggregation, and reporting for generated programs
- **AND** reference-interpreter-only timing MUST be named and budgeted separately

### Requirement: Clean-tree completion evidence is freshness checked
The soundness workflow SHALL provide a blocking verifier that recomputes the exact synthetic tree and intended-diff identity and validates that every required result was produced for that tree after final artifact and task state. A retained evidence file without successful freshness verification MUST NOT satisfy completion.

#### Scenario: Intended diff changes after evidence generation
- **WHEN** any intended tracked or untracked content changes after the clean-tree proof is recorded
- **THEN** the freshness verifier and aggregate soundness gate MUST fail until the proof is rerun for the new synthetic tree

#### Scenario: Required result is absent or stale
- **WHEN** the evidence record omits a required subgate, counterexample, category observation, mutant attribution, manifest identity, toolchain identity, or final task-state identity
- **THEN** the verifier MUST reject the record with the missing or mismatched field

#### Scenario: Verification preserves the caller index
- **WHEN** the freshness verifier materializes and checks the intended tree
- **THEN** it MUST use a temporary index or equivalent isolated mechanism
- **AND** the caller's real index and worktree contents MUST remain byte-for-byte unchanged

### Requirement: Finding identities are globally scoped and source-layout independent
Every stable goplint finding ID SHALL include full package-path semantic identity and SHALL remain invariant under unrelated file ordering, file length, token-position, message, and package-leaf collisions. Baseline lookup MUST NOT suppress findings from a different import path or semantic source.

#### Scenario: Equal package leaf names do not collide
- **WHEN** two import paths contain the same package leaf, type, member, and diagnostic category
- **THEN** their stable finding IDs MUST differ by full package identity
- **AND** baselining one finding MUST NOT suppress the other

#### Scenario: Unrelated source edits do not rotate IDs
- **WHEN** unrelated declarations or files are inserted, removed, reordered, or reformatted without changing a finding's semantic identity
- **THEN** that finding's stable ID MUST remain byte-identical
- **AND** raw `token.Pos` or file-set ordinal values MUST NOT participate in the ID

#### Scenario: Stable-ID migration is reviewed
- **WHEN** correcting an ID algorithm changes existing repository IDs
- **THEN** tooling MUST produce a deterministic old-to-new migration and collision report from repeated identical scans
- **AND** unexplained churn or a collision MUST block baseline acceptance

### Requirement: Goplint directives are total and fail visibly
Goplint SHALL validate directive names, attachment locations, arguments, duplication, and conflicts across field, declaration, type, function, method, and file documentation. An unknown, misspelled, incomplete, misplaced, or conflicting directive MUST produce an actionable failure rather than silently disabling or weakening a check.

#### Scenario: Type-level typo is rejected
- **WHEN** type or declaration documentation contains an unknown directive resembling `enum-cue`, `nonzero`, `path-domain`, or another supported directive
- **THEN** goplint MUST report the unknown directive at its source location
- **AND** the associated check MUST NOT silently disappear

#### Scenario: Parameterized directive requires a value
- **WHEN** a known parameterized directive is present without its required value or with an invalid value
- **THEN** directive validation MUST fail before the directive consumer runs
- **AND** recognizing only the directive name MUST NOT count as valid configuration

#### Scenario: Duplicate or conflicting directives fail
- **WHEN** the same declaration contains duplicate or mutually incompatible goplint directives
- **THEN** goplint MUST emit one deterministic actionable configuration error
- **AND** traversal order MUST NOT choose one directive silently

### Requirement: Semantic evidence credit matches executed production stages
Every goplint evidence observation SHALL derive its category, semantic feature, boundary, stages, dimensions, properties, and population from cases actually executed by its producer. Declarations, labels, fixture ordering, hard-coded counts, or a final reporting mutation MUST NOT award credit for unobserved semantics.

#### Scenario: Metamorphic evidence transforms semantics-bearing input
- **WHEN** a category claims metamorphic coverage
- **THEN** the producer MUST apply a documented semantics-preserving or predictably semantics-changing transformation to a program or semantic model
- **AND** merely reversing independent fixture order MUST NOT satisfy the relation

#### Scenario: Fuzz evidence decodes variable semantic structure
- **WHEN** a category claims fuzz coverage
- **THEN** input bytes MUST control reviewed variable facts, identities, aliases, branches, procedures, calls, returns, effects, or constraints relevant to that category
- **AND** the target MUST check an independent semantic property rather than only labels, determinism, or internal consistency

#### Scenario: Determinism credit is category specific
- **WHEN** a category claims file, package, map, worklist, or equivalent-schedule determinism
- **THEN** that category's real analyzer cases MUST execute under every credited reordering
- **AND** a stable unrelated global corpus MUST NOT transfer determinism credit to the category

#### Scenario: Mutation stages match the mutated boundary
- **WHEN** a category mutant changes only diagnostic reporting
- **THEN** its observation MAY claim reporting-stage mutation evidence only
- **AND** extraction, identity, graph, propagation, refinement, or aggregation credit MUST require separate causal mutants through those applicable stages

#### Scenario: Fixed fixtures retain honest labels
- **WHEN** expected outcomes come from explicit declarative fixtures rather than an executable independent interpreter
- **THEN** the evidence MUST be labeled independent boundary-oracle evidence
- **AND** it MUST NOT claim an integrated independent-model comparison

### Requirement: Aggregate subgate populations are executable censuses
Every aggregate subgate SHALL prove that each required test, case, shard, category, seed, mutant, benchmark, or other population member exists and executed in the current run. A successful command with no matching tests or with a hard-coded population MUST fail evidence validation.

#### Scenario: Missing regex-selected test fails
- **WHEN** a subgate's `go test -run` pattern matches no test or omits any required named test
- **THEN** the subgate MUST fail before emitting a successful report
- **AND** Go's zero-exit no-tests behavior MUST NOT count as execution evidence

#### Scenario: Population comes from observations
- **WHEN** a subgate reports a nonzero population
- **THEN** the report count MUST be derived from uniquely observed current-run members
- **AND** a constant count, duplicate observation, skipped shard, or prior report MUST be rejected

#### Scenario: Contract mutation removes a test
- **WHEN** adversarial gate tests delete or rename each required test while leaving the command and hard-coded report path intact
- **THEN** the owning subgate and aggregate runner MUST fail

### Requirement: Mutation kills prove the intended mismatch
The targeted mutation gate SHALL accept a kill only when clean controls pass, the exact declared transformation compiles, the expected guard observes the declared semantic mismatch, restoration succeeds, and repeated post-controls remain clean. Expected-test-name failure alone MUST NOT establish causality.

#### Scenario: Expected test fails for unrelated assertion
- **WHEN** the named guard fails because of setup, unrelated assertion, environmental error, or a mismatch different from the mutant's declared concern
- **THEN** the runner MUST classify the mutant invalid rather than killed
- **AND** no semantic observation or stage credit may be emitted

#### Scenario: Every blocking mutant is killed causally
- **WHEN** the blocking profile completes
- **THEN** every selected mutant MUST have zero survivors and one structured intended-mismatch attribution
- **AND** compilation failure, timeout, panic, generic `FAIL`, missing test, or pre-existing failure MUST remain non-kill outcomes

#### Scenario: Post-validation mutants cannot survive
- **WHEN** mutation removes post-validation summary or unresolved-effect transfer
- **THEN** a production-boundary guard MUST fail for the exact lost violation or inconclusive outcome
- **AND** both post-validation mutants identified by this review MUST be killed before completion

### Requirement: Completion proof covers the complete dependent diff
The goplint soundness completion proof SHALL materialize and verify every intended tracked and untracked change across `complete-goplint-soundness-hardening`, `close-goplint-soundness-review-gaps`, and `close-residual-goplint-soundness-gaps`. Omitted changed paths, incomplete task state, stale artifacts, or out-of-order archives MUST invalidate completion.

#### Scenario: Changed path is omitted from proof selection
- **WHEN** any changed or untracked repository path is absent from the proof selection without an explicit reviewed unrelated-path exclusion
- **THEN** materialization or freshness verification MUST fail
- **AND** the omitted content MUST NOT remain outside the synthetic-tree identity silently

#### Scenario: Combined proof follows final task state
- **WHEN** any predecessor or current task, manifest, counterexample, baseline, evidence producer, documentation claim, or artifact changes after proof generation
- **THEN** freshness verification and the completion profile MUST fail until the proof is regenerated

#### Scenario: Archive order preserves dependencies
- **WHEN** the combined final tree passes every required gate and freshness check
- **THEN** maintainers MUST synchronize and archive the three changes in dependency order with strict validation after each transition
- **AND** artifact readiness or a partially complete task ledger MUST NOT authorize an earlier archive

#### Scenario: Completion record is one combined authority
- **WHEN** completion evidence is retained
- **THEN** one reviewed record MUST bind the exact base, full intended diff, synthetic tree, toolchain, task ledgers, commands, observations, mutation attributions, populations, and outcomes
- **AND** separate partial records MUST NOT substitute for the combined proof

### Requirement: Mutation-kernel coverage contract is a documented blocking subgate
Invowk SHALL document the blocking mutation-kernel category-coverage contract in the same documentation surfaces that describe other blocking goplint gates, so contributors can see that mutation coverage is a first-class blocking requirement rather than an implementation detail.

#### Scenario: Kernel coverage subgate appears in documentation
- **WHEN** contributors read `.agents/rules/commands.md`, `.agents/rules/checklist.md`, `tools/goplint/AGENTS.md`, or `tools/goplint/README.md`
- **THEN** they MUST see that the blocking mutation profile MUST cover every semantic category whose registered rule in `tools/goplint/spec/semantic-rules.v1.json` requires the `mutation` evidence layer
- **AND** they MUST see the exact command that runs the kernel-coverage subgate
- **AND** they MUST see that the subgate cannot be baselined, excepted, or inline-ignored

#### Scenario: Repo hygiene excludes goplint test binaries
- **WHEN** contributors build or run goplint tests locally
- **THEN** the resulting `tools/goplint/**/*.test` binaries MUST be ignored by `.gitignore`
- **AND** `git status` MUST NOT list them as untracked candidates for accidental staging
