# Formal verification phase 3 close-out (F0)

> **STATUS: DONE** (2026-11-14). Closes the formal-verification adoption before the
> next wave (`harden-formal-instruments` and the finding fixes). Not an OpenSpec change:
> it finishes existing changes and archives them.

## Result (2026-11-14)

1. Docs-only PR #165: the formal check skipped Setup Go and every formal step ("affected: 0 of 14 changed paths") and reported success.
2. The maintainer made `Models, golden vectors, and correspondence` a required check of the "Safety" ruleset, so the "only `goplint consumer gates`" bullet below is historical.
3. Gates re-verified on main 6002b8cf. These pass: tidy, license-check, lint, test-scripts, check-file-length, `make test` (2:01, `-race` CLI), the goplint routed gate and baseline, `make formal` (174 verdicts) plus `make formal-traces` (208 traces, 24 mutations), and the artifacts check. `lint-scripts` has 7 pre-existing info-level findings. `sonar-local` on main failed on go:S5332 (Accepted; real gap N5/S3), which the separate e13 suppression PR addresses. 8.5 and 10.1–10.8 are ticked.
4. `promote-formal-ci-gate` 0.1, 5.2 and 5.3 are ticked.
5. `/learn` was not run: the 2026-11-14 agent-docs optimisation (#164) supersedes it (5.4, 7.10, 10.8).
6. The seven changes were archived in merge order. The eight new main specs got real Purposes and were split to pass `openspec validate --specs --strict`. The PR is left to the coordinator.

## State on 2026-11-14 (before close-out)

- `make formal-promotion-gate` reports **PASS**: four consecutive first-attempt green
  weekly runs on main `85f1eb7b` (2026-10-19, 10-26, 11-02, 11-09; run ids 39260158549,
  40328419492, 41478013172, 42655328044).
- The check context `Models, golden vectors, and correspondence` reported SUCCESS on PRs
  #157–#160. No docs-only PR has gone through the always-report lane since #157.
- The main ruleset requires only `goplint consumer gates`.

## Steps

| # | Step | Owner |
|---|---|---|
| 1 | Open a docs-only PR (for example, this file's archive commit) and confirm that the formal check runs, skips Setup Go and every formal step, and still reports success (`promote-formal-ci-gate` task 5.2). | agent opens PR, maintainer approves |
| 2 | Add `Models, golden vectors, and correspondence` (source GitHub Actions) to the required status checks of the "Safety" ruleset. Only after step 1 passes: a required context that does not report on some PR class blocks those PRs. | **maintainer** (GitHub UI) |
| 3 | Re-verify `adopt-formal-verification` section 10 (10.1–10.8) on current main before ticking: `make tidy`, `make license-check`, `make lint`, `make lint-scripts`, `make test-scripts`, `make check-file-length`, `make test`, `make check-goplint-consumer-routed`, `make check-baseline`, `make formal`, the `git ls-files artifacts/ '**/testdata/rapid/**'` check, `make sonar-local`. Tick 8.5 once step 2 is done. | agent |
| 4 | Tick `promote-formal-ci-gate` 0.1 (all five siblings merged: #153, #155, #156, #158 and #157 itself), 5.2 (step 1), 5.3 (`make lint` + `make test`; `-race` now runs locally because gcc is present). | agent |
| 5 | Run `/learn` for 5.4, `formal-infra-refinements` 7.10 and `adopt-formal-verification` 10.8 in one pass. | agent |
| 6 | Sync and archive the seven formal changes: `adopt-formal-verification`, `formal-infra-refinements`, `trace-validate-token-and-lock`, `model-concurrent-lock-writes`, `model-module-path-containment`, `mutation-test-formal-bindings`, `promote-formal-ci-gate`. | agent (signed commit, PR) |

The F16 junction probe never ran on Windows CI (it skipped with "junction creation
unavailable here"). That is owned by `harden-formal-instruments`, not by this close-out,
and does not block step 2: the skip does not change the check's verdict on any PR.
