## Context

`formal-verification.yml` has one job, `Models, golden vectors, and correspondence`. It runs weekly (`cron: "17 6 * * 1"`), on dispatch, and on path-filtered pull requests. The weekly schedule was merged in #144, and as of 2026-09-25 no `schedule` or `workflow_dispatch` run exists. Of the 11 runs so far, all are `pull_request` runs, 8 green.

The "Safety" ruleset (id 12472716, `~DEFAULT_BRANCH`) requires one check today: `goplint consumer gates`, which uses `strict_required_status_checks_policy: false`. `lint.yml` sets the precedent: a required check's `pull_request` trigger has no path filter; the job fetches the base SHA shallowly and classifies the diff itself; and any non-PR event defaults to the full profile.

This change lands last, after `formal-infra-refinements`, `trace-validate-token-and-lock`, `model-concurrent-lock-writes`, `model-module-path-containment`, and `mutation-test-formal-bindings`. Together they add:
- two TLC trace suites, `HostCallbackToken` and `LockIntegrity`, which use real SSH and git fixtures and one TLC JVM per trace;
- the `ConcurrentModuleEdits` TLA+ model;
- two Alloy models with golden enumeration;
- golden format 2, Alloy commands generated from the manifest, and a runner that rejects unknown manifest keys. The new keys in this change (`budget_seconds`, `budget_source`, `[ci]`) must therefore be added to its allowed-key set.

### Pre-sibling baseline (2026-09-25, not the budget source)

Green `pull_request` runs, attempt shown. Times are in seconds, from the jobs API (1 s resolution).

| Run | Setup Go | Check models | Trace validation | Replay | Job |
|---|---|---|---|---|---|
| 36153682688 #1 | 16 | 36 | 67 | 17 | 2m29s |
| 36145671143 #2 | 13 | 29 | 42 | 13 | 1m50s |
| 36145252097 #1 | 15 | 36 | 53 | 17 | 2m16s |
| 36142811158 #2 | 16 | 39 | 56 | 18 | 2m20s |
| 36142492103 #1 | 14 | 27 | 39 | 13 | 1m47s |
| 36106509149 #1 | 16 | 39 | 56 | 17 | 2m24s |
| 36105083723 #1 | 16 | 38 | 56 | 18 | 2m19s |
| 36103895114 #1 | 16 | 32 | 43 | 13 | 1m55s |

- The other steps take 0–4 s each.
- `Deep property tests` was skipped on every run. Locally it takes 3.1 s.
- This baseline covers 8 models and 3 trace suites, with 97 `[[model.command]]` entries. The final set is larger, so the baseline only shows orders of magnitude.
- Per-model times cannot be recovered from these logs. The runner's stdout is block-buffered when piped, so every result line in a step carries the same timestamp. No manifest command records `distinct_states` today.
- The replay step's hard-coded `-run` regex misses five binding tests that the tables name: `TestSyncFreshCacheRejectsChangedContent`, `TestSyncRejectsRepointedTag`, `TestSyncExistingCacheRejectsTamperedContent`, `TestRevokeTokenClosesAuthenticatedConnection`, and `TestRevocationRacesAuthenticationSafely`. They run in `ci.yml`'s `make test`, but not in this lane.

## Goals / Non-Goals

**Goals:**
- Set hard and soft CI time budgets for the final model set, backed by recorded measurements, that fail or warn automatically.
- Make the lane safe to require: it reports on every PR, fails closed, replays every binding test, and cannot have a scheduled run cancelled.
- Make the promotion rule a command that gives the same answer for everyone, and that cannot be satisfied by vacuous or pre-change runs.
- Close task 8.3 of `adopt-formal-verification`.

**Non-Goals:**
- Changing repository settings or rulesets. The maintainer does that by hand.
- Changing any model, golden vector, or Go test.
- Speeding up the lane.
- Running the gate inside CI.
- Generating `FORMAL_RAPID_PACKAGES` in the Makefile. It has the same drift risk as the replay list, but the rapid-test packages are few and appear in the binding cells, so the replay plan already covers them. Left as a follow-up.

