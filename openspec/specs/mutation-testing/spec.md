# mutation-testing Specification

## Purpose
Define Invowk's mutation-testing contract so mutation runs stay reproducible, baseline-aware, safe for local worktrees, and separate from the regular test matrix while providing useful test-quality signal for root-module and `tools/goplint` Go code.

## Requirements

### Requirement: Pinned mutation-testing toolchain
Invowk SHALL use an exact pinned Go mutation-testing tool version for local and CI mutation runs, and SHALL NOT install or invoke an unversioned or `latest` mutation-testing tool in repository automation.

#### Scenario: Tool version is pinned
- **WHEN** maintainers inspect mutation-testing configuration, Make targets, scripts, and workflows
- **THEN** every mutation-testing tool install or invocation SHALL reference an exact version or a Go tool dependency resolved through `go.mod`
- **THEN** no mutation-testing automation SHALL use `@latest`, floating tags, or unversioned install scripts

#### Scenario: Tool version is verified in CI
- **WHEN** a mutation-testing CI job installs or resolves the mutation-testing tool
- **THEN** the job SHALL verify the resolved binary version before running mutants
- **THEN** a mismatch between the expected and resolved version SHALL fail the job before mutation execution

#### Scenario: Version policy stays synchronized
- **WHEN** the pinned mutation-testing tool version changes
- **THEN** Invowk SHALL update every workflow, script, Make target, and agent-facing version-pinning rule that references the version

### Requirement: Mutation run profiles
Invowk SHALL provide named mutation run profiles for changed-line PR feedback, scheduled broad scans, baseline updates, dry-runs, and focused single-mutant reruns, scoped to the root Go module. Goplint's mutation profile migrates to the `invowk/goplint` repository.

#### Scenario: Changed-line PR profile runs
- **WHEN** a pull request mutation profile runs against a branch with Go production-line changes
- **THEN** Invowk SHALL mutate only changed eligible Go source lines relative to the configured base ref
- **THEN** the profile SHALL exit successfully when no eligible mutations are generated for the change

#### Scenario: Scheduled full profile runs
- **WHEN** a scheduled or manual full mutation profile runs
- **THEN** Invowk SHALL use the curated target manifest for the root module
- **THEN** the profile SHALL write reports for the root-module profile

#### Scenario: Baseline update profile runs intentionally
- **WHEN** maintainers run the baseline update profile
- **THEN** Invowk SHALL regenerate the accepted-survivor baseline for the root-module profile
- **THEN** the command SHALL make clear that the baseline update is an intentional maintenance operation

#### Scenario: Single mutant rerun is supported
- **WHEN** maintainers provide a stable escaped-mutant ID to the focused rerun profile
- **THEN** Invowk SHALL rerun only that mutant for the root-module profile
- **THEN** the command SHALL preserve enough report output to guide a targeted killing test

#### Scenario: Goplint mutation coverage lives with goplint
- **WHEN** maintainers need mutation testing over goplint's analyzer sources
- **THEN** the `invowk/goplint` repository MUST provide the equivalent curated-manifest profile, baseline, and workflow
- **THEN** invowk automation MUST NOT offer a `goplint` module selection it can no longer execute

### Requirement: Safe local execution
Invowk SHALL protect developer worktrees from accidental source mutation during local mutation-testing runs.

#### Scenario: Dirty worktree is rejected
- **WHEN** a local mutation run would rewrite Go source files in place and the worktree contains uncommitted tracked changes outside approved report or baseline outputs
- **THEN** Invowk SHALL fail before mutation execution with an actionable message

#### Scenario: Clean local run restores sources
- **WHEN** a local mutation run completes, fails, or is interrupted after mutating source files
- **THEN** Invowk SHALL restore mutated source files to their pre-run contents or run the mutation tool in an isolated temporary worktree
- **THEN** Invowk SHALL remove untracked files created during the mutation run while preserving untracked files that existed before the run

