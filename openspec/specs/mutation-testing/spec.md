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
- **THEN** it SHALL fail for any baselined ID without at least two recorded `escaped` reruns whose digest equals that mutant's evidence digest: the `digest` its baseline entry records when it has one, else the digest the baseline records

#### Scenario: Per-mutant digest after a partial refresh
- **WHEN** a change replaces some baseline entries without regenerating the whole baseline
- **THEN** each new entry SHALL record, as its own `digest`, the plan-inputs digest of its reruns, and the baseline's top-level digest and the other entries' evidence SHALL stay unchanged

#### Scenario: Full regeneration drops per-mutant digests
- **WHEN** `mutation-formal-baseline-update` writes the baseline
- **THEN** it SHALL write only the top-level digest, and no entry SHALL carry its own

#### Scenario: Evidence survives squash merges
- **WHEN** the branch that recorded the evidence is squash-merged and deleted, or the repository is freshly cloned
- **THEN** the triage check SHALL give the same verdict as on that branch, as long as the plan inputs are unchanged

### Requirement: Formal-bindings baseline and triage ledger
Accepted formal-bindings survivors SHALL be stored in a baseline separate from the root baseline. Each SHALL be classified in a checked triage ledger as `equivalent`, `abstraction`, `binding-gap`, `model-gap`, or `defect`, identified by model and element, and anchored to the code it mutates by the `removed` and `added` lines of its diff (plus, for a mutant that only inserts lines, the `context` line it inserts after). The triage check SHALL run in `make test-scripts` and in `mutation-formal-baseline-update`.

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

#### Scenario: Anchor text matches the id
- **WHEN** the triage check reads a ledger entry that records `removed` and `added`
- **THEN** it SHALL fail, naming the id and symbol, unless go-mutesting's stable-id hash of the entry's file, the baseline's mutator for that id, and those lines equals the entry's id

#### Scenario: Survivor whose mutated code is gone
- **WHEN** the entry's symbol no longer names a function in its file, or the entry's anchor lines (its `removed` lines, or its `context` line when it removes none), matched verbatim with their indentation, no longer appear in order in the current file starting at a line inside that function's body, with every later code line inside that body too (a later comment line may follow the function)
- **THEN** the triage check SHALL fail and name the entry's id, its symbol, and which of the two conditions failed

#### Scenario: Edits elsewhere in a bound file
- **WHEN** a bound file is edited so that lines move, or code outside an entry's `removed` lines changes, while every entry's anchor lines still appear in its symbol's body
- **THEN** the staleness check SHALL pass

#### Scenario: Unanchored entry
- **WHEN** a ledger entry lacks `removed` or `added`, or has an empty `removed` and no `context`
- **THEN** the triage check SHALL fail and name the id and symbol, unless the id is listed in `[triage] stale_survivors` or `[triage] orphaned_survivors`

#### Scenario: Orphaned survivors are not anchored
- **WHEN** a ledger entry's id is listed in `[triage] orphaned_survivors`
- **THEN** the triage check SHALL NOT require or check its anchor, because the plan no longer targets its function

#### Scenario: Staleness check is static
- **WHEN** the triage check runs, with or without `--base`
- **THEN** it SHALL read only the current tree's Go sources, the ledger, the baseline, and `formal/manifest.toml` to decide staleness: it SHALL NOT run a mutation, `go list`, or the Go toolchain, and SHALL NOT read git history: a depth-1 checkout SHALL give the same staleness verdict

### Requirement: Formal-bindings profile stays advisory
The formal-bindings profile SHALL be a manual, advisory signal. It SHALL NOT run in `make test` or on pull requests, and SHALL NOT be a required status check.

#### Scenario: Manual dispatch only
- **WHEN** the mutation-testing workflow is dispatched manually with the formal-bindings target set
- **THEN** it SHALL run in advisory mode and upload the plan, the unbound-rows report, and the go-mutesting reports

#### Scenario: Escapes do not fail advisory runs
- **WHEN** escaped mutants outside the baseline are found in advisory mode
- **THEN** the run SHALL report them and exit successfully

### Requirement: Triage check lists binding gaps and orphaned survivors
`formal/manifest.toml` SHALL hold a `[triage]` table with exactly `binding_gaps` (model, element, symbol), `orphaned_survivors` (mutant ids) and `stale_survivors` (mutant ids). `triage --check` SHALL classify rows as the formal-bindings plan does, without `go list`, and fail on an unlisted `no-binding` row, orphaned survivor or stale survivor, and on a stale entry. A survivor is orphaned when the row it names is unbound. The reach gate SHALL NOT read the table.

