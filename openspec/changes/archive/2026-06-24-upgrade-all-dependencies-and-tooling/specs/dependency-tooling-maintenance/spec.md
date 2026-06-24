## ADDED Requirements

### Requirement: Upgrade inventory is refreshed before edits
Invowk SHALL refresh dependency and tooling inventory before applying a repo-wide upgrade batch.

#### Scenario: Inventory covers every dependency surface
- **WHEN** maintainers start a repo-wide dependency and tooling upgrade
- **THEN** they MUST inventory the root Go module, the `tools/goplint` Go module, website npm dependencies, website npm advisories, Go tool pins, workflow tool installs, MCP server pins, GitHub Actions pins, release tooling pins, and Node.js workflow pins
- **THEN** inventory output MUST identify available updates, deprecated modules, retracted modules, vulnerabilities, and tooling-policy exceptions

#### Scenario: Inventory failures are visible
- **WHEN** an inventory command fails for a module, package manager, registry, or workflow surface
- **THEN** maintainers MUST report that surface as incomplete evidence
- **THEN** they MUST NOT claim the dependency graph is fully current until that surface is checked or explicitly deferred

### Requirement: Security fixes are prioritized without unsafe graph swaps
Invowk SHALL prioritize dependency upgrades that address reachable or reported security advisories while preserving tested dependency boundaries.

#### Scenario: Go vulnerability scan is clean
- **WHEN** Go dependency updates are complete
- **THEN** the repository vulnerability-scan command MUST report no reachable vulnerabilities for every tracked Go module

#### Scenario: Website advisories are reduced safely
- **WHEN** website npm advisories have compatible transitive fixes available
- **THEN** maintainers MUST refresh the lockfile or selected transitive packages to reduce those advisories
- **THEN** they MUST NOT accept a direct package downgrade, unsafe major replacement, or broad override without documenting why it is required and verifying the website build

#### Scenario: Remaining advisories are documented
- **WHEN** an npm advisory remains because an upstream package has not released a compatible fix or the safe fix would require a separate migration
- **THEN** maintainers MUST document the advisory, affected package path, blocker, and deferred follow-up

### Requirement: Go module upgrades are explicit and tidy
Invowk SHALL apply Go dependency updates through explicit module selections and keep each Go module tidy.

#### Scenario: Root direct dependencies are upgraded explicitly
- **WHEN** root Go direct dependencies have available compatible updates
- **THEN** each selected root direct dependency MUST be named in the implementation command sequence or summary
- **THEN** root `go.mod` and `go.sum` MUST remain tidy after the update

#### Scenario: Nested goplint dependencies are upgraded explicitly
- **WHEN** `tools/goplint` direct dependencies have available compatible updates
- **THEN** each selected nested-module direct dependency MUST be named in the implementation command sequence or summary
- **THEN** `tools/goplint/go.mod` and `tools/goplint/go.sum` MUST remain tidy after the update

#### Scenario: Broad transitive churn is justified
- **WHEN** maintainers apply a broad transitive Go update beyond what selected direct or tool updates require
- **THEN** the implementation summary MUST justify the broad update
- **THEN** verification MUST include tests for the affected product or tool surfaces

### Requirement: Website dependency updates preserve documentation behavior
Invowk SHALL keep the documentation website behavior stable while refreshing website dependencies and lockfiles.

#### Scenario: Direct website dependencies are already current
- **WHEN** direct website dependencies are current but transitive advisories remain
- **THEN** maintainers MUST prefer lockfile or transitive refreshes over direct dependency churn
- **THEN** direct `website/package.json` changes MUST be limited to necessary compatible upgrades or explicitly justified migrations

#### Scenario: Website validation passes
- **WHEN** website dependency or lockfile updates are complete
- **THEN** `npm --prefix website ci` MUST install from the resulting lockfile
- **THEN** website typecheck and build commands MUST pass before the website dependency phase is complete

### Requirement: Tooling and workflow pins stay synchronized
Invowk SHALL update all synchronized tooling and workflow version references together.

#### Scenario: Tool sync pairs are updated together
- **WHEN** a pinned tool version changes
- **THEN** every workflow install, wrapper expectation, cache key, rule, command reference, and documentation entry for that tool MUST be updated in the same change phase

#### Scenario: GitHub Actions major pins are consistent
- **WHEN** a shared GitHub Action is moved to a newer major version
- **THEN** every workflow invocation of that shared action MUST use the same major version unless a documented compatibility exception requires otherwise

#### Scenario: Node LTS policy is preserved
- **WHEN** Node.js workflow pins are reviewed during the upgrade
- **THEN** Node.js MUST remain on the repository-approved active LTS major unless the design records a policy-approved reason to move

### Requirement: Upgrade results include deferrals and verification evidence
Invowk SHALL finish repo-wide upgrade work with explicit evidence of what changed, what remained, and what was verified.

#### Scenario: Final summary records versions
- **WHEN** the upgrade implementation is complete
- **THEN** the implementation summary MUST list the final upgraded versions or version tracks for each dependency and tooling surface touched

#### Scenario: Final summary records deferrals
- **WHEN** deprecated modules, advisories, branch-pinned actions, or unavailable updates remain after implementation
- **THEN** the implementation summary MUST list each remaining finding with its reason and recommended follow-up

#### Scenario: Final summary records verification
- **WHEN** the upgrade implementation is complete
- **THEN** the implementation summary MUST record the local and remote verification commands or checks that passed
- **THEN** any skipped verification MUST be called out with the reason and residual risk