#### Scenario: Dry-run does not mutate sources
- **WHEN** maintainers run the mutation dry-run profile
- **THEN** Invowk SHALL count or list candidate mutants without executing tests against mutated source files
- **THEN** the command SHALL NOT leave source changes in the worktree

### Requirement: Target selection and exclusions
Invowk SHALL select mutation targets deliberately so mutation testing measures production Go behavior rather than generated files, fixtures, docs, website assets, test-only support code, or surfaces that require a separate high-assurance oracle.

#### Scenario: Root module target manifest excludes non-production surfaces
- **WHEN** the root module mutation profile resolves its targets
- **THEN** it SHALL use a curated set of eligible production packages under `internal/` and `pkg/` for the initial full profile
- **THEN** it SHALL exclude `tests/`, `website/`, `docs/`, `samples/`, `specs/`, `openspec/`, generated artifacts, testdata fixtures, and Go test files
- **THEN** large adapter, runtime, TUI, audit, or container surfaces SHALL remain opt-in until advisory timing and survivor data justify adding them to a baselineable full profile

#### Scenario: Goplint target manifest runs from nested module
- **WHEN** maintainers need the former nested-module goplint mutation profile
- **THEN** it now runs from the `invowk/goplint` repository root with that repository's dependency graph, reports, and baseline
- **THEN** invowk MUST NOT retain the nested-module manifests or baseline

#### Scenario: Packages without local test ownership are visible
- **WHEN** a target manifest includes production packages or file targets whose owning package has no local Go tests
- **THEN** the mutation workflow SHALL either exclude them with an explicit rationale or report them as not covered rather than hiding them silently

### Requirement: Baseline and gate behavior
Invowk SHALL support brownfield adoption by distinguishing known accepted surviving mutants from new escaped mutants.

#### Scenario: Known survivors do not fail new-code gate
- **WHEN** a blocking mutation profile runs with a baseline
- **THEN** mutants already accepted in the baseline SHALL NOT fail the job
- **THEN** newly escaped mutants outside the baseline SHALL fail the job

#### Scenario: Advisory mode never blocks implementation
- **WHEN** a mutation profile is configured as advisory
- **THEN** escaped mutants SHALL be reported through logs, annotations, and artifacts
- **THEN** the job SHALL NOT fail solely because mutants escaped

#### Scenario: Blocking mode fails on new escapes
- **WHEN** a mutation profile is configured as blocking
- **THEN** the job SHALL fail when a new escaped mutant is detected or when configured mutation score gates are not met

#### Scenario: Baselines can shrink
- **WHEN** maintainers add tests that kill previously accepted survivors
- **THEN** the baseline update profile SHALL allow the accepted-survivor baseline to be regenerated with those mutants removed

### Requirement: Reports and annotations
Invowk SHALL produce machine-readable mutation-testing reports, human-readable CI output, and GitHub annotations suitable for review and agent-assisted test improvement.

#### Scenario: PR report artifacts are uploaded
- **WHEN** a pull request mutation profile runs in CI
- **THEN** Invowk SHALL upload summary and escaped-mutant report artifacts for the profile
- **THEN** the artifacts SHALL be named so root-module and `tools/goplint` reports are distinguishable

#### Scenario: Escaped mutants are annotated
- **WHEN** a pull request mutation profile finds escaped mutants on changed lines
- **THEN** CI SHALL emit GitHub annotations that identify the affected file, line, mutator, and profile

#### Scenario: Agent-focused report is available
- **WHEN** a mutation profile finds escaped mutants
- **THEN** Invowk SHALL produce a report containing stable mutant IDs, diffs, surrounding context, and nearby test hints when the selected tool supports that output

#### Scenario: Summary metrics are machine-readable
- **WHEN** a mutation profile completes
- **THEN** Invowk SHALL produce machine-readable summary metrics including total mutants, killed mutants, escaped mutants, skipped or errored mutants, and mutation score when the selected tool supports those fields

### Requirement: CI integration
Invowk SHALL integrate mutation testing with GitHub Actions without increasing the cost of the regular test matrix by default.