#### Scenario: New binding gap
- **WHEN** a correspondence row has no binding, names a function, and is not in `[triage] binding_gaps`
- **THEN** `triage --check` SHALL fail and name the row

#### Scenario: New orphaned survivor
- **WHEN** a baselined survivor's row becomes unbound and its id is not in `[triage] orphaned_survivors`
- **THEN** `triage --check` SHALL fail and name the survivor id and its row

#### Scenario: Listed stale survivor
- **WHEN** a ledger entry fails the staleness or anchor check and its id is in `[triage] stale_survivors`
- **THEN** `triage --check` SHALL NOT fail because of it, and SHALL print it in the report with its symbol and the failed condition

#### Scenario: Stale entry
- **WHEN** a row listed in `binding_gaps` is bound, unbound by design, or names no row; or an id listed in `orphaned_survivors` is not in the ledger or its row is bound; or an id listed in `stale_survivors` is not in the ledger or passes the staleness and anchor checks
- **THEN** `triage --check` SHALL fail and name the stale entry

#### Scenario: Unknown triage key
- **WHEN** `[triage]` contains a key other than `binding_gaps`, `orphaned_survivors` or `stale_survivors`
- **THEN** manifest validation SHALL fail

#### Scenario: Missing stale list key
- **WHEN** `[triage]` has no `stale_survivors` key
- **THEN** manifest validation SHALL fail, so deleting the key cannot reopen its seeding in a later change

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
`triage --check` SHALL read the base commit's `formal/manifest.toml` and fail on any entry of `[correspondence] unresolved_elements`, `[triage] binding_gaps`, `[triage] orphaned_survivors`, or `[triage] stale_survivors` that the base does not list. The base SHALL be `--base`, else a non-empty `INVOWK_TRIAGE_BASE`, else the merge-base of `HEAD` and `origin/main`. A base that cannot be determined or read SHALL fail closed with a message naming the base, `INVOWK_TRIAGE_BASE`, and `--base`.

#### Scenario: Entry absent at the base
- **WHEN** the tree lists an entry in one of the four lists that the base manifest does not list
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
The growth check SHALL treat a base manifest that has none of the first three lists (`unresolved_elements`, `binding_gaps`, `orphaned_survivors`) as the seeding change for all four: it SHALL pass and print one seeding notice per list. A base that has some but not all of those three SHALL fail for each missing list, so renaming a key cannot reopen seeding. `stale_survivors` SHALL be seeded from a base that has all three but lacks it, and SHALL fail against a base that lacks it while having only some of the three.

#### Scenario: Base that introduces no list
- **WHEN** the base manifest is readable and has none of the first three lists, as for the change that introduces them
- **THEN** the growth check SHALL pass and print a seeding notice for each list

#### Scenario: Base with only some lists
- **WHEN** the base manifest has one or two of the first three lists
- **THEN** the growth check SHALL fail and name each list the base lacks

#### Scenario: Base before the stale list
- **WHEN** the base manifest has the first three lists and no `stale_survivors`, as for the change that introduces it
- **THEN** the growth check SHALL pass for `stale_survivors` with a seeding notice, and SHALL check the first three lists for growth as usual

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
The `[triage]` lists `binding_gaps` and `orphaned_survivors` SHALL be seeded once by this change: 19 binding gaps and 20 orphaned survivors (`Auth` 8, `unpackRejectsEscape` 12). Later changes SHALL NOT add entries. The survivor-triage follow-up SHALL empty `orphaned_survivors`. Listed entries SHALL be printed as a report, and the ledger's `[[survivor]]` entries SHALL NOT change to seed the lists. `stale_survivors` follows its own requirement, "The stale survivors list only shrinks".

#### Scenario: Seeded list passes
- **WHEN** `triage --check` runs on the tree that seeds the lists
- **THEN** it SHALL pass, report 19 listed gaps, 20 listed orphans, and the derived by-design rows, and need no `[[survivor]]` edit

#### Scenario: Removing a listed entry fails
- **WHEN** one listed entry that still applies is removed from `[triage]`
- **THEN** `triage --check` SHALL fail

