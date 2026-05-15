---
name: bencher
description: Maintain and troubleshoot Invowk's Bencher benchmark infrastructure. Use when working on Bencher, BMF benchmark output, benchmark GitHub Actions, dedicated/bare-metal runner handoff, benchmark image packaging, Bencher thresholds, "No thresholds found" dashboard warnings, benchmark alerts, or files such as `.github/workflows/benchmarks*.yml`, `scripts/bench-bmf.mjs`, `scripts/bencher-threshold-args.sh`, `scripts/bencher-registry-login.sh`, and `build/bencher/Dockerfile`.
---

# Bencher

## Overview

Use this skill to keep Invowk's Bencher integration coherent: GitHub Actions packages the benchmark image, Bencher runs the measurement job on the configured Spec/Testbed, and BMF output plus thresholds determine the dashboard, alerts, and PR status.

## Source Files

- `.github/workflows/benchmarks.yml`: base-repo PR and `main` benchmark flow.
- `.github/workflows/benchmarks-upload.yml`: trusted workflow for fork PR benchmark upload.
- `build/bencher/Dockerfile`: reproducible benchmark image, Go/Node versions, vendoring, warmup.
- `scripts/bench-bmf.mjs`: emits BMF JSON and controls the public tracked suite through `TRACKED_GO_BENCHMARKS` and `SHORT_BENCH_REGEX`.
- `scripts/bencher-threshold-args.sh`: defines every threshold model passed to `bencher run`.
- `scripts/bencher-registry-login.sh`: derives the Docker registry user from the Bencher JWT `sub` claim.
- Benchmark workloads currently come from `internal/benchmark`, `internal/app/modulesync`, `internal/app/moduleops`, and `cmd/invowk`.

## Runner Model

- Preserve the dedicated Bencher runner design. GitHub Actions should package and push the benchmark image only; it must not measure benchmarks on GitHub-hosted runners.
- Keep `BENCHER_SPEC` and `BENCHER_TESTBED` in sync across both benchmark workflows. Current values are `intel-v1` and `bencher-intel-v1-go-1-26`.
- For base-repo PRs, use branch `pr-<number>`, hash the PR head SHA, and pass the base branch and base SHA as the start point.
- For PR branches, keep `--start-point-clone-thresholds` and `--start-point-reset` so PR history is anchored to the base branch.
- For fork PRs, use the trusted upload workflow: checkout trusted packaging separately from untrusted source, build the image with trusted workflow/scripts, then ask Bencher to run the source image.

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

```bash
project=ddfe58db-e86d-49b8-a6c7-60fc46eabf0b
branch=pr-112
testbed=bencher-intel-v1-go-1-26
api=https://api.bencher.dev/v0
curl -fsS "$api/projects/$project/reports?branch=$branch&testbed=$testbed&sort=date_time&direction=desc&per_page=1" |
	jq -r '.[0].results[][] as $bench |
		$bench.measures[] |
		select(.threshold == null) |
		[$bench.benchmark.name, .measure.slug, .measure.name, .metric.value] |
		@tsv'
```

Summarize threshold coverage by measure:

```bash
curl -fsS "$api/projects/$project/reports?branch=$branch&testbed=$testbed&sort=date_time&direction=desc&per_page=1" |
	jq -r '[.[0].results[][] as $bench |
		$bench.measures[] |
		{measure:.measure.slug, has_threshold:(.threshold != null)}] |
		group_by(.measure)[] |
		[.[0].measure, length, (map(select(.has_threshold)) | length), (map(select(.has_threshold | not)) | length)] |
		@tsv'
```

List the active threshold models:

```bash
curl -fsS "$api/projects/$project/thresholds?branch=$branch&testbed=$testbed" |
	jq -r '.[] |
		[.measure.slug, .measure.name, .model.test, .model.upper_boundary, .model.min_sample_size, .model.max_sample_size] |
		@tsv' |
	sort
```

Inspect Actions logs for the actual `bencher run` request and report URL:

```bash
gh run view <run-id> --repo invowk/invowk --log > /tmp/bencher.log
rg -n 'Bencher New Report|thresholds|View report|No thresholds|Job status|BENCHER_|--threshold|--start-point' /tmp/bencher.log
```

## Known Pitfalls

- `latency` versus `Latency` is not a threshold bug by itself. In PR 112, the slug `latency` correctly matched the displayed `Latency`; the warning came from missing `build-time`.
- `boundary.baseline: null` usually means the threshold exists but Bencher does not yet have enough samples for that model. This is different from `threshold: null`.
- A Bencher JWT or registry failure often looks like broken benchmark code. Check `BENCHER_API_TOKEN`, the registry login script, and direct API access before rewriting benchmark logic.
- Context7 may resolve `Bencher` to unrelated benchmark projects. When that happens, use official Bencher docs plus direct API, CLI output, and report JSON as the source of truth.
- Do not treat `SHORT_BENCH_REGEX` as internal cleanup. It controls what becomes long-lived public benchmark history.

## Verification

After Bencher infrastructure changes:

- Run focused script checks such as `bash -n scripts/bencher-threshold-args.sh`.
- If BMF output changed, run the Node tests or the BMF script tests in CI.
- Push and watch the `Benchmarks / Track Benchmarks` check.
- Verify the latest Bencher report has zero `threshold: null` measures if the task involved thresholds.
- Check PR status with `gh pr view <number> --json mergeStateStatus,mergeable,statusCheckRollup`.