#### Scenario: Regular test workflow remains separate
- **WHEN** `make test` or the existing CI test matrix runs
- **THEN** mutation testing SHALL NOT run as part of those commands unless a mutation-specific target or workflow is selected

#### Scenario: Pull request workflow fetches diff base
- **WHEN** a pull request mutation workflow uses changed-line filtering
- **THEN** checkout SHALL fetch enough Git history to compute the configured base diff accurately

#### Scenario: Scheduled workflow runs broad scans
- **WHEN** the scheduled mutation workflow runs
- **THEN** it SHALL execute the configured broad scan profiles and upload reports even when no pull request context exists

#### Scenario: Workflow permissions are minimal
- **WHEN** mutation-testing workflows run in GitHub Actions
- **THEN** they SHALL request only the permissions required for checkout, annotations, artifacts, and any explicitly configured report publication

### Requirement: Test execution boundaries
Invowk SHALL keep default mutation runs focused on package-level Go tests while allowing explicitly selected high-assurance execution profiles for cross-package or CLI-only oracles.

#### Scenario: Default mutation profile avoids race and container overhead
- **WHEN** a default mutation profile executes tests for each mutant
- **THEN** it SHALL NOT pass `-race`, run container engine tests, or run CLI `testscript` suites unless explicitly configured for that profile

#### Scenario: Focused high-assurance profile can use custom execution
- **WHEN** maintainers select a high-assurance mutation profile for a package whose behavior is primarily tested through cross-package or CLI tests
- **THEN** Invowk SHALL allow that profile to use an explicit custom execution command or test flag set

#### Scenario: Integration profiles are opt-in
- **WHEN** a mutation profile would require Docker, Podman, tmux, network access, platform-specific runners, or long-running CLI fixtures
- **THEN** that profile SHALL be opt-in and SHALL document its expected prerequisites and runtime cost

### Requirement: Documentation and validation
Invowk SHALL document mutation-testing usage and validate the wrapper logic without requiring a full mutation scan in normal validation.

#### Scenario: Developer commands are documented
- **WHEN** maintainers read agent-facing command documentation
- **THEN** it SHALL list the mutation-testing Make targets or scripts, required tools, baseline update workflow, and report locations

#### Scenario: Wrapper logic has tests
- **WHEN** repository script tests run
- **THEN** they SHALL validate mutation target resolution, dirty-worktree protection, report path calculation, and command construction without executing a full mutation scan

#### Scenario: OpenSpec and agent docs remain synchronized
- **WHEN** implementation changes `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` for mutation-testing guidance
- **THEN** `make check-agent-docs` SHALL pass before the change is complete

### Requirement: Mutation tool upgrades preserve PR targeting semantics
Invowk SHALL keep changed-line mutation testing aligned with pull request review semantics when the pinned mutation tool gains improved diff-base behavior.

#### Scenario: Changed-line mutation uses the intended base
- **WHEN** maintainers update the pinned mutation-testing tool to a version that changes changed-line diff handling
- **THEN** the pull request mutation profile MUST continue mutating eligible production Go lines relative to the intended PR base
- **THEN** wrapper tests or focused dry-run evidence MUST prove the selected base-ref behavior is still correct

#### Scenario: Baselines are not recomputed by tool refresh
- **WHEN** maintainers update the pinned mutation-testing tool as part of dependency maintenance
- **THEN** accepted-survivor baselines MUST NOT be regenerated unless the change explicitly enters the baseline update profile
- **THEN** any baseline change in the same implementation MUST be justified as intentional survivor triage rather than routine tool refresh

### Requirement: Mutation tool upgrades preserve report contracts
Invowk SHALL preserve machine-readable mutation reports and stable mutant identifiers across routine mutation-testing tool upgrades.

#### Scenario: Report files remain available
- **WHEN** mutation wrapper tests run after a mutation tool upgrade
- **THEN** expected summary, agentic, GitLab, HTML, log, target-resolution, excluded-package, and not-covered-package report paths MUST remain stable unless the design documents a migration

