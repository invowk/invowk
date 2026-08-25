## MODIFIED Requirements

### Requirement: All Go modules are vulnerability-scanned
Invowk SHALL provide a vulnerability-scan path that checks every Go module tracked by the repository through shared module discovery, and SHALL cover the pinned goplint tool dependency through the root module graph.

#### Scenario: Local vulnerability scan covers all tracked modules
- **WHEN** maintainers run the repository vulnerability-scan command
- **THEN** the command MUST run `govulncheck ./...` from every Go module discovered by the repository's shared module discovery logic (the root module after the goplint extraction)
- **AND** the pinned `github.com/invowk/goplint` tool dependency and its graph MUST be visible to the root-module scan

#### Scenario: CI vulnerability scan covers all tracked modules
- **WHEN** the CI vulnerability scanning job runs
- **THEN** it MUST scan every tracked Go module discovered by the repository's shared module discovery logic
- **AND** a vulnerability in any tracked module MUST fail the job with the affected module path visible in logs
