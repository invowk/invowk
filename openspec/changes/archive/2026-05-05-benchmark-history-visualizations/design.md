## Context

The current benchmark release path is Markdown-first. `scripts/bench-report.sh` runs startup timing scenarios plus `internal/benchmark` Go benchmarks, summarizes them into a timestamped Markdown report under `docs/benchmarks`, and includes raw startup rows plus raw Go benchmark output. `scripts/stage-release-bench-report.sh` deletes the target report directory, runs `make bench-report`, expects exactly one Markdown report, and copies it to `release-assets/invowk_<version>_bench-report.md` for GoReleaser upload. The release workflow and manual fallback workflow both depend on that staging script.

This works for single-release inspection but is weak for performance history:

- Markdown tables are awkward and brittle as source data.
- Benchmark names include machine-specific suffixes such as `-24`, making history joins noisy unless normalized.
- Runner CPU, Go version, OS, benchmark mode, sample count, and warning state are important context but not structured.
- GitHub release assets are the durable public surface, while local `docs/benchmarks` currently contains only an example report.
- Users need simple trend signals; developers need deeper metric and environment evidence.

The change should produce a complete performance-history system, not a minimal chart. It should extend the release asset pipeline, add structured data, generate textual and graphical views, publish an interactive website page, backfill historical reports where possible, and validate every generated surface.

## Goals / Non-Goals

**Goals:**

- Make benchmark JSON the canonical data model for generated Markdown, generated SVG, release assets, and history aggregation.
- Preserve the existing Markdown release asset while enriching it with evolution comparisons.
- Publish release assets for Markdown, canonical JSON, static graphical SVG, and optional raw benchmark data.
- Build aggregate history data from release assets and expose last-3-months, last-1-year, and all-history windows.
- Provide user-facing and developer-facing views in both textual and graphical forms.
- Include confidence/noise handling by recording environment metadata, comparing compatible data where possible, and calling out environment changes.
- Add legacy backfill for old Markdown-only reports so existing releases are not ignored.
- Keep release dry-run and manual fallback paths covered so benchmark-history publication cannot silently disappear.
- Document how to read the trends, where the artifacts live, and when apparent regressions need manual interpretation.

**Non-Goals:**

- This change does not establish hard performance regression gates for every benchmark. It can expose trend and warning data, but gating thresholds require a separate policy decision.
- This change does not replace PGO profile generation or the existing PGO benchstat workflow.
- This change does not require a backend service or database; the website must be build-time/static.
- This change does not promise statistically rigorous comparisons for all data. It must communicate confidence and environment limitations honestly.

## Decisions

### Decision: Canonical JSON-first benchmark artifacts

Benchmark generation will produce a schema-versioned JSON artifact before rendering Markdown or SVG. The JSON artifact will include:

- `schema_version`
- release identity: tag, version, commit, branch, generated timestamp
- benchmark context: mode, startup samples, Go benchmark count, commands, binary path
- environment: OS, architecture, kernel, runner label, CPU model, logical CPU count, Go version
- startup measurements: scenario ID, label, samples, mean, min, max, optional raw samples
- Go benchmark summaries: stable benchmark ID, raw benchmark name, CPU suffix, samples, mean/min/max `ns/op`, `B/op`, `allocs/op`, derived timing fields
- raw artifacts or references: raw Go benchmark output, raw startup rows, optional benchfmt file path
- warnings: partial runs, incompatible environment comparisons, missing history, parser fallbacks

Rationale: all public views must agree because they render from the same source. JSON also makes release-asset validation straightforward.

Alternatives considered:

- Continue scraping Markdown: fastest initial path, but it would cement a brittle parser as the core system.
- Store only benchfmt output: excellent for Go benchmarks, but startup timings and report metadata still need a first-class schema.
- Commit every release report into the repo: simple for website builds, but release assets are already the durable release surface and avoid noisy repository churn.

### Decision: Normalize benchmark identities

Go benchmark rows will split raw names like `BenchmarkCUEParsing-24` into a stable ID (`BenchmarkCUEParsing`) and a measured CPU suffix (`24`). History joins will use stable IDs plus metric names, not raw row labels.

Rationale: suffixes vary by runner CPU and GOMAXPROCS and should not create fake benchmark series.

Alternatives considered:

- Preserve raw names only: simpler, but makes charts unstable across machines.
- Rename benchmark functions: unnecessary and potentially disruptive.

### Decision: Release assets remain the source of public history

Each release will upload:

- `invowk_<version>_bench-report.md`
- `invowk_<version>_bench-report.json`
- `invowk_<version>_bench-summary.svg`
- optionally `invowk_<version>_bench-raw.txt` or benchfmt-compatible raw output when useful

The history aggregation script will fetch release assets through `gh` or the GitHub API, validate JSON assets, use legacy Markdown parsers only for releases without JSON, and write static aggregate data for the website.

Rationale: release assets are immutable enough for public provenance and already part of Invowk's release flow.

Alternatives considered:

- Git history as source: misses retroactive/fallback assets and couples chart history to docs commits.
- GitHub Actions artifacts as source: artifacts expire and are not reliable for long-term public history.

### Decision: Static SVG release summary plus interactive website charts

The release workflow will generate a deterministic SVG summary asset for the release. The website will render interactive React charts from static aggregate JSON.

The static SVG should show a small curated set:

- CLI startup scenarios
- `BenchmarkFullPipeline`
- key parser/discovery benchmarks
- largest improvements and regressions when history exists

The website should provide:

- window controls: last 3 months, last 1 year, all
- audience controls: users, developers
- metric controls: time, allocations, memory
- absolute and indexed views, with indexed series defaulting to `100` at the first point in the selected window
- tables that mirror visible chart data
- environment annotations for Go version, CPU, runner, OS, and benchmark mode changes

Rationale: SVG is robust on GitHub releases, while the website can offer exploration without needing a backend.

Alternatives considered:

- Use D2 for charts: D2 fits architecture diagrams, but time-series charts need axes, scales, and tabular data semantics.
- Add a full charting library immediately: useful if interactions grow, but deterministic React/SVG rendering may be enough and avoids another dependency.
- Generate only PNG images: less accessible and harder to diff than SVG.

### Decision: Textual evolution belongs in the Markdown report

The Markdown report will keep current metadata, startup timing, Go benchmark, and raw output sections, and add:

- Performance Evolution summary
- Previous Release comparison
- Last 3 Months comparison
- Last 1 Year comparison
- All History overview
- Environment Notes
- Links or filenames for JSON and SVG sibling assets

Comparisons will identify best/worst changes, highlight compatible and incompatible environments, and avoid overstating results when data is missing.

Rationale: release readers should not need to open a website to understand whether a release changed performance materially.

### Decision: Backfill is compatibility, not the steady-state path

A backfill script will parse legacy Markdown benchmark reports and release assets where JSON is missing. Parsed legacy records will be marked with lower confidence and a `source_kind` such as `legacy-markdown`.

Rationale: historical coverage is valuable, but the future path should not depend on Markdown parsing.

### Decision: Validation is part of the release contract

Validation will cover:

- JSON schema and required fields
- asset naming and count expectations
- Markdown generated from JSON
- SVG generated from JSON and non-empty
- history aggregate generated from known fixtures
- website TypeScript/build success
- release dry-run and manual fallback workflow support for all benchmark assets

Rationale: the previous benchmark-report release path had a subtle failure mode where a path could appear healthy without running useful benchmark rows. This feature must keep the release path exercised end to end.

## Risks / Trade-offs

- Runner noise and hardware changes can masquerade as product changes -> Record environment metadata, annotate changes, default to indexed views, and separate compatible from incompatible comparisons.
- Full history fetching could hit GitHub API limits or slow docs builds -> Generate aggregate history in a scheduled/manual workflow and commit or upload static data rather than fetching during Docusaurus build.
- JSON schema evolution could break older website data -> Version the schema and implement explicit migration/backfill handling.
- SVG rendering can become hard to maintain if handcrafted directly in shell -> Prefer a dedicated Go or Node utility with tests and golden fixtures.
- Adding chart dependencies can bloat the website -> Start with deterministic React/SVG components unless richer interactions justify a small dependency.
- Markdown and graphical views can drift -> Render both from canonical JSON and validate generated fixtures.
- Legacy Markdown parsing can be lossy -> Mark legacy records with lower confidence and retain warnings in the aggregate data.
- Release fallback workflow can lag the main release workflow -> Update both workflows and add validation that all expected asset globs are handled.

## Migration Plan

1. Introduce JSON schema, parser data model, and fixture-based tests.
2. Update benchmark report generation to emit JSON first, then Markdown from JSON, while preserving the existing report name and content shape.
3. Add textual evolution rendering using existing history input when available and graceful "insufficient history" output otherwise.
4. Add deterministic SVG summary generation from JSON/history.
5. Update release staging to stage Markdown, JSON, SVG, and optional raw assets, with exact expected asset validation.
6. Update GoReleaser `extra_files`, release workflow, release dry-run workflow, and manual fallback workflow.
7. Add history aggregation from release assets, including legacy Markdown backfill.
8. Add website static data, performance history page, chart/table components, navigation, i18n-ready text, and docs.
9. Add validation targets and CI coverage.
10. Backfill current public release history where possible and document confidence limitations.

Rollback strategy:

- Keep Markdown generation compatible throughout implementation.
- If website history has a problem, release artifact generation can continue independently.
- If SVG generation fails unexpectedly, release staging should fail before publishing rather than upload incomplete history assets.

## Open Questions

- Should aggregate history data be committed to the repository, generated into `website/static`, or uploaded as a release/docs deployment artifact?
- Should raw sample values be included in public JSON by default, or only summary statistics plus raw Go benchmark output?
- Should the interactive website page live under docs (`/docs/performance/history`) or as a top-level page (`/performance`)?
- Should prerelease and stable release history be shown together by default, or should stable releases be the default with a toggle for prereleases?