## Decisions

### D1. Two budget tiers, derived after the siblings land

- **Hard tier: step `timeout-minutes`.** Set on `Check models`, `Trace validation`, the replay step, and `Deep property tests`. Each value is `ceil(4 × max CI seconds / 60)`, with a floor of 2. The maximum comes from the post-sibling re-measurement (task 2.4): one local run plus at least three CI `workflow_dispatch` runs on `main`. Dispatch runs execute every step, deep rapid included.
- **Job timeout.** The invariant is `job timeout-minutes > Σ step timeout-minutes + 3`. The 3 minutes cover setup and upload. `test_formal.py` enforces the invariant with a small regex scan of the workflow; the standard library has no YAML parser, and the file's indentation is fixed. Until re-measurement, the job keeps today's 20 minutes.
- **Why the ordering matters.** A job-level timeout cancels the job, and then `if: failure()` does not run. The upload step therefore uses `if: failure() || cancelled()`.
- **Soft tier.** `budget_seconds = ceil(1.5 × max observed CI seconds)` on every `[[model]]` and `[[trace]]` entry, with `budget_source = "ci"`. New models, including a sibling's entry added before CI measures it, may carry `budget_source = "local"`, which is `ceil(3 × local seconds)`. The runner emits a `::notice::` for provisional budgets in CI. `[ci.budget]` records the headroom factors, the measurement date, and the run IDs.
- **Overrun policy.** A soft overrun warns (`WARNING` plus `::warning::`) and never fails the run. Noise on shared runners must not turn a required check red.
- **Budget granularity.** Budgets are per model, not per command. At implementation time, count the `[[model.command]]` entries and record the number in `[ci.budget]`; it is 97 before the siblings land. TLC commands run in parallel, so per-command wall times interfere with each other, while a model's summed checker time is stable apart from contention.
- **Rejected alternative:** failing in the runner at 2× the soft budget. It duplicates the step timeouts and turns runner variance into red required checks.

### D2. Observable timing and deterministic state counts

The runner changes its output as follows:
- it prints with `flush=True`, and `PYTHONUNBUFFERED=1` is set in the job `env:`;
- it prints `timing <model> <s>s budget <n>s (<source>)`;
- it prints `states <model>.<command> <distinct>`;
- under Actions, it appends a timing table to `$GITHUB_STEP_SUMMARY`.

`distinct_states` is **mandatory** on every TLC command that expects `pass`, and `validate_manifest` fails without it. Unlike time, the count is deterministic under `-workers 1`, so an author records it from a local run. `formal-infra-refinements` records the Serverbase counts (its task 5.4). This change fills in the rest and makes the field mandatory.

### D3. Always-reporting job and fail-closed classification

- **Trigger and checkout.** The `pull_request` trigger has no `paths:`. Checkout stays shallow, and the classification step runs `git fetch --depth=1 origin "$BASE_SHA"` and diffs `BASE_SHA..github.sha`, as `lint.yml` does. `fetch-depth: 0` is rejected because it clones the full history on every PR.
- **`formal.py affected --event E --base B --head H`:**
  - It is dispatched in `main()` **before** `load_manifest()`/`validate_manifest()`.
  - If `E != pull_request`, it prints `run=true` without a diff.
  - Otherwise it reads `[ci] paths` with `tomllib` directly and matches each changed path with `fnmatch.fnmatchcase`. `fnmatch` is not used, because it applies `normcase`.
  - Its whole body is wrapped so that any exception prints `run=true` and exits 0.
