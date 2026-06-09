## ADDED Requirements

### Requirement: All Go modules are vulnerability-scanned
Invowk SHALL provide a vulnerability-scan path that checks every Go module tracked by the repository, including nested tool modules.

#### Scenario: Local vulnerability scan covers nested modules
- **WHEN** maintainers run the repository vulnerability-scan command
- **THEN** the command MUST run `govulncheck ./...` from the root Go module
- **AND** the command MUST run `govulncheck ./...` from the `tools/goplint` Go module

#### Scenario: CI vulnerability scan covers nested modules
- **WHEN** the CI vulnerability scanning job runs
- **THEN** it MUST scan every tracked Go module discovered by the repository's shared module discovery logic
- **AND** a vulnerability in a nested Go module MUST fail the job with the affected module path visible in logs

### Requirement: Security dependency updates are prioritized and bounded
Invowk SHALL address reachable dependency vulnerabilities with the smallest dependency update set that fixes the finding before applying unrelated dependency churn.

#### Scenario: Nested goplint vulnerability is fixed
- **WHEN** `tools/goplint` depends on a vulnerable `golang.org/x/net` version with a fixed version available
- **THEN** the nested module MUST select a fixed `golang.org/x/net` version
- **AND** `govulncheck ./...` from `tools/goplint` MUST report no reachable vulnerabilities before the change is complete

#### Scenario: Unrelated transitive churn is avoided
- **WHEN** maintainers fix a reachable dependency vulnerability
- **THEN** they MUST NOT use a broad transitive update that changes unrelated tool or product dependencies unless the design explicitly justifies that expanded scope

### Requirement: Direct dependency refreshes are explicit and verified
Invowk SHALL refresh direct Go dependency updates through explicit module selections and validate the behavior surfaces those dependencies affect.

#### Scenario: Root direct dependency batch is applied
- **WHEN** maintainers refresh root direct dependency versions
- **THEN** each direct dependency selected for upgrade MUST be named explicitly in the implementation or command sequence
- **AND** root `go.mod` and `go.sum` MUST remain tidy after the update

#### Scenario: Affected surfaces are verified
- **WHEN** a direct dependency update affects TUI, ACP, testscript, terminal, or platform behavior
- **THEN** maintainers MUST run focused tests or documented checks for the affected surface before marking the update complete

### Requirement: Deferred dependency findings remain visible
Invowk SHALL keep dependency-audit findings visible when they are intentionally deferred to separate migrations or tool updates.

#### Scenario: Major module-path migrations are deferred
- **WHEN** an audit finds a newer major module path for a dependency used by Invowk
- **THEN** maintainers MUST either include that migration in the current change or record it as a separate explicit workstream

#### Scenario: Deprecated transitive modules are not hidden
- **WHEN** an audit finds deprecated transitive modules that are not removed by the bounded update set
- **THEN** maintainers MUST report those modules as deferred findings rather than claiming the dependency graph is fully clean