### Requirement: The stale survivors list only shrinks
`[triage] stale_survivors` SHALL list only baselined survivors that fail the staleness or anchor check and that the change that seeds the list does not refresh. Later changes SHALL NOT add entries. An id leaving the list SHALL be explained: it passes the check, or it left the ledger.

#### Scenario: Seeded from the refresh run
- **WHEN** the change that introduces the list lands
- **THEN** the list SHALL hold exactly the baselined ids, outside `standardVirtualAnchorsForOS` and outside `orphaned_survivors`, that the fresh report-only run could not anchor or that fail the staleness check, and SHALL be empty if there are none

#### Scenario: Stale survivor fixed since the base
- **WHEN** the base lists a stale survivor whose ledger entry now passes the staleness and anchor checks, or whose id left the ledger
- **THEN** `triage --check` SHALL pass and print a notice naming the id and the reason it left

### Requirement: Partial refresh of formal-bindings survivors
A committed partial-refresh command SHALL rewrite the formal-bindings baseline and triage ledger entries of chosen symbols from a report-only formal-bindings run. A symbol is chosen by name (optionally qualified by its file), or because one of its ledger entries fails the staleness or anchor check. The command SHALL refuse a chosen symbol that still exists but that the formal-bindings plan does not target.

#### Scenario: Symbols chosen by name
- **WHEN** the refresh runs with one or more symbol names
- **THEN** it SHALL refresh exactly those symbols, and SHALL fail without writing, with a message naming the symbol, when a name matches planned functions in several files without a file qualifier, or matches no planned function and has no ledger entries in a file that no longer declares it

#### Scenario: Symbols chosen by staleness
- **WHEN** the refresh runs in stale mode
- **THEN** it SHALL refresh every symbol that has a ledger entry failing the staleness or anchor check of the triage check, including entries listed in `[triage] stale_survivors`, and SHALL NOT choose orphaned survivors' symbols

#### Scenario: Unplanned symbol
- **WHEN** a chosen symbol still names a function in its file but is not a target of the formal-bindings plan, such as an orphaned survivor's symbol
- **THEN** the refresh SHALL fail without writing any file

#### Scenario: Deleted or renamed function
- **WHEN** a symbol chosen by name or by staleness has ledger entries, matches no planned function, and no longer names a function in its file
- **THEN** the refresh SHALL remove all of its entries, add none, and say so in the summary

#### Scenario: Row still names the deleted function
- **WHEN** a correspondence row still names a function that no longer exists, so the formal-bindings plan cannot be built
- **THEN** the refresh SHALL fail without writing and say to update the row first

### Requirement: The refresh attributes mutants to plan functions
Each reported mutant SHALL belong to the plan function whose line range contains its original start line. A symbol's escaped ids SHALL be the ids with at least one escaped mutant attributed to it. Baseline entries SHALL belong to a symbol only through the ledger entry of the same id.

#### Scenario: Mutant outside every plan function
- **WHEN** the run reports a mutant whose line lies in no plan function of its file
- **THEN** the refresh SHALL count and list it as unattributed and SHALL NOT add it

#### Scenario: Id both killed and escaped
- **WHEN** the run reports one mutant of a chosen symbol's baselined id as killed and another mutant with the same id in that symbol as escaped
- **THEN** the refresh SHALL count the id as escaped in that symbol and keep the entry

#### Scenario: Id shared with a symbol that was not chosen
- **WHEN** an escaped id of a chosen symbol is ledgered under a symbol that was not chosen, as when the same line is mutated the same way in two functions of one file
- **THEN** the refresh SHALL leave both its entries unchanged, report it as shared with its owner, and SHALL NOT fail

### Requirement: The refresh removes survivors the run no longer reports as escaped
For each chosen symbol, the refresh SHALL remove every ledger entry of that symbol, and the baseline entry of the same id, whose id the run reports escaped in no mutant: an id the run no longer generates, or one it reports only killed, skipped or errored. It SHALL remove every id of a chosen symbol from `[triage] stale_survivors`.

#### Scenario: Removed id is dropped
- **WHEN** a chosen symbol's ledger entry names an id the run does not generate
- **THEN** the refresh SHALL delete that id from the baseline, the ledger and `[triage] stale_survivors`, and SHALL count it as removed

#### Scenario: Baselined id now killed
- **WHEN** the run reports a chosen symbol's baselined id as killed
- **THEN** the refresh SHALL remove it as no longer a survivor and SHALL say it was killed

