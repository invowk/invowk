## ADDED Requirements

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
- **THEN** it SHALL append the mutant ID, status, timestamp, and commit to `tools/mutation/triage/formal-bindings-reruns.jsonl`

#### Scenario: Survivors are confirmed by evidence
- **WHEN** the triage check runs
- **THEN** it SHALL fail for any baselined ID without at least two recorded `escaped` reruns

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
