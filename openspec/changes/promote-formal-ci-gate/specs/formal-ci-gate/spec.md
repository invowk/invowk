## ADDED Requirements

### Requirement: Always-reporting, fail-closed classification
The Formal Verification workflow's `pull_request` trigger SHALL NOT use a `paths:` filter, and the formal job SHALL run and report on every pull request under an unchanged check name. A classification step SHALL run `scripts/formal.py affected`. It SHALL decide whether the heavy steps (Setup Go, Setup Java, tool fetch, model checks, trace validation, replay, deep property tests) run, using these rules:
- On any event other than `pull_request`, `affected` SHALL print `run=true` without computing a diff.
- On `pull_request`, it SHALL print `run=true` if any changed path between the base SHA and `github.sha` matches a `formal/manifest.toml` `[ci] paths` pattern under `fnmatch.fnmatchcase`, and `run=false` otherwise.
- `affected` SHALL be dispatched before manifest validation, SHALL always exit 0, and SHALL print `run=true` on any error.
- Heavy steps SHALL be gated on `steps.affected.outputs.run != 'false'`, so an empty output runs the full lane.

Checkout SHALL stay shallow, and the base SHA SHALL be fetched with `git fetch --depth=1`.

#### Scenario: Relevant pull request
- **WHEN** a pull request changes a path matched by `[ci] paths`
- **THEN** the workflow SHALL run the full lane within the step and job `timeout-minutes` budgets

#### Scenario: Unrelated pull request still reports
- **WHEN** a pull request changes no path matched by `[ci] paths`
- **THEN** the formal job SHALL complete with success, skip Setup Go, Setup Java, and every formal step, and report under the same check name

#### Scenario: Scheduled run always runs the full lane
- **WHEN** the event is `schedule` or `workflow_dispatch`
- **THEN** `affected` SHALL print `run=true` without computing a diff, and `Check models`, `Trace validation`, the replay step, and `Deep property tests` SHALL execute

#### Scenario: Classification failure
- **WHEN** the base SHA cannot be fetched or the diff cannot be computed
- **THEN** `affected` SHALL print `run=true` and exit 0

#### Scenario: Malformed manifest
- **WHEN** `formal/manifest.toml` cannot be parsed or fails validation
- **THEN** `affected` SHALL print `run=true` and exit 0, and the later `make formal` step SHALL fail

#### Scenario: Path list covers every modelled file
- **WHEN** `scripts/test_formal.py` runs
- **THEN** it SHALL fail unless `[ci] paths` matches all of these:
  - every file named in any correspondence table;
  - the directory of every binding-test file;
  - every `[[trace]]` package, normalised from `./dir/` to `dir/**`;
  - every golden output.

### Requirement: Generated replay plan
The workflow's golden and property replay step SHALL take its package list and its `-run` pattern from `scripts/formal.py replay-plan`, not from a hand-maintained list. The plan SHALL contain every binding test named in any correspondence table's binding cell, excluding `Test*_TraceHarness` functions, which trace validation runs. Its pattern SHALL be anchored as `^(TestA|TestB|...)$`, and its package list SHALL be the set of packages that declare those tests.

#### Scenario: Every binding test is replayed
- **WHEN** `scripts/test_formal.py` runs
- **THEN** it SHALL fail if a binding test named in any correspondence table is not matched by the plan's pattern, or if its package is not in the plan's package list

#### Scenario: Unknown binding test
- **WHEN** a binding cell names a test that no `_test.go` file declares
- **THEN** `replay-plan` SHALL exit non-zero and name the model and row

### Requirement: CI time budgets
Each heavy formal step SHALL have a hard budget, set as a step-level `timeout-minutes`: model checks, trace validation, replay, and deep property tests. Each hard budget SHALL be about four times the maximum measured CI time for the final model set, rounded up to whole minutes. The job-level `timeout-minutes` SHALL exceed the sum of the step `timeout-minutes` values plus a setup margin of at least 3 minutes, so a step timeout always fires before the job timeout.

Every `[[model]]` and `[[trace]]` entry SHALL carry a soft budget `budget_seconds` and a `budget_source` of `ci` or `local`. The runner SHALL measure the wall time of every model and trace suite. A soft-budget overrun SHALL warn and SHALL NOT change the exit status.

#### Scenario: Soft budget exceeded
- **WHEN** a model's measured wall time exceeds its `budget_seconds`
- **THEN** the runner SHALL print a `WARNING` naming the model, the measured time, and the budget
- **THEN** under GitHub Actions it SHALL also emit a `::warning::` annotation, and the exit status SHALL be unchanged

#### Scenario: Hard budget exceeded
- **WHEN** a formal step runs longer than its step `timeout-minutes`
- **THEN** GitHub Actions SHALL fail the step and the job
- **THEN** the upload step, conditioned on `failure() || cancelled()`, SHALL upload `artifacts/formal/`

#### Scenario: Job timeout covers the steps
- **WHEN** `scripts/test_formal.py` runs
- **THEN** it SHALL fail if the formal job's `timeout-minutes` is not greater than the sum of its step `timeout-minutes` plus 3

