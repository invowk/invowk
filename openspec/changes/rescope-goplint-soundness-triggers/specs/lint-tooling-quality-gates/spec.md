## MODIFIED Requirements

### Requirement: Automatic goplint assurance routing is conservative and change aware
Repository automation SHALL classify every changed path into exactly one reviewed change class — `documentation`, `consumer`, `harness`, or `analyzer-semantics` — through the versioned ownership manifest, SHALL select the least expensive assurance tier that completely covers the highest class present in the diff, and SHALL fail closed to the semantic or completion profile whenever it cannot prove that a cheaper tier is sufficient.

#### Scenario: Documentation and bookkeeping changes route to the documentation tier
- **WHEN** a diff changes only paths in the `documentation` class, including goplint documentation, OpenSpec change artifacts, agent-facing markdown, and performance reports
- **THEN** automation MUST run the static goplint documentation validator and no analyzer execution
- **THEN** no repository audit, semantic population, or performance measurement MAY be triggered by that diff

#### Scenario: Consumer code changes without analyzer ownership changes
- **WHEN** a pull request changes only root-module code that consumes goplint and no harness or analyzer-semantics path
- **THEN** automation MUST run one blocking canonical repository audit with baseline and exception governance
- **THEN** it MUST NOT rerun unchanged analyzer soundness subgates solely because `cmd`, `internal`, or `pkg` changed

#### Scenario: Harness-only changes route to the harness tier
- **WHEN** a diff's highest class is `harness` — gate orchestration code, execution planners, distributed plumbing, workflow topology, or gate scripts that cannot change analyzer verdicts
- **THEN** automation MUST run the harness tier: the goplint module test suites, one shared repository audit, and the fixture-driven serial-versus-parallel normalized-report parity check
- **THEN** it MUST NOT require re-executing the full semantic populations for that diff alone

#### Scenario: Analyzer-semantics changes select the semantic profile
- **WHEN** a change touches goplint production semantics, analyzer tests, evidence producers, manifests, schemas, baselines, exceptions, threshold manifests, or governing goplint specification requirements
- **THEN** automation MUST select the semantic profile
- **THEN** the profile MUST retain every causal core population required before this change

#### Scenario: Completion event requires exhaustive evidence
- **WHEN** a completion proof, release, scheduled certification, or explicit exhaustive dispatch runs
- **THEN** automation MUST select the completion profile regardless of changed paths
- **THEN** clean-tree freshness and statistically stable performance certification MUST be blocking

#### Scenario: Executable inputs are never classified as documentation
- **WHEN** the ownership manifest assigns classes to path families
- **THEN** every file a gate reads as input — configuration, manifests, schemas, scripts, baselines, thresholds — MUST belong to `consumer`, `harness`, or `analyzer-semantics`
- **THEN** manifest validation MUST reject a `documentation` assignment for any enumerated executable-input family

#### Scenario: Change context is missing or ambiguous
- **WHEN** the merge base, changed-path census, ownership manifest, or event context is missing, malformed, stale, or ambiguous
- **THEN** routing MUST select the applicable semantic or completion profile
- **THEN** it MUST NOT silently select the documentation, consumer, or harness tier

#### Scenario: Pre-commit execution is capped below the semantic tier
- **WHEN** the local pre-commit hook routes a staged diff
- **THEN** it MUST execute at most the documentation or consumer tier locally
- **THEN** for harness or analyzer-semantics diffs it MUST report the authoritative tier that continuous integration will run
- **THEN** explicit Make targets MUST remain available to run the semantic and completion profiles locally on demand

#### Scenario: Continuous integration routes from the cumulative pull-request diff
- **WHEN** a pull-request event triggers the lint workflow
- **THEN** the selected tier MUST derive from the full base-to-head diff classification, not from the individual push
- **THEN** schedule, release, dispatch, and default-branch push events MUST retain their forced profiles

### Requirement: Clean-tree completion evidence is freshness checked
The soundness workflow SHALL provide a blocking verifier that recomputes the exact synthetic tree and intended-diff identity as two reviewed content-class digests — one over semantic content (every path class that any gate executes or reads) and one over prose — and validates that every required result was produced for the recorded semantic content after final artifact and task state. A retained evidence file without successful freshness verification MUST NOT satisfy completion.

#### Scenario: Semantic content changes after evidence generation
- **WHEN** any tracked or untracked content in a non-documentation class changes after the clean-tree proof is recorded
- **THEN** the freshness verifier and aggregate soundness gate MUST fail until the proof is regenerated with fresh gate execution for the new semantic content

#### Scenario: Prose-only drift permits cheap re-binding
- **WHEN** only `documentation`-class content differs from the retained record
- **THEN** evidence generation MUST offer a re-binding path that recomputes both digests, revalidates task ledgers and the diff census, and retains the prior aggregate report without executing any assurance profile
- **THEN** the re-bound record MUST identify the carried-forward report and the new prose digest, and verification MUST treat it as fresh

#### Scenario: Required result is absent or stale
- **WHEN** the evidence record omits a required subgate, counterexample, category observation, mutant attribution, manifest identity, toolchain identity, or final task-state identity
- **THEN** the verifier MUST reject the record with the missing or mismatched field

#### Scenario: Verification preserves the caller index
- **WHEN** the freshness verifier materializes and checks the intended tree
- **THEN** it MUST use a temporary index or equivalent isolated mechanism
- **AND** the caller's real index and worktree contents MUST remain byte-for-byte unchanged

## ADDED Requirements

### Requirement: Goplint documentation is statically anchored to executable evidence
A blocking static validator SHALL verify, in seconds and without loading Go packages or executing gates, that goplint prose remains anchored to executable artifacts.

#### Scenario: Evidence claims map to existing executables
- **WHEN** the documentation validator runs over the goplint evidence index and gate documentation
- **THEN** every claim-to-evidence row MUST name a test, gate, or observation identifier that exists in the current tree
- **THEN** every referenced Make target, subgate identifier, command, and repository path MUST exist

#### Scenario: Documentation drift is visible and blocking at its tier
- **WHEN** prose references a removed target, renamed subgate, or missing file
- **THEN** the documentation tier MUST fail with the exact stale reference
- **THEN** the failure MUST NOT be baselinable, exceptable, or inline-ignorable
