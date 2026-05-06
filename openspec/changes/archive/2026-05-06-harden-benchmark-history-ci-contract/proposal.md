## Why

The benchmark history feature now has multiple release, report, aggregation, and website surfaces, but the current CI checks can pass without proving that the release dry-run renders against the committed history data. Hardening this contract now prevents benchmark-report regressions from first appearing during a real release or after a performance page deploy.

## What Changes

- Add an explicit benchmark-history CI contract that runs in GitHub Actions and validates report generation, history-aware rendering, release asset staging, and website static asset availability.
- Make release dry-run, main release, and manual fallback benchmark staging pass the committed benchmark history data explicitly.
- Strengthen staged asset validation so CI proves the generated Markdown, JSON, SVG, and raw assets exist, are schema-valid, and include history/evolution content when history data is available.
- Add website post-build checks for the static performance page and benchmark history JSON output.
- Keep the existing internal benchmark smoke run as a separate fast benchmark-code execution gate.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `benchmark-history-visualizations`: strengthen validation requirements so CI must exercise history-aware benchmark report generation, release staging, aggregate history validation, and website static outputs.

## Impact

- Affected workflows: `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/workflows/release-benchmark-asset.yml`, and `.github/workflows/test-website.yml`.
- Affected scripts/targets: `scripts/stage-release-bench-report.sh`, `scripts/bench-report.sh`, `scripts/benchmark-report.mjs`, `scripts/test_benchmark_report.mjs`, and related Make targets if needed.
- Affected artifacts: `release-assets/invowk_<version>_bench-report.{md,json}`, `release-assets/invowk_<version>_bench-summary.svg`, `release-assets/invowk_<version>_bench-raw.txt`, and `website/static/benchmarks/history.json`.
- No user-facing CLI API or benchmark data schema breaking changes are intended.
