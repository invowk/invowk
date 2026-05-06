## MODIFIED Requirements

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
