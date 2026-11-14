# Formal verification phase 3 close-out (F0)

> **STATUS: OPEN** (2026-11-14). Closes the formal-verification adoption before the
> next wave (`harden-formal-instruments` and the finding fixes). Not an OpenSpec change:
> it finishes existing changes and archives them.

## State on 2026-11-14

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
