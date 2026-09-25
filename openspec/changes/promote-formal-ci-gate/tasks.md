## 0. Preconditions

- [ ] 0.1 Confirm that all five sibling changes are merged to `main`: `formal-infra-refinements`, `trace-validate-token-and-lock`, `model-concurrent-lock-writes`, `model-module-path-containment`, and `mutation-test-formal-bindings`. Rebase onto `main`
- [ ] 0.2 Take a local baseline: time `make formal`, `make formal-traces`, the replay command, and `make formal-rapid-deep`, and record the `[[model.command]]` count

## 1. Runner

- [ ] 1.1 In `scripts/formal.py`, flush result lines as they are produced. Time each Alloy model, each TLC model (the sum of its commands' checker wall times), and each trace suite, and print `timing` and TLC `states` lines
- [ ] 1.2 Add `budget_seconds`, `budget_source` (`ci` or `local`), `[ci] paths`, and `[ci.budget]` to the manifest loader and to the known-key set from `formal-infra-refinements`. Make `validate_manifest` fail on a missing budget or source, and on a passing TLC command with no `distinct_states`
- [ ] 1.3 Warn on a soft-budget overrun (`WARNING`, plus `::warning::` when `GITHUB_ACTIONS=true`) without changing the exit status. Emit `::notice::` for `local` budgets in CI, and append a timing table to `$GITHUB_STEP_SUMMARY` when it is set
- [ ] 1.4 Add `affected --event --base --head`, dispatched before manifest validation. It prints `run=true` on non-PR events and on any exception, matches paths with `fnmatch.fnmatchcase`, and always exits 0. Move the workflow path list into `[ci] paths`, and add `internal/testutil/**`, `go.mod`, `go.sum`, `Makefile`, and the siblings' new packages
- [ ] 1.5 Add `replay-plan`. It parses comma-separated binding cells (sharing the parser from `mutation-test-formal-bindings`), drops `Test*_TraceHarness`, resolves packages, prints `packages=` and an anchored `run=`, and fails on an unknown test
- [ ] 1.6 Extend `scripts/test_formal.py` with these cases:
  - `affected`: missing budget or source fails; a missing `distinct_states` fails; an overrun warns but passes; a match and a miss; `schedule` and `workflow_dispatch` return `run=true` without git; a bad SHA and a malformed manifest both return `run=true` with exit 0;
  - coverage: correspondence files, binding-test directories, normalised trace packages, and golden outputs are all matched by `[ci] paths`;
  - replay plan: covers every binding test, including the five that are missing today;
  - workflow invariant: job `timeout-minutes` > Σ step `timeout-minutes` + 3
- [ ] 1.7 Record `distinct_states` for every passing TLC command that `formal-infra-refinements` has not already recorded, from a local run. Seed `budget_seconds` with `budget_source = "local"` (`ceil(3 × local)`) so the manifest validates

## 2. Workflow

- [ ] 2.1 Remove the `pull_request` `paths:` filter. Add the `Classify changed paths` step (id `affected`) with the shallow `git fetch --depth=1 origin "$BASE_SHA"` and the `|| echo run=true` fallback. Gate Setup Go, Setup Java, the jar cache, fetch, the self-tests, and every formal step on `steps.affected.outputs.run != 'false'`, keeping the job name unchanged
- [ ] 2.2 Drive the replay step from `formal.py replay-plan --format github`
- [ ] 2.3 Set `PYTHONUNBUFFERED: "1"` in job `env:`. Change the upload step to `if: failure() || cancelled()`. Keep the job `timeout-minutes` at 20 until task 2.5
- [ ] 2.4 Set the concurrency to `group: formal-verification-${{ github.event_name == 'pull_request' && github.ref || github.run_id }}` and `cancel-in-progress: ${{ github.event_name == 'pull_request' }}`
- [ ] 2.5 After merge, run at least three `workflow_dispatch` runs on `main`. From their timings:
  - set step `timeout-minutes = max(2, ceil(4 × max / 60))` for Check models, Trace validation, Replay, and Deep property tests;
  - set the job `timeout-minutes` above Σ steps + 3;
  - set every `budget_seconds = ceil(1.5 × max)` with `budget_source = "ci"`;
  - record the run IDs, the date, the headroom factors, and the command count in `[ci.budget]`.

  Land the result as a follow-up PR, which must itself pass the lane

## 3. Promotion gate

- [ ] 3.1 Write `scripts/formal_promotion_gate.py` with an SPDX header. It has a GET-only request helper, the token chain from design D6, and a pure `evaluate(runs, jobs_by_run, now, since)` that checks: first attempt, exact job name, the required steps are `success` and not `skipped`, the `Classify changed paths` step is present, the gap is at most 8 days, and the newest run is at most 8 days old. It also supports text and `--json` output and the `--repo`, `--since`, and `--now` flags
- [ ] 3.2 Write `scripts/test_formal_promotion_gate.py` with these cases:
  - pass;
  - second-attempt success;
  - a vacuous run with skipped steps;
  - a pre-change run without the classify step, excluded and counted;
  - a `--since` exclusion;
  - a missed week, fewer than four runs, and stale history;
  - an API error and a malformed payload;
  - a foreign `JUnit Report` job;
  - refusal of any non-GET method.

  Register it in `make test-scripts`
- [ ] 3.3 Add a `make formal-promotion-gate` target and its `make help` entry. Fix the stale `formal-traces … (phase 3; currently a no-op)` help line. Run the gate once and confirm the expected FAIL: no current-pipeline scheduled runs, with the pre-change runs reported as excluded

## 4. Documentation and governance

- [ ] 4.1 Add a "CI budgets" section and a "Promoting the lane" section (design D7: gate, manual ruleset step, rollback) to `formal/README.md`. Update the workflow header comment to cite `make formal-promotion-gate`
- [ ] 4.2 Update `.agents/skills/formal-verification/SKILL.md`:
  - new models need `budget_seconds` with `budget_source = "local"` until CI measures them, `distinct_states` on passing TLC commands, and `[ci] paths` entries for new modelled files;
  - the replay plan comes from the binding cells.
- [ ] 4.3 Update `.agents/rules/commands.md`:
  - the workflow table row reads "Weekly schedule, manual dispatch, and every PR (classified by `formal.py affected` against `[ci] paths`)";
  - its promotion wording cites the gate;
  - add `make formal-promotion-gate` to the Quick Reference table.

  Then run `make check-agent-docs`
- [ ] 4.4 In `openspec/changes/adopt-formal-verification/tasks.md`, tick 8.3 after task 2.5, citing the run IDs and date. Reword 8.5 to "**Phase 3 gate:** trace mutations are rejected as declared, and `make formal-promotion-gate` reports PASS (lane stays non-required until then)"

## 5. Verification

- [ ] 5.1 Run `make test-scripts`, `python3 -m py_compile scripts/formal.py scripts/formal_promotion_gate.py`, `make license-check`, `make formal`, and `make formal-traces` locally. No Python linter is configured in the repository, and `make lint-scripts` covers shell scripts only
- [ ] 5.2 Confirm on the PR itself that the formal check runs the full lane and reports success. Confirm with `affected` against a docs-only SHA range, or with a docs-only test PR, that Setup Go and every formal step are skipped and the check still reports success
- [ ] 5.3 Run `make lint` and `make test`, and record that Go code is unchanged
- [ ] 5.4 Run `/learn`
