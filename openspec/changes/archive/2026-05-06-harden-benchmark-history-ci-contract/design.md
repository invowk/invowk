## Context

The benchmark history feature spans Go benchmark execution, shell report generation, Node-based rendering/validation, release asset staging, GoReleaser packaging, and Docusaurus website rendering. The current CI coverage is distributed across several jobs:

- The main CI test matrix runs a focused `internal/benchmark` smoke test on one Ubuntu/Docker leg.
- `Release Dry Run` stages benchmark report assets and feeds them into GoReleaser.
- `Test Website Build` validates `website/static/benchmarks/history.json` and builds the website.
- Script tests validate the benchmark report renderer and aggregation helpers.

Those checks are useful, but their composition is not explicit enough. In particular, release dry-run benchmark staging can currently run without the committed `website/static/benchmarks/history.json`, so CI can prove "assets exist" without proving "release-style report generation renders against real history." The hardening should keep CI fast, deterministic, and compatible with normal GitHub-hosted runners.

## Goals / Non-Goals

**Goals:**

- Add a GitHub Actions-compatible benchmark-history contract that proves report generation, history-aware rendering, release staging, and website static outputs compose correctly.
- Make release dry-run, release publish, and manual fallback staging use the same explicit benchmark history input.
- Preserve the existing fast benchmark smoke gate as the proof that focused benchmark code still compiles and runs.
- Keep the new contract lightweight enough for pull requests by using low sample counts and the existing short benchmark subset.
- Fail with actionable messages when staged assets are missing, malformed, unexpectedly named, or not history-aware when history input is present.

**Non-Goals:**

- Do not add long-running full benchmark performance measurement to every pull request.
- Do not change the benchmark report JSON schema unless implementation discovers a required validation field is missing.
- Do not require external release downloads, release publishing permissions, or mutable GitHub release state in pull-request CI.
- Do not replace the existing website build workflow or release workflows.

## Decisions

### Add a benchmark-history contract job

Add an Ubuntu-only CI job dedicated to the benchmark history pipeline. It should run independently of the OS test matrix and use only normal GitHub-hosted runner capabilities: checkout, Go setup, Node setup, `go mod download`, and local scripts.

The job should run:

1. `node scripts/test_benchmark_report.mjs`
2. `node scripts/benchmark-report.mjs validate-history --input website/static/benchmarks/history.json`
3. `TAG=v0.0.0-ci STARTUP_SAMPLES=3 BENCH_COUNT=1 BENCH_REPORT_OUT_DIR=$RUNNER_TEMP/release-benchmarks BENCH_HISTORY_JSON=website/static/benchmarks/history.json scripts/stage-release-bench-report.sh`
4. Assertions over `release-assets/` proving the expected Markdown, JSON, SVG, and raw assets exist and the Markdown includes history/evolution content.

Alternative considered: fold these checks into the existing `Release Dry Run` job only. Keeping a named contract job gives maintainers a clearer failure surface while still allowing release dry-run to exercise GoReleaser packaging.

### Pass benchmark history explicitly in every release staging path

The release dry-run, real release, and manual fallback workflows should set:

```yaml
BENCH_HISTORY_JSON: website/static/benchmarks/history.json
```

This keeps rehearsal and release behavior aligned. `scripts/stage-release-bench-report.sh` should continue to work without that variable for local first-report scenarios, but CI and release workflows should be explicit.

Alternative considered: change `scripts/stage-release-bench-report.sh` to always default to the website history file. That would reduce workflow YAML, but it would make local first-run or alternate-history use cases less obvious.

### Validate content, not only file presence

Staging already validates expected release asset names and non-empty files. Harden it by ensuring the staged JSON validates as a benchmark report, the staged SVG validates as a non-empty SVG, and history-aware runs produce the expected history/evolution section in Markdown.

The validation can stay in Node and shell scripts; no new third-party action is needed.

Alternative considered: parse the generated Markdown in shell with simple greps only. Greps are acceptable for high-level "history section exists" assertions, but schema/SVG validation should remain in `scripts/benchmark-report.mjs` so logic is shared with local checks.

### Keep benchmark execution scope intentionally short in CI

The contract job should use `STARTUP_SAMPLES=3`, `BENCH_COUNT=1`, and the existing short benchmark subset. Full benchmark runs remain available through `make bench-report-full`, release/fallback defaults, and manual performance investigations.

Alternative considered: run `make bench-report-full` in CI. That would better exercise container/full benchmark surfaces, but it would make pull-request CI slower and more runner-sensitive without materially improving the report/history integration contract.

### Add website post-build static checks

After `npm run build`, `Test Website Build` should verify that the built output includes the performance page and benchmark history JSON. This proves Docusaurus actually shipped the static surfaces that the runtime page expects.

Alternative considered: add Playwright browser checks immediately. Browser checks would be stronger for interactive controls but introduce more moving parts. A static build contract is the right first hardening step; Playwright can be a follow-up if visual/runtime regressions appear.

## Risks / Trade-offs

- History fixtures or committed history drift can make the new contract noisy. Mitigation: keep `validate-history` as the first failure point and document regeneration steps in the benchmark history docs if they are missing.
- Release dry-run and the new contract may duplicate some work. Mitigation: keep the contract focused on report/history/staging assertions and keep release dry-run focused on GoReleaser packaging.
- Content assertions can become brittle if headings change. Mitigation: assert stable semantic output such as "Performance Evolution" only while that heading is part of the existing generated report contract.
- Low sample counts do not catch performance regressions. Mitigation: this change is a pipeline correctness contract; performance regression detection remains with benchmark reports, scheduled benchstat, and manual full runs.

## Migration Plan

1. Add explicit `BENCH_HISTORY_JSON` to release dry-run, release publish, and fallback benchmark workflows.
2. Add the benchmark-history contract job to CI.
3. Strengthen local script validation so CI and local runs share the same checks.
4. Add website post-build static output checks.
5. Verify locally with the Node benchmark report tests, history validation, staging dry-run, website build, and any existing workflow lint/check targets.

Rollback is straightforward: remove the new CI job and explicit workflow environment variables. The underlying benchmark report generation and release staging scripts remain compatible with the current behavior.

## Open Questions

- Should the benchmark-history contract job upload the generated release assets as CI artifacts for debugging, or is console output sufficient for the first pass?
- Should static website output checks stay in `test-website.yml`, or should the new CI contract job also inspect the built website once to keep all benchmark-history checks together?
