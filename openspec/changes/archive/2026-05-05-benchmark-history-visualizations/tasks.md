## 1. Data Model and Fixtures

- [x] 1.1 Define the canonical benchmark report JSON schema with schema version, release identity, run metadata, environment metadata, startup metrics, Go benchmark metrics, raw source references, and warnings.
- [x] 1.2 Add representative JSON fixtures for a current release, a previous compatible release, an incompatible-environment release, a partial benchmark run, and a no-history first release.
- [x] 1.3 Add legacy Markdown benchmark fixtures based on the existing report shape for backfill coverage.
- [x] 1.4 Implement stable benchmark identity normalization that splits raw names such as `BenchmarkCUEParsing-24` into stable ID and suffix fields.
- [x] 1.5 Add validation tests for required fields, unsupported schema versions, invalid metric values, warning propagation, and stable identity normalization.

## 2. Benchmark Report Generation

- [x] 2.1 Refactor benchmark report generation so canonical JSON is generated before Markdown rendering.
- [x] 2.2 Preserve the existing Markdown report sections for metadata, startup timings, Go benchmarks, raw startup data, and raw Go benchmark output.
- [x] 2.3 Add JSON output path handling and naming so local reports and release-staged reports are discoverable and deterministic.
- [x] 2.4 Add optional raw benchmark artifact generation for Go benchmark output or benchfmt-compatible data when enabled by the release/report pipeline.
- [x] 2.5 Ensure benchmark generation fails when no Go benchmark rows are parsed or when canonical JSON validation fails.

## 3. Textual Evolution Rendering

- [x] 3.1 Implement previous-release comparison rendering from canonical report JSON and aggregate history.
- [x] 3.2 Implement last-3-months, last-1-year, and all-history textual summaries.
- [x] 3.3 Include largest improvements, largest regressions, unchanged metrics, insufficient-history states, and reduced-confidence notes.
- [x] 3.4 Annotate environment changes in Markdown comparisons for Go version, CPU, runner, OS, architecture, benchmark mode, and sample policy.
- [x] 3.5 Add tests for compatible comparisons, incompatible comparisons, missing history, legacy history, and partial benchmark data.

## 4. Static SVG Summary Rendering

- [x] 4.1 Implement deterministic SVG generation from canonical JSON and aggregate history without requiring a backend service.
- [x] 4.2 Include curated user-facing startup metrics and developer-facing Go benchmark metrics in the SVG summary.
- [x] 4.3 Render lower-is-better labels, window labels, insufficient-history states, and environment warning markers.
- [x] 4.4 Add SVG validation for existence, non-empty output, parseable SVG text, and presence of either data series or explicit empty state.
- [x] 4.5 Add golden or structural tests for SVG output using stable fixtures.

## 5. Historical Aggregation and Backfill

- [x] 5.1 Implement a history aggregation command or script that reads local assets and can fetch release assets through `gh` or the GitHub API.
- [x] 5.2 Validate JSON release assets during aggregation and fail or warn according to explicit partial-history policy.
- [x] 5.3 Parse legacy Markdown benchmark reports when JSON assets are unavailable and mark those records as `legacy-markdown`.
- [x] 5.4 Normalize release ordering, prerelease/stable metadata, metric IDs, units, environment metadata, and confidence flags.
- [x] 5.5 Emit aggregate static history data for last-3-months, last-1-year, and all-history website/report use.
- [x] 5.6 Add fixture tests covering JSON releases, legacy Markdown releases, bad assets, missing assets, partial history, and windowed aggregate output.

## 6. Release Asset Pipeline

- [x] 6.1 Update `scripts/stage-release-bench-report.sh` to stage Markdown, JSON, SVG, and optional raw benchmark assets with exact asset count and naming validation.
- [x] 6.2 Update `.goreleaser.yaml` `extra_files` to include all benchmark asset types.
- [x] 6.3 Update `.github/workflows/release.yml` to generate and publish all benchmark assets.
- [x] 6.4 Update `.github/workflows/ci.yml` release dry-run coverage to stage and validate all benchmark asset types on pull requests and integration branches.
- [x] 6.5 Update `.github/workflows/release-benchmark-asset.yml` to regenerate, upload, and verify all benchmark asset types for an existing release.
- [x] 6.6 Add release staging tests or dry-run checks that fail on missing, duplicate, or unexpected benchmark assets.

## 7. Website Performance History

- [x] 7.1 Add static aggregate history data under the website/static or docs-served data path chosen by the design implementation.
- [x] 7.2 Add TypeScript types and data-loading helpers for benchmark history records, metrics, windows, confidence notes, and environment annotations.
- [x] 7.3 Build interactive React chart components for last 3 months, last 1 year, and all history.
- [x] 7.4 Build controls for user-facing versus developer-facing metrics and time, allocation, memory, absolute, and indexed views.
- [x] 7.5 Add tables that mirror the visible chart data and remain readable without chart interaction.
- [x] 7.6 Add clear empty states for missing, partial, or insufficient history.
- [x] 7.7 Add environment annotations and reduced-confidence markers to chart and table views.
- [x] 7.8 Add the performance history page to the Docusaurus navigation in a location appropriate for users and maintainers.
- [x] 7.9 Ensure the page is accessible, responsive, dark-mode compatible, and build-time static.

## 8. Documentation

- [x] 8.1 Document benchmark release assets, including Markdown, JSON, SVG, and optional raw output.
- [x] 8.2 Document how to read last-3-months, last-1-year, and all-history views.
- [x] 8.3 Document indexed versus absolute values and lower-is-better semantics.
- [x] 8.4 Document environment annotations, confidence notes, common benchmark noise sources, and when regressions need manual investigation.
- [x] 8.5 Document maintainer workflows for regenerating reports, aggregating history, backfilling legacy reports, validating assets, and troubleshooting release/fallback failures.
- [x] 8.6 Update website i18n/doc parity surfaces required by this repo's documentation workflow.

## 9. Validation and CI

- [x] 9.1 Add or update Make targets for benchmark report generation, history aggregation, benchmark asset validation, and website history validation.
- [x] 9.2 Add shellcheck coverage for any new or changed shell scripts.
- [x] 9.3 Add Go or Node unit tests for schema validation, Markdown rendering, SVG rendering, history aggregation, and legacy backfill.
- [x] 9.4 Run `make lint` and fix lint issues.
- [x] 9.5 Run `make test` and fix test failures.
- [x] 9.6 Run website typecheck and build validation.
- [x] 9.7 Run release dry-run validation or the closest local equivalent and record any local signing limitations honestly.
- [x] 9.8 Run `make check-agent-docs` if `AGENTS.md`, `.agents/rules/`, or `.agents/skills/` changed.

## 10. Backfill and Final Acceptance

- [x] 10.1 Backfill available historical release benchmark reports into aggregate history data.
- [x] 10.2 Verify that reports with history show previous-release, last-3-months, last-1-year, and all-history textual sections.
- [x] 10.3 Verify that release SVG summaries render meaningful data or explicit insufficient-history states.
- [x] 10.4 Verify that the website performance page shows user-facing and developer-facing trends across all required windows.
- [x] 10.5 Verify that release, dry-run, and manual fallback workflows all reference the same expected benchmark asset set.
- [x] 10.6 Run `openspec validate benchmark-history-visualizations --strict` and fix all reported issues.