- **Workflow step.** The step, named `Classify changed paths`, adds `|| echo run=true >> "$GITHUB_OUTPUT"` as a second fail-safe.
- **Gating.** Every heavy step, Setup Go and Setup Java included, uses `if: steps.affected.outputs.run != 'false'`, so an empty output runs the full lane.
- **Pattern semantics.** In `fnmatchcase`, `*` also matches `/`. Patterns can therefore only over-match relative to GitHub `paths` globs, which fails safe.
- **`[ci] paths` contents:** today's filter list, plus `internal/testutil/**` (for `tlatrace`, `alloygolden`, and the siblings' `fstree` and `pathmatrix`), `go.mod`, `go.sum`, and `Makefile`, plus the siblings' new packages (for example `internal/runtime/**`, `internal/app/moduleops/**`).
- **Coverage self-test.** It asserts that `[ci] paths` matches:
  - every correspondence file cell;
  - the directory of every binding-test file (resolved by mapping each test name to its declaring file);
  - every `[[trace]] package`, normalised from `./x/y/` to `x/y/**`;
  - every `[model.golden] output`.

  It cannot catch code that a model depends on but no table names. That remains the author's duty.
- **Rejected alternatives:** `dorny/paths-filter`, which adds an action and a pin; and a duplicate-name "skipped" workflow, which is fragile.

### D4. Generated replay plan

`formal.py replay-plan [--format github]` builds the replay step's inputs:
- It reads every binding cell, which is comma-separated per `mutation-test-formal-bindings` D8.
- It drops `Test*_TraceHarness`, because trace validation runs those.
- It resolves each test to its declaring package through the same index that `check_correspondence` uses.
- It prints `packages=<space-separated ./dir/>` and `run=^(A|B|...)$`.

The replay step becomes `go test -count=1 -run "$RUN" $PACKAGES` with values from that step. Tests extracted from the tables must equal the plan, the five currently missing tests appear in the plan, and an unknown test fails with the model and row. Where `mutation-test-formal-bindings` adds a binding-test completeness check, the plan and that check share one binding-cell parser.

### D5. Concurrency

The concurrency setting becomes:

```yaml
group: formal-verification-${{ github.event_name == 'pull_request' && github.ref || github.run_id }}
cancel-in-progress: ${{ github.event_name == 'pull_request' }}
```

Non-PR runs never share a group, so neither an in-progress nor a pending scheduled run can be cancelled. GitHub keeps one pending run per group, and a newly queued run cancels it; with unique groups, no scheduled run is ever pending in a shared group.

### D6. Promotion-gate script

- **Stack:** Python standard library, GET-only. The request helper raises an error for any other method, and a test covers this.
- **Endpoints:**
  - `GET /repos/{repo}/actions/workflows/formal-verification.yml/runs?branch=main&event=schedule&status=completed&per_page=30`;
  - then `GET .../runs/{id}/jobs` for candidate runs, newest first, until four current-pipeline runs are found or the candidates run out (at most 30 requests).
- **Token:** `GH_TOKEN`, then `GITHUB_TOKEN`, then `gh auth token` if `gh` is present, otherwise unauthenticated. The repository is public.
- **Flags:** `--repo` (default `invowk/invowk`), `--since`, `--now`, and `--json`.
- **Logic:** a pure function, `evaluate(runs, jobs_by_run, now, since) -> Verdict`, which applies the rule in the spec:
  - **First attempt only** (orchestrator decision): a rerun to green is exactly the flake the gate exists to catch.
  - **Step conclusions:** every heavy step must be `success`, not `skipped`. A green job with skipped steps is vacuous.
  - **Current pipeline:** the formal job must contain the `Classify changed paths` step. That step first appears in this change, which lands last, so any counted run exercised the final model set and this change's workflow. This is checkable from the jobs API alone, with no merge SHA to hard-code. `--since` gives the maintainer an extra cut-off, for example the merge date.
  - **8-day gap:** allows GitHub schedule jitter on top of the 7-day cron.