#### Scenario: Missing budget
- **WHEN** a `[[model]]` or `[[trace]]` entry has no `budget_seconds` or no valid `budget_source`
- **THEN** `validate_manifest` SHALL fail, naming the entry

#### Scenario: Provisional budget in CI
- **WHEN** the runner checks a model whose `budget_source` is `local` under GitHub Actions
- **THEN** it SHALL emit a `::notice::` stating that the budget is provisional and naming the model

#### Scenario: Timing is observable
- **WHEN** the runner checks a model under GitHub Actions
- **THEN** it SHALL flush each result line as the command completes
- **THEN** it SHALL print one timing line per model and trace suite, print TLC distinct-state counts, and append a timing table to `$GITHUB_STEP_SUMMARY`

### Requirement: Recorded TLC state counts
Every TLC command that expects `pass` SHALL record `distinct_states` in `formal/manifest.toml`. TLC runs with `-workers 1`, so the count is deterministic and SHALL be recorded from a local run. Drift from the recorded count continues to warn, as the existing drift check does.

#### Scenario: Missing state count
- **WHEN** a TLC command expects `pass` and has no `distinct_states`
- **THEN** `validate_manifest` SHALL fail, naming the model and command

### Requirement: Isolated concurrency for scheduled runs
Only `pull_request` runs SHALL share a concurrency group and cancel each other. Every `schedule` and `workflow_dispatch` run SHALL use a concurrency group unique to that run.

#### Scenario: Scheduled run is not cancelled by a dispatch
- **WHEN** a `workflow_dispatch` run starts on `main` while a scheduled run is in progress
- **THEN** the scheduled run SHALL NOT be cancelled

#### Scenario: A queued scheduled run is not cancelled by a later dispatch
- **WHEN** a scheduled run is queued while a dispatch run is in progress, and a second dispatch is triggered
- **THEN** the queued scheduled run SHALL NOT be cancelled

### Requirement: Checkable promotion gate
Invowk SHALL provide `make formal-promotion-gate`, backed by `scripts/formal_promotion_gate.py` (Python standard library only). The script SHALL query GitHub Actions run history for `formal-verification.yml` and SHALL exit 0 only when all of these hold:
- the four most recent completed runs with event `schedule` on branch `main`, considering only runs of the current pipeline (see below), all concluded `success` on `run_attempt` 1;
- in each of those runs, the job named `Models, golden vectors, and correspondence` concluded `success`;
- in each of those runs, the steps `Classify changed paths`, `Check models`, `Trace validation`, `Replay golden vectors and property tests`, and `Deep property tests` each concluded `success` and not `skipped`;
- consecutive counted runs are at most 8 days apart;
- the newest counted run is at most 8 days old at evaluation time.

A run of the current pipeline is one whose formal job contains the `Classify changed paths` step. This step exists only from this change onward, so runs made before this change merged cannot count. Runs created before an optional `--since` date SHALL also be excluded.

The script SHALL send only GET requests to the GitHub API. It SHALL never modify repository settings or rulesets. Making the check required SHALL remain a manual maintainer action. The decision logic SHALL be a pure function tested by `scripts/test_formal_promotion_gate.py` under `make test-scripts`.

#### Scenario: Gate holds
- **WHEN** the four most recent current-pipeline scheduled runs on `main` are first-attempt successes, every required step succeeded, the runs are one week apart, and the newest is two days old
- **THEN** the script SHALL print PASS, a table of the four runs (ID, date, head SHA, conclusion), and the exact check context to require, and SHALL exit 0

#### Scenario: A rerun hides a flake
- **WHEN** one of the four runs succeeded only on `run_attempt` 2
- **THEN** the script SHALL print FAIL, naming that run, and exit 1

#### Scenario: Vacuous green run
- **WHEN** a counted run concluded `success`, but one of the required steps is `skipped` or missing
- **THEN** the script SHALL print FAIL, naming the run and the step, and exit 1

#### Scenario: Runs predating the gate's workflow are not counted
- **WHEN** scheduled runs exist whose formal job has no `Classify changed paths` step, or that were created before `--since`
- **THEN** the script SHALL exclude them from the count, and SHALL report how many it excluded

#### Scenario: Missed week
- **WHEN** two consecutive counted runs are more than 8 days apart
- **THEN** the script SHALL print FAIL, naming the gap, and exit 1

#### Scenario: Too few runs
- **WHEN** fewer than four counted runs exist
- **THEN** the script SHALL print FAIL with the count found, and exit 1

#### Scenario: Stale history
- **WHEN** the newest counted run is more than 8 days old
- **THEN** the script SHALL print FAIL, because the schedule may have been disabled, and exit 1

#### Scenario: API unavailable
- **WHEN** a GitHub API request fails or returns an unexpected payload
- **THEN** the script SHALL print FAIL with the error and exit 1, never PASS

#### Scenario: Unrelated check runs in the run
- **WHEN** a run's jobs list contains check runs from other workflows, such as `JUnit Report (...)`
- **THEN** the script SHALL evaluate only the formal job, matched by its exact name

#### Scenario: Write requests are refused
- **WHEN** the script's request helper is called with any method other than GET
- **THEN** it SHALL raise an error without sending the request