### Requirement: The refresh moves ids between chosen symbols only
An entry of a chosen symbol whose id escapes only in other symbols SHALL move to the chosen one of them as a new untriaged entry. When the id escapes only in symbols that were not chosen, the refresh SHALL fail without writing and name them.

#### Scenario: Id moves to another chosen symbol
- **WHEN** a chosen symbol's baselined id escapes only in another chosen symbol
- **THEN** the refresh SHALL remove it from the first symbol and add it to the second as an untriaged entry

#### Scenario: Id moves to a symbol that was not chosen
- **WHEN** a chosen symbol's baselined id escapes only in symbols that were not chosen
- **THEN** the refresh SHALL fail without writing any file and name those symbols

### Requirement: The refresh adds new escaped mutants as untriaged survivors
For each chosen symbol, the refresh SHALL add every id the run reports as escaped in that symbol and the baseline does not list. Each new baseline entry SHALL carry go-mutesting's fields from the run and its own evidence `digest`; each new ledger entry SHALL carry its anchor derived from the run's diff and an untriaged placeholder for its class and reason. Ids the run reports with any other status SHALL NOT be added, and an id escaped in several chosen symbols SHALL be added once.

#### Scenario: New escapee is added
- **WHEN** the run reports an escaped mutant in a chosen symbol whose id is not baselined
- **THEN** the refresh SHALL add one baseline entry for the id, with its file, mutator, line and the run's plan-inputs digest as its own `digest`, and one ledger entry whose anchor hashes to the id

#### Scenario: Killed id is not added
- **WHEN** the run reports a mutant in a chosen symbol as killed, skipped or errored and its id is not baselined
- **THEN** the refresh SHALL NOT add it

#### Scenario: Escapees of other symbols are ignored
- **WHEN** the run reports an escaped, unbaselined mutant in a symbol that was not chosen
- **THEN** the refresh SHALL NOT add it

#### Scenario: Placement of a new entry
- **WHEN** the refresh adds a ledger entry
- **THEN** it SHALL insert it after the last ledger entry of the same symbol, else of the same file, else before the first closed entry, and SHALL insert the baseline entry at its sort position

### Requirement: Untriaged survivors fail the triage check
The triage check SHALL fail on every ledger entry that carries the refresh's untriaged placeholder, in its class or in its reason, with a message that names the id and symbol and says to write the class and reason. The placeholder SHALL NOT be a valid class.

#### Scenario: Placeholder after a refresh
- **WHEN** the triage check runs on a ledger to which the refresh added an entry
- **THEN** it SHALL fail for that entry with the untriaged message

#### Scenario: Class written but reason left
- **WHEN** a person sets a valid class on a refreshed entry but leaves the placeholder reason
- **THEN** the triage check SHALL still fail for that entry

#### Scenario: Triaged entry passes
- **WHEN** a refreshed entry has a valid class, a written reason, the follow-up its class needs, and two recorded escaped reruns at its own digest
- **THEN** the triage check SHALL raise no failure for that entry

### Requirement: The refresh leaves other entries byte-identical
The refresh SHALL leave every baseline entry, ledger entry, ledger comment and `[triage]` entry that it does not remove, add or re-anchor byte-identical, including the baseline entries it keeps in the chosen symbols and the baseline's top-level digest.

#### Scenario: Unrelated entries unchanged
- **WHEN** the refresh adds and removes entries of one symbol
- **THEN** the diff of the baseline, the ledger and `formal/manifest.toml` SHALL contain only the added, removed and re-anchored entries of that symbol and its ids' `stale_survivors` lines

#### Scenario: Kept survivor unchanged
- **WHEN** a chosen symbol's baselined id is reported escaped again and its ledger entry passes the anchor check
- **THEN** the refresh SHALL keep its baseline and ledger entries unchanged and count it as kept

#### Scenario: Refresh is idempotent
- **WHEN** the refresh runs twice with the same run and the same symbols
- **THEN** the second run SHALL change no file, re-anchor nothing, and report every entry of those symbols as kept

### Requirement: Kept entries of chosen symbols pass the anchor check after a refresh
A kept ledger entry that fails the anchor check SHALL have only its anchor keys (`removed`, `added`, `context`) rewritten from the run's diff. After the refresh, every remaining entry of a chosen symbol SHALL pass the staleness and anchor checks and SHALL NOT be listed in `[triage] stale_survivors`.

