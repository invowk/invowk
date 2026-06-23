## ADDED Requirements

### Requirement: Mutation tool upgrades preserve PR targeting semantics
Invowk SHALL keep changed-line mutation testing aligned with pull request review semantics when the pinned mutation tool gains improved diff-base behavior.

#### Scenario: Changed-line mutation uses the intended base
- **WHEN** maintainers update the pinned mutation-testing tool to a version that changes changed-line diff handling
- **THEN** the pull request mutation profile MUST continue mutating eligible production Go lines relative to the intended PR base
- **THEN** wrapper tests or focused dry-run evidence MUST prove the selected base-ref behavior is still correct

#### Scenario: Baselines are not recomputed by tool refresh
- **WHEN** maintainers update the pinned mutation-testing tool as part of dependency maintenance
- **THEN** accepted-survivor baselines MUST NOT be regenerated unless the change explicitly enters the baseline update profile
- **THEN** any baseline change in the same implementation MUST be justified as intentional survivor triage rather than routine tool refresh

### Requirement: Mutation tool upgrades preserve report contracts
Invowk SHALL preserve machine-readable mutation reports and stable mutant identifiers across routine mutation-testing tool upgrades.

#### Scenario: Report files remain available
- **WHEN** mutation wrapper tests run after a mutation tool upgrade
- **THEN** expected summary, agentic, GitLab, HTML, log, target-resolution, excluded-package, and not-covered-package report paths MUST remain stable unless the design documents a migration

#### Scenario: Automation avoids terminal-label scraping
- **WHEN** mutation terminal labels change across tool versions
- **THEN** automation MUST continue using machine-readable report fields or stable mutant IDs instead of parsing human terminal labels
- **THEN** human-facing documentation MUST identify the current terminal labels and preserve historical label notes as version-scoped evidence