#### Scenario: Automation avoids terminal-label scraping
- **WHEN** mutation terminal labels change across tool versions
- **THEN** automation MUST continue using machine-readable report fields or stable mutant IDs instead of parsing human terminal labels
- **THEN** human-facing documentation MUST identify the current terminal labels and preserve historical label notes as version-scoped evidence

### Requirement: Mutation terminal labels are current and unambiguous
Invowk SHALL document and interpret mutation-testing terminal output labels according to the pinned mutation-testing tool version.

#### Scenario: Current tool output uses explicit labels
- **WHEN** Invowk uses a mutation-testing tool version whose terminal output labels killed mutants as `KILLED` and surviving mutants as `ESCAPED`
- **THEN** current documentation and agent guidance MUST use `KILLED` for killed mutants
- **AND** current documentation and agent guidance MUST use `ESCAPED` for surviving mutants

#### Scenario: Historical output labels are version-scoped
- **WHEN** historical triage notes refer to older mutation-testing output where `PASS` meant killed and `FAIL` meant escaped
- **THEN** those notes MUST clearly identify that the label interpretation applies to the older tool version that produced the evidence
- **AND** those notes MUST NOT present `PASS` and `FAIL` as the current mutation-testing terminal labels

#### Scenario: Automation relies on machine-readable reports
- **WHEN** mutation-testing gates decide whether a run passes or fails
- **THEN** they MUST use machine-readable report fields or stable mutant IDs when available
- **AND** they MUST NOT introduce new parsing of human terminal status labels unless no machine-readable alternative exists

### Requirement: Deterministic property-based tests under mutation profiles
Mutation profiles SHALL run property-based tests deterministically so that each mutant's kill or escape status is reproducible against the stable-ID baseline. The mutation wrapper SHALL set a fixed `RAPID_SEED`, `RAPID_NOFAILFILE=1`, and a bounded `RAPID_SHRINKTIME` for every mutated test run.

#### Scenario: Mutant status is reproducible
- **WHEN** the same mutant is run twice under the same profile
- **THEN** the rapid tests SHALL use the same seed both times, and the mutant's kill or escape status SHALL be identical

#### Scenario: No failure files are written
- **WHEN** a mutant makes a rapid test fail
- **THEN** no file SHALL be written under any package's `testdata/rapid/` directory

#### Scenario: Shrinking is bounded
- **WHEN** a rapid test fails under a mutation profile
- **THEN** shrinking SHALL stop within the configured `RAPID_SHRINKTIME`

#### Scenario: Packages outside the curated manifest
- **WHEN** rapid tests are added to packages absent from `tools/mutation/root-packages.txt`
- **THEN** the formal-verification documentation SHALL state that those tests add no killers to the full profile until the manifest is changed deliberately

### Requirement: Formal-bindings mutation target set
Invowk SHALL provide a `formal-bindings` mutation target set. It mutates only the Go functions named in bound rows of the formal models' correspondence tables, and runs only those functions' killer tests. It SHALL be selectable on the `dry-run`, `full`, `baseline-update`, and `rerun` profiles, and through the Make targets `mutation-formal-dry-run`, `mutation-formal`, `mutation-formal-baseline-update`, and `mutation-formal-rerun`. The `pr` profile SHALL reject it.

#### Scenario: Targets come from the correspondence tables
- **WHEN** a formal-bindings profile resolves its targets
- **THEN** the files, function names, line ranges, killer tests, and their packages SHALL be generated at run time from the correspondence tables and `formal/manifest.toml` as they exist in the checked-out tree
- **THEN** rows and bindings added by other changes SHALL be included without editing any list
- **THEN** no hand-maintained target list SHALL be consulted
- **THEN** the generated plan SHALL be written to the profile's report directory

