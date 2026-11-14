## MODIFIED Requirements

### Requirement: CI time budgets
Each heavy formal step SHALL have a hard budget, set as a step-level `timeout-minutes`: model checks, trace validation, replay, binding reach, and deep property tests. Each hard budget SHALL be about four times the maximum measured CI time for the final model set, rounded up to whole minutes. The job-level `timeout-minutes` SHALL exceed the sum of the step `timeout-minutes` values plus a setup margin of at least 3 minutes, so a step timeout always fires before the job timeout.

#### Scenario: Hard budget exceeded
- **WHEN** a formal step runs longer than its step `timeout-minutes`
- **THEN** GitHub Actions SHALL fail the step and the job
- **THEN** the upload step, conditioned on `failure() || cancelled()`, SHALL upload `artifacts/formal/`

#### Scenario: Job timeout covers the steps
- **WHEN** `scripts/test_formal.py` runs
- **THEN** it SHALL fail if the formal job's `timeout-minutes` is not greater than the sum of its step `timeout-minutes` plus 3

#### Scenario: Binding reach before it is measured in CI
- **WHEN** the `Binding reach` step has no measured CI time yet
- **THEN** its `timeout-minutes` SHALL be `max(2, ceil(4 × 2 × local seconds / 60))`, assuming CI is at most twice as slow as the local measurement, until three CI runs replace it