- **Job matching:** by exact name. Runs 36106509149 and 36105083723 list `JUnit Report (...)` check runs from `ci.yml`'s test reporter among the Formal Verification run's jobs.
- **Output on PASS:** the exact context string, `Models, golden vectors, and correspondence`, and its source, GitHub Actions.
- **Rejected alternative:** a CI job that reports the gate. It needs `actions: read` on a `contents: read` workflow, for a check someone runs once.

### D7. Maintainer promotion procedure (documented, not executed)

`formal/README.md` gains a "Promoting the lane" section:

1. Run `make formal-promotion-gate`. It must print PASS.
2. In Settings → Rules → "Safety", add `Models, golden vectors, and correspondence` (source: GitHub Actions) to the required status checks, next to `goplint consumer gates`. A `gh api -X PUT repos/invowk/invowk/rulesets/12472716` payload is shown for reference only.
3. Tick `adopt-formal-verification` task 8.5 with the date and the gate output, and remove the "Not a required status check" comment from the workflow header.

Rollback: remove the context from the ruleset.

### D8. Capability and archival order

The first draft modified `formal-verification-toolchain`. That capability exists only inside the unarchived `adopt-formal-verification`, and adopt's task 8.5 cannot close until this gate passes, at least four weeks after merge. That would have chained this change's archive behind adopt's. The requirements now live in a new capability, `formal-ci-gate`, as the other siblings did. The existing "CI lane and promotion" requirement stays accurate and is only tightened by the new one.

Order:
1. Implement and merge this change.
2. Archive this change. It is independent of adopt.
3. Wait for the gate to report PASS.
4. Tick adopt's 8.5.
5. Archive `adopt-formal-verification`.

**Cross-change flag for the orchestrator:** `mutation-test-formal-bindings` still carries a delta against `formal-verification-toolchain`, so it cannot be archived before adopt, which now waits on this gate.

### D9. Closing task 8.3

Tick 8.3 once `budget_seconds` (source `ci`) and `distinct_states` are in the manifest and cite the post-sibling run IDs. Reword 8.5 to: "**Phase 3 gate:** trace mutations are rejected as declared, and `make formal-promotion-gate` reports PASS (lane stays non-required until then)."

## Risks / Trade-offs

- **The streak starts only after merge.** Because only current-pipeline runs count, the first countable run is the first Monday after this change merges, and PASS comes three weeks after that. This is a deliberate trade for a gate that only counts the final pipeline.
- **GitHub disables scheduled workflows after 60 days without repository activity.** → Caught by the staleness rule.
- **Trace validation grows the most.** The new SSH and git trace suites start one JVM per trace, and trace validation was already the slowest step (39–67 s). → The hard limits are derived after re-measurement, not fixed now. If the step exceeds about 5 minutes, raise it as a follow-up to parallelise the suites.
- **Every PR starts a runner.** → Irrelevant PRs skip Go and Java setup. A classification failure runs the full lane, about 2.5 minutes, before the siblings land.
- **Path-list drift.** → The coverage self-test covers the table files, the binding-test directories, trace packages, and golden outputs. An unnamed dependency is not covered (see D3).
- **Budgets from dispatch runs, not scheduled runs.** A cold Go cache on a scheduled run can be slower. → Soft overruns only warn, and the hard limits have about 4× headroom.
- **Required-check semantics.** A job skipped at job level reports "skipped", which GitHub treats as passing. Our job always runs and skips only steps, so it reports "success". The gate separately rejects runs with skipped steps.

## Review edits not applied as written

None were rejected. Two were applied in a narrower form:
- **Review #12, archival coupling.** Of the review's two options, this change takes the new capability rather than only flagging the problem (D8). The coupling that remains for `mutation-test-formal-bindings` is flagged to the orchestrator, not fixed here, because that change is outside this change's edit scope.
- **Review #7, a creation-date cut-off.** The primary mechanism is the step-presence check (D6), which needs no merge date or SHA known in advance. `--since` stays available as an optional extra cut-off.