#### Scenario: Unbound rows are reported, not mutated
- **WHEN** a correspondence row has binding `-`, names a type or interface, has only `Test*_TraceHarness` bindings, or has only characterisation bindings
- **THEN** the row SHALL NOT be mutated
- **THEN** it SHALL be listed in an unbound-rows report with its model, its element, and a reason of `no-binding`, `type-symbol`, `trace-only`, or `characterisation-only`

#### Scenario: Trace harnesses and characterisation tests are not killers
- **WHEN** a row's binding cell names a `Test*_TraceHarness` function or a characterisation test
- **THEN** that test SHALL NOT appear in any `-run` expression of the profile

#### Scenario: Plan generation fails closed
- **WHEN** the function-name filter would select a function in a target file that no row names for that file, a killer test cannot be located in exactly one package, the correspondence check fails, or the plan has no function targets
- **THEN** the profile SHALL exit non-zero before any mutant runs, and SHALL name the offending model and row

#### Scenario: PR profile rejects the target set
- **WHEN** the `pr` profile is invoked with the formal-bindings target set
- **THEN** the wrapper SHALL fail with an actionable message

### Requirement: Clean-code pre-flight
Before any mutant runs, the formal-bindings profile SHALL run each target file's killer tests against the unmutated code, and SHALL derive each file's exec timeout from the measured clean wall time.

#### Scenario: Broken bindings fail the run
- **WHEN** a killer test fails, skips, or times out on unmutated code
- **THEN** the profile SHALL exit non-zero before any mutant runs
- **THEN** it SHALL name the file, the function, and the test

#### Scenario: Every function has a live killer
- **WHEN** the pre-flight runs
- **THEN** at least one killer test per named function SHALL be reported as run and passed, not skipped

#### Scenario: Timeouts are derived from measurement
- **WHEN** the pre-flight completes
- **THEN** each file's exec timeout SHALL be the larger of 10 seconds and five times its clean wall time, unless explicitly overridden
- **THEN** the timeout SHALL be recorded in the plan

### Requirement: Binding-only cross-package execution
Each formal-bindings mutant SHALL be tested by running only the killer tests of the rows that name the mutated function, in every package that owns one of them, with the mutant applied through a Go build overlay rather than by rewriting the tracked file.

#### Scenario: Cross-package binding kills a mutant
- **WHEN** a mutant is applied to a function whose binding test lives in a different package (for example, `CommandScope.CanCallTarget` in `pkg/invowkmod`, bound by a test in `internal/app/deps`)
- **THEN** the executor SHALL run that binding test against the mutated code, and a test failure SHALL count as a kill

#### Scenario: Only the mutated function's bindings count
- **WHEN** a mutant is tested
- **THEN** only the killer tests of the rows naming the function that contains the mutated line SHALL run, filtered by an anchored `-run` expression
- **THEN** tests bound to other functions in the same file, or to other models, SHALL NOT run

#### Scenario: Result mapping
- **WHEN** the killer tests pass, fail, fail to build, or time out
- **THEN** the mutant SHALL be recorded as escaped, killed, skipped, or killed respectively
- **THEN** timeout kills SHALL be counted per file in the run log

#### Scenario: Golden vectors replay in full
- **WHEN** a formal-bindings mutant runs
- **THEN** the tests SHALL run without `-short`, without `-race`, with `-count=1`, and without trace validation or TLC

#### Scenario: Worktree is not rewritten
- **WHEN** a formal-bindings run completes, fails, or is interrupted
- **THEN** no tracked source file SHALL differ from its pre-run contents

### Requirement: Deterministic formal-bindings execution
The formal-bindings executor SHALL run with the wrapper's fixed rapid seed, disabled failure files, and bounded shrink time, and SHALL refuse to run without them. Focused reruns SHALL record their outcomes as evidence.

#### Scenario: Missing determinism settings are refused
- **WHEN** the executor is invoked without `RAPID_SEED`, without `RAPID_NOFAILFILE=1`, or without `RAPID_SHRINKTIME`
- **THEN** it SHALL exit with the errored status without running tests

