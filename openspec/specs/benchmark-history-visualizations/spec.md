## Purpose
Benchmark tracking provides release-version performance evidence for Invowk users and regression checks for maintainers. Bencher is the durable source of truth; local benchmark output is ephemeral Bencher Metric Format JSON used for upload.

## Requirements

### Requirement: Canonical Bencher Metric Format payload
The benchmark pipeline SHALL emit Bencher Metric Format JSON containing Invowk release and maintainer performance metrics.

#### Scenario: BMF payload is generated
- **WHEN** the benchmark data command runs
- **THEN** it SHALL write a valid BMF JSON object containing startup latency, Go benchmark latency, Go benchmark memory usage, Go benchmark allocation counts, build time, and binary size

#### Scenario: Stable Go benchmark identity is preserved
- **WHEN** raw Go benchmark output contains a row such as `BenchmarkCUEParsing-24`
- **THEN** the BMF emitter SHALL use `go/BenchmarkCUEParsing` as the benchmark identity so CPU-count suffixes do not split historical series

#### Scenario: Lower and upper values preserve spread
- **WHEN** multiple samples exist for a metric
- **THEN** the BMF metric SHALL set `value` to the mean, `lower_value` to the minimum, and `upper_value` to the maximum when those values are available

#### Scenario: Invalid BMF fails validation
- **WHEN** a generated BMF payload is empty, malformed, or contains non-finite metric values
- **THEN** validation SHALL fail with an actionable error

### Requirement: Release-version performance history
The release pipeline SHALL publish benchmark results to Bencher using the release tag as the durable public identity.

#### Scenario: Release tag is tracked
- **WHEN** a release workflow runs for tag `vX.Y.Z`
- **THEN** Bencher SHALL receive a report with `--branch vX.Y.Z` and `--hash` set to the release commit SHA

#### Scenario: Release history starts from the previous version
- **WHEN** a previous release tag exists
- **THEN** the release Bencher run SHALL use the previous tag as `--start-point`, set `--start-point-hash`, clone thresholds, and reset the release branch from that start point

#### Scenario: First release can be tracked
- **WHEN** no previous release tag exists
- **THEN** the release Bencher run SHALL omit start-point arguments and still upload the BMF payload

#### Scenario: Release performance is required
- **WHEN** the release workflow cannot generate or upload Bencher performance data
- **THEN** the release workflow SHALL fail before publishing the release

### Requirement: Pull request regression checks
Pull request benchmark runs SHALL be maintainer guardrails and SHALL NOT be the public release history.

#### Scenario: Trusted pull request uploads directly
- **WHEN** a pull request comes from the base repository
- **THEN** CI SHALL run benchmarks, upload BMF data to Bencher with branch `pr-<number>`, start from the PR base branch, and fail on Bencher alerts

#### Scenario: Fork pull request keeps secrets safe
- **WHEN** a pull request comes from a fork
- **THEN** CI SHALL generate and upload only a BMF artifact from the untrusted workflow, and a separate `workflow_run` job SHALL upload that artifact to Bencher without executing forked code with secrets

#### Scenario: Main branch baseline is tracked
- **WHEN** changes land on `main`
- **THEN** CI SHALL upload BMF data to Bencher on branch `main`

### Requirement: GitHub Actions integration
Benchmark tracking SHALL be integrated with GitHub Actions without relying on committed release benchmark assets.

#### Scenario: Supported GitHub-hosted runner is pinned
- **WHEN** benchmark workflows run on GitHub-hosted runners
- **THEN** they SHALL use a pinned supported Ubuntu runner label and a matching Bencher testbed name

#### Scenario: Runner can be upgraded
- **WHEN** GitHub publishes an `ubuntu-26.04` runner label
- **THEN** maintainers SHALL be able to upgrade benchmark jobs by changing the runner label and Bencher testbed name without changing the BMF schema

#### Scenario: Bencher credentials are secret
- **WHEN** GitHub Actions uploads benchmark data to Bencher
- **THEN** it SHALL read the API token from `BENCHER_API_TOKEN` and SHALL NOT store the token in repository files

### Requirement: Website and documentation
The website SHALL point users to release-version performance history in Bencher.

#### Scenario: User opens performance page
- **WHEN** a visitor opens the performance page
- **THEN** the page SHALL explain that release performance is tracked in Bencher by release tag and link to the Bencher performance page

#### Scenario: Maintainer reads benchmark documentation
- **WHEN** a maintainer reads benchmark history documentation
- **THEN** it SHALL explain `make bench-bmf`, the BMF output path, the release-tag Bencher branch model, and pull-request regression guardrails

### Requirement: Validation and regression coverage
The benchmark tracking system SHALL include automated tests and local end-to-end checks for BMF generation and CI wiring.

#### Scenario: Script tests validate BMF generation
- **WHEN** script tests run
- **THEN** they SHALL verify Go benchmark parsing, CPU suffix normalization, BMF validation failures, and end-to-end BMF emission from fixture inputs

#### Scenario: Local benchmark generation is executable
- **WHEN** maintainers run the benchmark target locally
- **THEN** `make bench-bmf` SHALL build the binary, run startup and Go benchmarks, and write a valid BMF file

#### Scenario: CI no longer validates obsolete assets
- **WHEN** pull-request or push CI runs
- **THEN** it SHALL validate the BMF emitter and Bencher workflows rather than Markdown, JSON, SVG, raw release assets, or committed static history JSON