#### Scenario: Kept survivor with a failing anchor
- **WHEN** a chosen symbol's baselined id is reported escaped again but its ledger entry fails the anchor check, for example because the context line of an insertion-only mutant changed, or it is listed in `[triage] stale_survivors`
- **THEN** the refresh SHALL rewrite only that entry's anchor keys from the run's diff, keep its class and reason, remove the id from `[triage] stale_survivors`, count it as kept and re-anchored, and the triage check SHALL raise no staleness or anchor failure for it

### Requirement: The refresh fails closed before writing
Before writing, the refresh SHALL check that the baseline round-trips through its serializer, that the new ledger parses back to the old entries plus the added and minus the removed ones with only re-anchored entries' anchor keys changed, and that every remaining entry of a chosen symbol passes the anchor check. When any check fails, it SHALL write no file.

#### Scenario: Parse-back failure
- **WHEN** the new ledger text would not parse back to the expected entries
- **THEN** the refresh SHALL fail without writing any file

#### Scenario: Closed gap escapes again
- **WHEN** an id the refresh would add is recorded as a closed gap in the ledger
- **THEN** the refresh SHALL fail without writing any file, naming the id and its model

#### Scenario: Malformed rerun records
- **WHEN** the rerun evidence file has a malformed record
- **THEN** the refresh SHALL fail before writing any file

#### Scenario: Anchor still failing
- **WHEN** a remaining entry of a chosen symbol would still fail the anchor check after the refresh
- **THEN** the refresh SHALL fail without writing any file and name the entry

### Requirement: The refresh uses a complete run of the current inputs
A full formal-bindings run SHALL record its plan-inputs digest and each match group's exit status with its reports. The refresh SHALL fail without writing any file, naming the run command to repeat, when the run has no recorded digest, when the digest of the current tree's plan inputs differs from it, or when the run is incomplete: a plan group has no report, no recorded status, or a status other than success or the escaped-mutants quality-gate status.

#### Scenario: Run of other inputs
- **WHEN** a target file or killer-test file changed after the run
- **THEN** the refresh SHALL fail, naming both digests and `make mutation-formal`

#### Scenario: Incomplete run
- **WHEN** a match group of the run crashed, timed out or was not run, so it has no report or a failing status
- **THEN** the refresh SHALL fail without writing any file, naming the groups and `make mutation-formal`, and SHALL NOT remove any entry as not generated

#### Scenario: Run in another worktree
- **WHEN** the run was made in a throwaway worktree at a commit whose plan inputs equal the current tree's
- **THEN** the refresh SHALL accept it and resolve the reported file paths against that run's root

#### Scenario: Row changed after the run
- **WHEN** a correspondence row of a planned function was bound, unbound or edited after the run, so the plan's functions or rows differ while the inputs digest is unchanged
- **THEN** the refresh SHALL fail without writing any file, naming the differing functions and `make mutation-formal`

#### Scenario: Report path outside the run's root
- **WHEN** a reported file path is absolute and lies outside the run's root, also after resolving symlinks
- **THEN** the refresh SHALL fail without writing any file instead of treating the chosen symbols' ids as not generated

### Requirement: The refresh checks rerun evidence and reports a summary
After writing, the refresh SHALL check, for every remaining entry of the chosen symbols, the rerun evidence the triage check requires, and SHALL print the ids that lack it with the `make mutation-formal-rerun` command to run. It SHALL NOT run mutation itself. It SHALL print counts of removed, moved, added, untriaged, kept, re-anchored, shared, unattributed and awaiting-evidence entries.

#### Scenario: Evidence missing after adding
- **WHEN** the refresh adds entries that have no recorded reruns at their digest
- **THEN** it SHALL list them as awaiting evidence and print `make mutation-formal-rerun MUTATION_MUTANT_ID=<ids>` with the note that each id needs two escaped reruns

#### Scenario: Evidence complete
- **WHEN** every remaining entry of the chosen symbols has two escaped reruns at its evidence digest
- **THEN** the summary SHALL report zero awaiting evidence

#### Scenario: Make target
- **WHEN** a maintainer runs `make mutation-formal-refresh` with symbols or the stale mode
- **THEN** it SHALL run the refresh on the default run report directory, and print usage and fail when neither is given