#### Scenario: Reruns are recorded
- **WHEN** `mutation-formal-rerun` runs a mutant
- **THEN** it SHALL append the mutant ID, status, timestamp, and the SHA-256 digest of the plan inputs (the mutated target files and the files declaring their killer tests, sorted by path, each hashed as path, NUL, bytes) to `tools/mutation/triage/formal-bindings-reruns.jsonl`
- **THEN** a recorded commit, if any, SHALL be information only and SHALL NOT decide whether evidence counts

#### Scenario: Survivors are confirmed by evidence
- **WHEN** the triage check runs
- **THEN** it SHALL fail for any baselined ID without at least two recorded `escaped` reruns whose digest equals the digest the baseline records

#### Scenario: Evidence survives squash merges
- **WHEN** the branch that recorded the evidence is squash-merged and deleted, or the repository is freshly cloned
- **THEN** the triage check SHALL give the same verdict as on that branch, as long as the plan inputs are unchanged

### Requirement: Formal-bindings baseline and triage ledger
Accepted formal-bindings survivors SHALL be stored in a baseline separate from the root baseline. Each SHALL be classified in a checked triage ledger as `equivalent`, `abstraction`, `binding-gap`, `model-gap`, or `defect`, identified by model and element. The triage check SHALL run in `make test-scripts` and in `mutation-formal-baseline-update`.

#### Scenario: Ledger matches baseline
- **WHEN** the triage check runs
- **THEN** it SHALL fail unless the set of mutant IDs in `tools/mutation/baselines/formal-bindings-baseline.json` equals the set of survivor IDs in `tools/mutation/triage/formal-bindings.toml`

#### Scenario: Gaps and defects carry a follow-up
- **WHEN** a ledger entry is classified `binding-gap` or `model-gap`
- **THEN** the check SHALL fail unless the entry names a follow-up change or task
- **WHEN** a ledger entry is classified `defect`
- **THEN** the check SHALL fail unless its follow-up names a finding id declared by a `finding` command in `formal/manifest.toml`

#### Scenario: Abstractions are declared in the named row
- **WHEN** a ledger entry is classified `abstraction`
- **THEN** the check SHALL fail unless, after whitespace is collapsed, its reason is a substring of the abstraction cell of the correspondence row identified by the entry's `model` and `element`

#### Scenario: Baseline update enforces the ledger
- **WHEN** maintainers run `mutation-formal-baseline-update`
- **THEN** the wrapper SHALL run the triage check after writing the baseline, and SHALL exit non-zero while any baseline ID lacks a valid ledger entry

#### Scenario: IDs are executor-independent
- **WHEN** the same mutation is produced under the formal-bindings executor and under the built-in executor
- **THEN** both SHALL report the same stable mutant ID

### Requirement: Formal-bindings profile stays advisory
The formal-bindings profile SHALL be a manual, advisory signal. It SHALL NOT run in `make test` or on pull requests, and SHALL NOT be a required status check.

#### Scenario: Manual dispatch only
- **WHEN** the mutation-testing workflow is dispatched manually with the formal-bindings target set
- **THEN** it SHALL run in advisory mode and upload the plan, the unbound-rows report, and the go-mutesting reports

#### Scenario: Escapes do not fail advisory runs
- **WHEN** escaped mutants outside the baseline are found in advisory mode
- **THEN** the run SHALL report them and exit successfully

### Requirement: Triage check lists binding gaps and orphaned survivors
`formal/manifest.toml` SHALL hold a `[triage]` table with exactly `binding_gaps` (model, element, symbol) and `orphaned_survivors` (mutant ids). `triage --check` SHALL classify rows as the formal-bindings plan does, without `go list`, and fail on an unlisted `no-binding` row or orphaned survivor and on a stale entry. A survivor is orphaned when the row it names is unbound. The reach gate SHALL NOT read the table.

#### Scenario: New binding gap
- **WHEN** a correspondence row has no binding, names a function, and is not in `[triage] binding_gaps`
- **THEN** `triage --check` SHALL fail and name the row

