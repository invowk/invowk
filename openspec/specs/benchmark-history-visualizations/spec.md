## Purpose
Benchmark reports provide canonical textual, graphical, and historical performance evidence for Invowk releases so users and maintainers can understand current performance and performance evolution across the last 3 months, last 1 year, and all available history.
## Requirements
### Requirement: Canonical benchmark data artifact
The benchmark report pipeline SHALL emit a schema-versioned JSON artifact that is the canonical source for generated benchmark Markdown reports, graphical summaries, and historical aggregation.

#### Scenario: JSON artifact is generated with the report
- **WHEN** a benchmark report is generated for a release or release dry run
- **THEN** the system SHALL write a JSON artifact containing schema version, release identity, commit, generation timestamp, benchmark mode, commands, startup measurements, Go benchmark summaries, environment metadata, warnings, and source references

#### Scenario: JSON artifact preserves benchmark context
- **WHEN** the report includes startup and Go benchmark data
- **THEN** the JSON artifact SHALL include enough metadata to identify sample counts, benchmark counts, benchmark commands, binary path, Go version, platform, CPU model, logical CPU count, and runner context

#### Scenario: Invalid JSON artifact fails validation
- **WHEN** a generated benchmark JSON artifact is missing required fields or has an unsupported schema version
- **THEN** validation SHALL fail with an actionable error naming the missing or unsupported field

### Requirement: Stable benchmark identity normalization
The benchmark report pipeline SHALL normalize Go benchmark row names into stable benchmark identities while preserving raw benchmark names and machine-specific suffixes.

#### Scenario: CPU suffix is separated from benchmark identity
- **WHEN** raw Go benchmark output contains a row such as `BenchmarkCUEParsing-24`
- **THEN** the system SHALL store `BenchmarkCUEParsing` as the stable benchmark ID and `24` as suffix metadata

#### Scenario: Historical joins use stable identities
- **WHEN** historical data is aggregated across releases
- **THEN** the system SHALL join Go benchmark series by stable benchmark ID and metric name rather than by raw benchmark row text

### Requirement: Textual benchmark report evolution
The Markdown benchmark report SHALL include textual performance evolution summaries generated from canonical benchmark data and available history.

#### Scenario: Previous release comparison is available
- **WHEN** a generated report has compatible previous-release history
- **THEN** the Markdown report SHALL include previous-release comparisons for relevant startup and Go benchmark metrics with direction, absolute value change, and percentage change

#### Scenario: Last 3 months and last 1 year summaries are available
- **WHEN** historical data exists inside the last 3 months or last 1 year
- **THEN** the Markdown report SHALL summarize largest improvements, largest regressions, and notable unchanged metrics for each available window

#### Scenario: History is insufficient
- **WHEN** a report is generated without enough history for a comparison window
- **THEN** the Markdown report SHALL include an explicit insufficient-history note rather than omitting the evolution section silently

#### Scenario: Environment changes affect interpretation
- **WHEN** compared benchmark records differ in Go version, CPU model, runner, OS, architecture, benchmark mode, or sample policy
- **THEN** the Markdown report SHALL annotate the comparison with environment notes and avoid presenting the comparison as fully compatible

### Requirement: Static graphical release summary
The benchmark report pipeline SHALL generate a deterministic static SVG summary for each release benchmark report.

#### Scenario: SVG summary is generated
- **WHEN** release benchmark assets are staged
- **THEN** the system SHALL stage an SVG summary asset derived from the canonical JSON and available history

#### Scenario: SVG summary includes curated performance signals
- **WHEN** the SVG summary is generated
- **THEN** it SHALL include a curated set of user-facing startup metrics and developer-facing benchmark metrics with clear labels, time direction, and lower-is-better semantics

#### Scenario: SVG output is valid and non-empty
- **WHEN** benchmark graphical asset validation runs
- **THEN** it SHALL verify the SVG exists, is non-empty, is parseable as SVG text, and contains at least one rendered data series or explicit insufficient-history state

### Requirement: Release benchmark asset publishing
The release pipeline SHALL publish benchmark Markdown, JSON, and SVG assets for every release and preserve manual fallback support.

#### Scenario: Main release publishes all benchmark assets
- **WHEN** the main release workflow publishes a release
- **THEN** GoReleaser SHALL include the benchmark Markdown report, canonical JSON artifact, and SVG summary as release assets

#### Scenario: Release dry run exercises benchmark assets
- **WHEN** release dry-run CI runs for a pull request or integration branch
- **THEN** it SHALL stage and validate all expected benchmark asset types, including Markdown, JSON, and SVG

#### Scenario: Release dry run uses committed benchmark history
- **WHEN** release dry-run CI stages benchmark report assets and committed benchmark history data exists
- **THEN** the generated benchmark report SHALL render with that committed history data and include a performance evolution section

#### Scenario: Manual fallback uploads all benchmark assets
- **WHEN** the release benchmark fallback workflow is dispatched for an existing release
- **THEN** it SHALL generate, upload, and verify all expected benchmark asset types for that tag

#### Scenario: Release and fallback staging use the same history input
- **WHEN** main release, release dry-run, or manual fallback benchmark staging runs in GitHub Actions
- **THEN** each workflow SHALL pass the committed benchmark history data path explicitly to the staging script

#### Scenario: Asset set is incomplete
- **WHEN** release staging finds missing, duplicate, or unexpected benchmark assets
- **THEN** staging SHALL fail before publication with an actionable error listing the discovered assets

