## MODIFIED Requirements

### Requirement: Upgrade inventory is refreshed before edits
Invowk SHALL refresh dependency and tooling inventory before applying a repo-wide upgrade batch.

#### Scenario: Inventory covers every dependency surface
- **WHEN** maintainers start a repo-wide dependency and tooling upgrade
- **THEN** they MUST inventory the root Go module (including the pinned `github.com/invowk/goplint` tool dependency), website npm dependencies, website npm advisories, Go tool pins, workflow tool installs, MCP server pins, GitHub Actions pins, release tooling pins, and Node.js workflow pins
- **THEN** inventory output MUST identify available updates, deprecated modules, retracted modules, vulnerabilities, and tooling-policy exceptions

#### Scenario: Inventory failures are visible
- **WHEN** an inventory command fails for a module, package manager, registry, or workflow surface
- **THEN** maintainers MUST report that surface as incomplete evidence
- **THEN** they MUST NOT claim the dependency graph is fully current until that surface is checked or explicitly deferred
