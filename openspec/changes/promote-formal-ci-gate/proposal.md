## Why

The Formal Verification lane (`.github/workflows/formal-verification.yml`) is still advisory. Task 8.3 of `adopt-formal-verification` (measure CI wall time, record budgets and state counts) is open, and the promotion rule ("four consecutive green weekly runs") exists only as prose that nobody can check mechanically. As configured today, the lane also cannot safely become a required check:
- its `pull_request` trigger is path-filtered, so a required check would never report on unrelated PRs and would block them;
- its replay step's hard-coded `-run` pattern misses five binding tests that the correspondence tables name.

The five fix PRs (#146–#150) are merged, and the model set is about to grow through the sibling formal changes. After those land, this change turns the lane into a gate that can be verified.

## Dependencies

This change lands **last** in the formal series. All five sibling changes must be merged before its implementation starts, because its budgets, replay plan, and path list must cover the final model and trace set:
- `formal-infra-refinements`: manifest key validation, golden format 2, generated Alloy commands, Serverbase `distinct_states`;
- `trace-validate-token-and-lock`: `HostCallbackToken` and `LockIntegrity` trace suites;
- `model-concurrent-lock-writes`: `ConcurrentModuleEdits` TLA+ model;
- `model-module-path-containment`: two new Alloy models with golden vectors;
- `mutation-test-formal-bindings`: comma-separated binding cells and the binding-test completeness check.

## What Changes

- Record measured CI step times as explicit budgets.
  - The 2026-09-25 numbers in design.md are a **pre-sibling baseline** only.
  - Final budgets come from a re-measurement taken after the siblings land: one local run plus CI `workflow_dispatch` runs.
  - Hard budgets: step-level `timeout-minutes` on the four formal steps, each about 4× its measured maximum, rounded up to whole minutes. The job `timeout-minutes` must stay above the sum of the step timeouts plus setup time.
  - Soft budgets: per-model and per-trace-suite `budget_seconds` in `formal/manifest.toml`. An overrun warns and never fails the run. A budget measured only locally is marked provisional (`budget_source = "local"`).
- Make runner output measurable. `scripts/formal.py` flushes each result line as it completes, prints per-model and per-suite timings and TLC distinct-state counts, and writes a step-summary table. Every TLC command that expects `pass` must record `distinct_states`. TLC runs with `-workers 1`, so the count is deterministic and can be recorded locally.
- **BREAKING (CI behaviour)**: the `pull_request` trigger loses its `paths:` filter, so the job always runs and always reports. A new `scripts/formal.py affected` step decides whether the heavy steps run. It fails closed: on any non-PR event, and on any error (including a malformed manifest), it answers `run=true`. The path list moves to `formal/manifest.toml` `[ci] paths`, and a self-test ties it to every correspondence file, binding-test directory, trace package, and golden output.
- New `scripts/formal.py replay-plan` subcommand. It derives the replay step's package list and its anchored `-run` pattern from the correspondence tables' binding cells, replacing the hand-maintained regex that currently misses five binding tests.
- Concurrency: only pull-request runs share a cancellable concurrency group. Scheduled and dispatch runs each get their own group, so neither an in-progress nor a queued scheduled run can be cancelled.
- New `scripts/formal_promotion_gate.py` and a `make formal-promotion-gate` target. The script reads workflow history through the GitHub REST API with GET requests only. It passes only when all of these hold:
  - the four most recent scheduled runs on `main` succeeded on their first attempt;
  - each run ran the pipeline this change introduces;
  - in each run, every heavy formal step concluded `success`, not `skipped`;
  - no weekly slot was missed, and the newest run is at most 8 days old.
- Document the maintainer-only promotion procedure in `formal/README.md`: add the job's check context to the "Safety" ruleset. The change does not perform this step.
- Close task 8.3 of `adopt-formal-verification`, and reword 8.5 so it keeps its trace-mutation condition and adds the gate.

## Capabilities

### New Capabilities

- `formal-ci-gate`: an always-reporting, fail-closed CI classification step for the formal lane; a generated replay plan; hard and soft CI time budgets; recorded state counts; and a checkable, read-only promotion gate.

### Modified Capabilities

None. The existing "CI lane and promotion" requirement in `formal-verification-toolchain`, which is still unarchived inside `adopt-formal-verification`, stays true: the full lane still runs on relevant PRs, and promotion still needs four green weekly runs. `formal-ci-gate` adds stricter, checkable requirements on top of it. Adding a new capability avoids making this change's archive wait on `adopt-formal-verification`, which itself waits at least four weeks for this gate (design D8).

## Impact

- Files:
  - `.github/workflows/formal-verification.yml`;
  - `scripts/formal.py` and `scripts/test_formal.py`;
  - the new `scripts/formal_promotion_gate.py` and `scripts/test_formal_promotion_gate.py`;
  - `formal/manifest.toml` and `formal/README.md`;
  - `Makefile`: the `formal-promotion-gate` target, `test-scripts`, and the `help` text, including the stale `formal-traces` line;
  - `.agents/skills/formal-verification/SKILL.md` and `.agents/rules/commands.md`;
  - `openspec/changes/adopt-formal-verification/tasks.md`.
- No Go code, no new Go or npm dependencies, and no new GitHub Actions. The gate script uses the Python standard library only, and the `gh` CLI only as an optional source for a token.
- CI cost: every PR now starts the formal job. Irrelevant PRs skip Go and Java setup and every formal step, and finish in seconds. Relevant PRs cost the same as today.
- Repository settings: none are changed by this change. Making the check required stays a manual action for the maintainer.