### Requirement: Historical data aggregation
The system SHALL aggregate benchmark history from release assets into static data suitable for website rendering and report comparison generation.

#### Scenario: JSON release assets are aggregated
- **WHEN** the history aggregation command runs
- **THEN** it SHALL fetch or read benchmark JSON release assets, validate their schema, normalize their metrics, and emit aggregate history data

#### Scenario: Legacy Markdown reports are backfilled
- **WHEN** a release has a Markdown benchmark report but no JSON benchmark artifact
- **THEN** the aggregation command SHALL attempt legacy Markdown parsing, mark the resulting records as `legacy-markdown`, and include warnings about reduced confidence

#### Scenario: Bad release asset is encountered
- **WHEN** an asset cannot be fetched, parsed, or validated
- **THEN** aggregation SHALL record an actionable warning and continue only when policy allows partial history; otherwise it SHALL fail

#### Scenario: Aggregate history supports required windows
- **WHEN** aggregate history is emitted
- **THEN** it SHALL contain enough normalized data for last-3-months, last-1-year, and all-history views

### Requirement: Interactive website performance history
The website SHALL expose an interactive performance history view backed by static aggregate benchmark data.

#### Scenario: User opens performance history page
- **WHEN** a visitor opens the performance history page
- **THEN** the page SHALL render benchmark trend charts, summary cards, and tables from static aggregate history data without requiring a backend service

#### Scenario: Visitor selects time window
- **WHEN** a visitor selects last 3 months, last 1 year, or all history
- **THEN** charts and tables SHALL update to show only data in that window

#### Scenario: Visitor switches audience view
- **WHEN** a visitor selects user-facing or developer-facing metrics
- **THEN** the page SHALL show curated startup metrics for users and deeper Go benchmark, allocation, and memory metrics for developers

#### Scenario: Visitor switches metric mode
- **WHEN** a visitor selects time, allocations, memory, absolute values, or indexed values
- **THEN** the page SHALL update charts and tables using the selected metric mode while preserving lower-is-better labeling

#### Scenario: Website data is unavailable
- **WHEN** aggregate history data is missing or contains no records
- **THEN** the page SHALL render a clear empty state that explains no benchmark history is available yet

### Requirement: Benchmark trend interpretation
The system SHALL communicate benchmark trend confidence, compatibility, and environment changes in both textual and graphical views.

#### Scenario: Environment annotations are present
- **WHEN** a chart or report compares records with different Go versions, CPU models, runner labels, operating systems, architectures, benchmark modes, or sample policies
- **THEN** the view SHALL annotate those changes near the affected comparison or data point

#### Scenario: Confidence is limited
- **WHEN** a comparison uses legacy data, partial data, incompatible environments, or too few historical points
- **THEN** the view SHALL label the comparison with a reduced-confidence note

#### Scenario: Lower-is-better semantics are visible
- **WHEN** a chart or table shows benchmark duration, allocation count, or memory usage
- **THEN** the view SHALL make clear that lower values are better

### Requirement: Documentation for benchmark history
The documentation SHALL explain benchmark report artifacts, performance history views, and interpretation rules.

#### Scenario: User reads release artifact documentation
- **WHEN** a user reads performance or release documentation
- **THEN** the documentation SHALL identify the Markdown, JSON, SVG, and optional raw benchmark assets and describe what each is for

#### Scenario: User reads trend interpretation documentation
- **WHEN** a user reads benchmark history documentation
- **THEN** the documentation SHALL explain last-3-months, last-1-year, all-history views, indexed values, absolute values, environment annotations, confidence notes, and common noise sources

#### Scenario: Developer reads maintenance documentation
- **WHEN** a maintainer reads benchmark-history maintenance documentation
- **THEN** it SHALL explain how to regenerate history, backfill legacy releases, validate assets, update fixtures, and troubleshoot release/fallback failures

### Requirement: Validation and regression coverage
The benchmark history system SHALL include automated validation and tests for data generation, rendering, aggregation, release staging, and website rendering.

#### Scenario: Script tests validate artifact generation
- **WHEN** benchmark report tests run
- **THEN** they SHALL verify canonical JSON generation, Markdown rendering, SVG rendering, stable benchmark identity normalization, warnings, and failure modes

#### Scenario: History fixtures validate aggregation
- **WHEN** history aggregation tests run
- **THEN** they SHALL validate release JSON fixtures, legacy Markdown fixtures, partial history behavior, and windowed trend output

#### Scenario: CI validates the benchmark history contract
- **WHEN** pull-request or push CI runs
- **THEN** a GitHub Actions-compatible benchmark-history contract SHALL validate script tests, committed history data, history-aware release staging, and staged asset content with low sample counts

#### Scenario: CI validates staged benchmark asset contents
- **WHEN** the benchmark-history contract stages release benchmark assets
- **THEN** CI SHALL verify the release-named Markdown, JSON, SVG, and raw benchmark assets exist, are non-empty, and have valid benchmark-report content

#### Scenario: Website checks validate performance page
- **WHEN** website typecheck and build validation run
- **THEN** they SHALL include the performance history page, chart components, static history data imports, and empty-state behavior

#### Scenario: Website build ships benchmark history surfaces
- **WHEN** the website build completes in CI
- **THEN** CI SHALL verify the built output includes the performance page and static benchmark history JSON

#### Scenario: Agent documentation changes are checked
- **WHEN** this change modifies `AGENTS.md`, `.agents/rules/`, or `.agents/skills/`
- **THEN** `make check-agent-docs` SHALL pass before the change is considered complete