#### Scenario: New orphaned survivor
- **WHEN** a baselined survivor's row becomes unbound and its id is not in `[triage] orphaned_survivors`
- **THEN** `triage --check` SHALL fail and name the survivor id and its row

#### Scenario: Stale entry
- **WHEN** a listed row is bound, unbound by design, or names no row, or a listed id is not in the ledger or its row is bound
- **THEN** `triage --check` SHALL fail and name the stale entry

#### Scenario: Unknown triage key
- **WHEN** `[triage]` contains a key other than `binding_gaps` or `orphaned_survivors`
- **THEN** manifest validation SHALL fail

### Requirement: By-design unbound rows are derived
Rows unbound by design (`no-symbol`, `type-symbol`, `trace-only`, `characterisation-only`) SHALL NOT be listed in `[triage]`. `triage --check` SHALL derive them with the plan's row classification and print them as a report. A row whose symbol names no function SHALL be `type-symbol` whether or not it has a binding.

#### Scenario: New by-design row passes
- **WHEN** a new characterisation-only or type-symbol row is added
- **THEN** `triage --check` SHALL report it and SHALL NOT fail because of it

#### Scenario: Type symbol without a binding
- **WHEN** a row names a type, field, or interface, such as `Serverbase` `State`, and has binding `-`
- **THEN** it SHALL be classified as `type-symbol`, not as a binding gap

#### Scenario: Method symbol names a function
- **WHEN** a row's symbol is a `Type.Method` that the file declares as a method
- **THEN** it SHALL be classified as a function, not as `type-symbol`

### Requirement: The triage lists cannot grow against the base
`triage --check` SHALL read the base commit's `formal/manifest.toml` and fail on any entry of `[correspondence] unresolved_elements`, `[triage] binding_gaps`, or `[triage] orphaned_survivors` that the base does not list. The base SHALL be `--base`, else a non-empty `INVOWK_TRIAGE_BASE`, else the merge-base of `HEAD` and `origin/main`. A base that cannot be determined or read SHALL fail closed with a message naming the base, `INVOWK_TRIAGE_BASE`, and `--base`.

#### Scenario: Entry absent at the base
- **WHEN** the tree lists an entry in one of the three lists that the base manifest does not list
- **THEN** `triage --check` SHALL fail and name the list, the entry, and the base

#### Scenario: Shrink passes
- **WHEN** the tree lists a subset of the base's entries
- **THEN** the growth check SHALL pass

#### Scenario: Unreadable base
- **WHEN** the base cannot be determined or `git show <base>:formal/manifest.toml` fails
- **THEN** `triage --check` SHALL fail with a message that names the base and says to fetch it, set `INVOWK_TRIAGE_BASE`, or pass `--base`

#### Scenario: Empty variable
- **WHEN** `INVOWK_TRIAGE_BASE` is set to an empty string and `--base` is not given
- **THEN** `triage --check` SHALL use the merge-base, as if the variable were unset

### Requirement: The triage lists are seeded only from a base without them
The growth check SHALL treat a base manifest that has none of the three lists as the seeding change: it SHALL pass and print one seeding notice per list. A base that has some but not all of the three lists SHALL fail for each missing list, so renaming a key cannot reopen seeding.

#### Scenario: Base that introduces no list
- **WHEN** the base manifest is readable and has none of the three lists, as for the change that introduces them
- **THEN** the growth check SHALL pass and print a seeding notice for each list

#### Scenario: Base with only some lists
- **WHEN** the base manifest has one or two of the three lists
- **THEN** the growth check SHALL fail and name each list the base lacks

### Requirement: Rows bound at the base stay bound or leave visibly
`triage --check` SHALL classify the base commit's correspondence tables, read with the model and Go sources at the base, as the plan does. It SHALL fail when a row bound at the base is unbound by design in the tree: the same (model, element, symbol) row, or every row with its model and element, is `no-symbol`, `type-symbol`, `trace-only`, or `characterisation-only`. Other departures SHALL be notices. A base with none of the three lists SHALL report demotions as notices only.

