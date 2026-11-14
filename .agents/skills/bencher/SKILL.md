---
name: bencher
description: Maintain and troubleshoot Invowk's Bencher benchmark infrastructure. Use when working on Bencher, BMF benchmark output, benchmark GitHub Actions, PGO benchstat comparisons, dedicated/bare-metal runner handoff, benchmark image packaging, Bencher thresholds, "No thresholds found" dashboard warnings, benchmark alerts, the accepted-regression PR label, or files such as `.github/workflows/benchmarks*.yml`, `.github/workflows/pgo-benchstat.yml`, `scripts/bench-bmf.mjs`, `scripts/bencher-threshold-args.sh`, `scripts/bencher-registry-login.sh`, and `build/bencher/Dockerfile`.
---

# Bencher

## Overview

Use this skill to keep Invowk's Bencher integration coherent: GitHub Actions packages the benchmark image, Bencher runs the measurement job on the configured Spec/Testbed, and BMF output plus thresholds determine the dashboard, alerts, and PR status.

## Source Files

- `.github/workflows/benchmarks.yml`: weekly `main` series (cron, Monday 06:00 UTC), manual dispatch (use it before tagging a release), and PRs that carry the `benchmarks: run` label. It no longer runs on every push or PR: the Free plan's 10 GiB/day OCI bandwidth allows only ~6 remote runs a day, and a run that cannot push its image skips tracking and still shows green (see the "Benchmarks NOT measured" job summary).
- `.github/workflows/benchmarks-upload.yml`: trusted workflow for fork PR benchmark upload; it also skips PRs without the `benchmarks: run` label.
- `.github/workflows/pgo-benchstat.yml`: scheduled/manual GitHub-hosted comparison of `-pgo=off` versus default PGO using `benchstat`; this is not a Bencher upload path, but shares benchmark workflow hygiene.
- `.github/workflows/release.yml`: release-tag performance tracking against the previous tag.
- `build/bencher/Dockerfile`: reproducible benchmark image, Go/Node versions, vendoring, warmup.
- `scripts/bench-bmf.mjs`: emits BMF JSON and controls the public tracked suite through `TRACKED_GO_BENCHMARKS` and `SHORT_BENCH_REGEX`.
- `scripts/bencher-threshold-args.sh`: defines every threshold model passed to `bencher run`.
- `scripts/bencher-registry-login.sh`: derives the Docker registry user from the Bencher JWT `sub` claim.
- Benchmark workloads currently come from `internal/benchmark`, `internal/app/modulesync`, `internal/app/moduleops`, and `cmd/invowk`.
- Local BMF generation (`make bench-bmf`, `make bench-bmf-full`) and its `artifacts/benchmarks/` outputs: `.agents/runbooks/pgo-and-benchmarks.md`.

## Runner Model

- Preserve the dedicated Bencher runner design. GitHub Actions should package and push the benchmark image only; it must not measure benchmarks on GitHub-hosted runners.
- Keep `BENCHER_SPEC` and `BENCHER_TESTBED` in sync across both benchmark workflows and release performance tracking. Current values are `intel-v1` and `bencher-intel-v1-go-1-27`.
- For base-repo PRs, use branch `pr-<number>`, hash the PR head SHA, and pass the base branch and base SHA as the start point.
- For PR branches, keep `--start-point-clone-thresholds` and `--start-point-reset` so PR history is anchored to the base branch.
- For fork PRs, use the trusted upload workflow: checkout trusted packaging separately from untrusted source, build the image with trusted workflow/scripts, then ask Bencher to run the source image.
- For releases, benchmark the tag against the previous tag as the start point so release history is anchored to shipped versions rather than PR branches.
- PR benchmarks are opt-in: add the `benchmarks: run` label to PRs that touch hot paths (CUE parsing, discovery, runtime execution, module sync). Adding the label triggers the run; later pushes re-run while the label stays. Other labels do not trigger runs.
- Job budget: Bencher Cloud's Free plan clamps every remote job to 300 s (`BENCHER_JOB_TIMEOUT: 300`); keep `scripts/bench-bmf.mjs` `bench_count` low enough that the suite fits (~250 s at 3).
- Alert policy:
  - PR benchmark alerts fail by default through `--error-on-alert`.
  - If a maintainer has deliberately accepted an expected feature cost, add the `benchmarks: accepted-regression` PR label. Workflows then set `BENCHER_ERROR_ON_ALERT=false` for that PR, so alerts remain visible in Bencher while the Actions job stays advisory.
  - With the weekly main series, a PR's start point is usually main data up to a week old, so a PR alert can come from a change merged since the last weekly run; check the main series before blaming the PR.
  - Scheduled and manual runs stay advisory (a weekly series would stay red for weeks after an accepted regression while the 64-sample baseline absorbs it), so their alerts surface only in the Bencher report and dashboard: read the `bencher run` alert output or the report after each weekly run.
  - GitHub disables scheduled workflows after 60 days without repository activity; if the weekly series stops, re-enable the workflow.
  - Scheduled and manual benchmark runs are advisory: they publish main history to Bencher without `--error-on-alert` and without a Bencher GitHub check when alerts would otherwise fail already-merged history.
  - Release-tag benchmark alerts remain blocking.
  - Do not use advisory mode for benchmark script bugs, missing thresholds, or unexplained regressions.

## Threshold Model

