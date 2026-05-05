## Why

Invowk already publishes benchmark reports with releases, but those reports are point-in-time Markdown documents that do not show whether performance is improving, regressing, or merely changing because of the runner environment. Developers and users need a release-quality performance history that makes the last 3 months and last 1 year easy to understand while preserving enough raw data for deeper investigation.

## What Changes

- Add a canonical machine-readable benchmark report artifact alongside the existing Markdown release report.
- Add generated textual evolution sections to benchmark reports, including previous-release, last-3-months, last-1-year, and all-history comparisons.
- Add generated static graphical benchmark summaries as release assets.
- Add an interactive website performance history page with graphical and tabular views for user-facing and developer-facing metrics.
- Add a history aggregation pipeline that fetches, validates, normalizes, and publishes benchmark data from release assets.
- Add legacy backfill support for historical Markdown-only benchmark reports.
- Add validation and CI coverage so release benchmark data, generated Markdown, graphical assets, website history data, and fallback release-asset workflows cannot silently drift.
- Add documentation for interpreting benchmark trends, environment changes, confidence/significance, and known noise sources.
- Preserve the existing Markdown report and release asset behavior for users who rely on textual reports.

## Capabilities

### New Capabilities
- `benchmark-history-visualizations`: Covers canonical benchmark data artifacts, textual evolution reporting, static graphical release assets, interactive website history, historical aggregation, backfill, validation, and documentation for performance evolution.

### Modified Capabilities

None.

## Impact

- Affected scripts: `scripts/bench-report.sh`, `scripts/stage-release-bench-report.sh`, new benchmark data/render/history utilities, and related script tests.
- Affected release systems: `.github/workflows/release.yml`, `.github/workflows/release-benchmark-asset.yml`, release dry-run coverage in `.github/workflows/ci.yml`, GoReleaser `extra_files`, and any workflow that validates generated release assets.
- Affected docs site: Docusaurus source under `website/`, static performance history data, navigation/sidebar configuration, TypeScript types, and website build/typecheck validation.
- Affected benchmark artifacts: release Markdown reports remain, with additional JSON, static SVG, aggregate history JSON, and optional raw benchfmt-compatible files.
- Affected documentation: README or website docs for performance reporting, release assets, and benchmark trend interpretation.
- Potential dependencies: preferably standard Go and Node tooling already present in the repo; a small charting dependency may be introduced only if the design proves it materially simpler than deterministic SVG/React rendering.