#### Scenario: Bound row demoted to characterisation
- **WHEN** a row bound at the base gets an abstraction cell beginning `characterisation:`, or only trace-harness or characterisation bindings
- **THEN** `triage --check` SHALL fail and name the row, its by-design reason, and the base

#### Scenario: Bound row deleted
- **WHEN** a row bound at the base is deleted from its table
- **THEN** `triage --check` SHALL pass and print a notice that the row was deleted

### Requirement: Unresolved rows cannot grow against the base
`triage --check` SHALL fail on a row in the tree whose `"<model>: <element>"` pair is listed in `[correspondence] unresolved_elements` and whose (model, element, symbol) row the base tables do not have, so no new row enters an unresolved pair. A base with none of the three lists SHALL report such rows as notices only.

#### Scenario: New row under a listed pair
- **WHEN** a row is added under a listed unresolved pair, such as a second `readOp` row naming `IsModule`
- **THEN** `triage --check` SHALL fail and name the row and the base

### Requirement: Entries that leave a triage list are explained
When the base lists an entry that the tree does not, `triage --check` SHALL print a notice naming the list, the entry, and why it left: for a binding gap, its row is bound, reclassified by design, or deleted; for an orphan, its row is bound or its survivor left the ledger; for an unresolved element, it resolves or its row is deleted.

#### Scenario: Gap bound since the base
- **WHEN** the base lists a binding gap whose row the tree binds
- **THEN** `triage --check` SHALL pass and print a notice that the entry left because the row is bound

### Requirement: The triage checks run in the formal lane
The formal-verification workflow SHALL run `python3 scripts/test_formal_mutation.py` and `python3 scripts/formal_mutation.py triage --check` under the `affected` condition, with `INVOWK_TRIAGE_BASE` set by `scripts/ci-triage-base.sh` in the same step. The triage ledger, the formal-bindings baseline, and the helper SHALL be in `[ci] paths`.

#### Scenario: Workflow wiring is checked
- **WHEN** `scripts/test_formal.py` checks the workflows
- **THEN** it SHALL require both commands, the `affected` condition, and the helper's output exported as `INVOWK_TRIAGE_BASE` before `triage --check` and before each `make test-scripts` in `lint.yml` and `release.yml`

### Requirement: The triage base is resolved in one place
`scripts/ci-triage-base.sh` SHALL print the base as a full SHA on stdout and notices on stderr. On `pull_request` it SHALL fetch the base branch tip at depth 1 and print it, exiting non-zero when the fetch fails. On other events it SHALL print the commit under test with a notice that the base equals HEAD.

#### Scenario: Pull-request base
- **WHEN** the helper runs on a `pull_request` event in a depth-1 checkout
- **THEN** it SHALL fetch the base branch tip and print its full SHA

#### Scenario: Base reaches the gated suite
- **WHEN** `INVOWK_TRIAGE_BASE` names a commit that only the real checkout's object store holds and `make test-scripts` runs
- **THEN** the suite inside the throwaway repository SHALL see the variable, and `git show <base>:formal/manifest.toml` SHALL resolve there

### Requirement: The unplanned lists only shrink
The `[triage]` lists SHALL be seeded once by this change: 19 binding gaps and 20 orphaned survivors (`Auth` 8, `unpackRejectsEscape` 12). Later changes SHALL NOT add entries. The survivor-triage follow-up SHALL empty `orphaned_survivors`. Listed entries SHALL be printed as a report, and the ledger's `[[survivor]]` entries SHALL NOT change to seed the lists.

#### Scenario: Seeded list passes
- **WHEN** `triage --check` runs on the tree that seeds the lists
- **THEN** it SHALL pass, report 19 listed gaps, 20 listed orphans, and the derived by-design rows, and need no `[[survivor]]` edit

#### Scenario: Removing a listed entry fails
- **WHEN** one listed entry that still applies is removed from `[triage]`
- **THEN** `triage --check` SHALL fail