- Treat thresholds as linked to `branch + testbed + measure`, not to individual benchmarks.
- Use measure slugs as the stable identifiers in `--threshold-measure`. Bencher may display title-cased names such as `Latency`, `File Size`, and `Build Time`, while their slugs are `latency`, `file-size`, and `build-time`.
- If `scripts/bench-bmf.mjs` emits a new measure and alerts are expected, add the matching threshold model in `scripts/bencher-threshold-args.sh` in the same patch.
- If `scripts/bench-bmf.mjs` adds only new benchmarks using existing measures, no new threshold is needed.
- Keep `--thresholds-reset` so stale threshold models do not survive after measures are removed or renamed.
- Use upper-bound-only thresholds for regressions. Lower values are improvements for latency, memory, allocations, build time, and file size.

Current emitted measures and threshold defaults:

| Measure slug | Display name | Threshold |
|--------------|--------------|-----------|
| `latency` | `Latency` | percentage, min sample 5, max sample 64, upper `0.10` |
| `memory` | `memory` | percentage, min sample 5, max sample 64, upper `0.10` |
| `allocations` | `allocations` | percentage, min sample 5, max sample 64, upper `0.10` |
| `build-time` | `Build Time` | percentage, min sample 5, max sample 32, upper `0.20` |
| `file-size` | `File Size` | percentage, min sample 2, max sample 16, upper `0.10` |

## Diagnostics

When the dashboard says `No thresholds found`, check the report JSON first. A successful `bencher run` can still have one unthresholded measure.

Load [`references/diagnostics.md`](references/diagnostics.md) for the `curl`/`jq` recipes (unthresholded measures, threshold coverage by measure, active threshold models) and the Actions log search.

## Known Pitfalls

- A flat absolute latency increase across every CLI-spawning benchmark (`startup/*`, `command/execute-cli-*`) while in-process benchmarks are flat or improved points at process-init cost, not workload cost. Diagnose with `GODEBUG=inittrace=1 bin/invowk version` (per-package init wall time and allocations) and bisect with per-dependency `go get` builds. Precedent: transitive `go-runewidth` v0.0.27 built width tables eagerly in `init()` (~15ms, doubling startup); v0.0.28 restored lazy init.
- The `Bencher Report (bmf)` GitHub check is created per report and pinned to the head SHA. When a later run goes advisory (e.g., after adding `benchmarks: accepted-regression`), it deliberately creates no new check, so a stale failing check from a pre-label run stays attached until a new commit changes the SHA. Also: workflows read PR labels from the frozen event payload. Adding `benchmarks: run`, or adding `benchmarks: accepted-regression` while `benchmarks: run` is present, triggers a fresh run; any other label change needs a fresh `pull_request` event (new push, or close/reopen), and rerunning an old workflow run reuses its old payload.
- `latency` versus `Latency` is not a threshold bug by itself: the slug `latency` correctly matches the displayed `Latency`. Look for a measure without a threshold instead (the past `No thresholds found` warning came from a missing `build-time` threshold).
- `boundary.baseline: null` usually means the threshold exists but Bencher does not yet have enough samples for that model. This is different from `threshold: null`.
- A Bencher JWT or registry failure often looks like broken benchmark code. Check `BENCHER_API_TOKEN`, the registry login script, and direct API access before rewriting benchmark logic.
- `benchmarks: accepted-regression` is not the only advisory-alert path. `scripts/bencher-threshold-args.sh` also drops `--error-on-alert` when the start-point branch has no latest benchmark result, because Bencher cannot compare against an absent baseline.
- Context7 may resolve `Bencher` to unrelated benchmark projects. When that happens, use official Bencher docs plus direct API, CLI output, and report JSON as the source of truth:
  - `https://bencher.dev/docs/explanation/bencher-run/`
  - `https://bencher.dev/docs/explanation/thresholds/`
  - `https://bencher.dev/docs/api/projects/reports/`
- Do not treat `SHORT_BENCH_REGEX` as internal cleanup. It controls what becomes long-lived public benchmark history.
- In `pgo-benchstat.yml`, keep `BENCH_*` settings available to every step that references them (general rule: `.agents/rules/ci-workflows.md`). Today they are set only on the `Run benchmark batches` step, so the `Generate benchstat report` step fails under `set -u` with `BENCH_REGEX: unbound variable`. Promote shared benchmark variables to job-level `env:` or duplicate them on each step that uses them.
- After renaming a Go benchmark function, update both `TRACKED_GO_BENCHMARKS` and `benchmarkNameMap`. Keep the public benchmark slug stable when the workload is the same behavior under a renamed implementation, so Bencher history and thresholds continue across code renames.
- Large feature branches can trigger broad, legitimate alerts across latency, memory, allocations, and `binary/bin-invowk` file size. First verify the BMF script still emits the intended tracked suite and every measure has thresholds. If the regression is an intentional tradeoff, use the explicit `benchmarks: accepted-regression` label instead of weakening thresholds or deleting `--error-on-alert` globally.

## Verification

After Bencher infrastructure changes:

- Recheck Bencher constants: `rg -n 'BENCHER_(PROJECT|SPEC|TESTBED)' .github/workflows/benchmarks.yml .github/workflows/benchmarks-upload.yml .github/workflows/release.yml`.
- Run focused script checks: `bash -n scripts/bencher-threshold-args.sh`, `node scripts/test_bench_bmf.mjs`, and `bash scripts/test_bencher_registry_login.sh`.
- Run `make test-scripts` for broad script coverage.
- Run `make check-agent-docs` if this skill changed.
- Push, then add the `benchmarks: run` label (or dispatch the workflow on the branch) and watch the `Benchmarks / Track Benchmarks` check; it no longer runs on plain pushes.
- Verify the latest Bencher report has zero `threshold: null` measures if the task involved thresholds (recipe: [`references/diagnostics.md`](references/diagnostics.md)).
- Check PR status with `gh pr view <number> --json mergeStateStatus,mergeable,statusCheckRollup`.
