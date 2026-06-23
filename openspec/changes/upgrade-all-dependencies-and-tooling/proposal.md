## Why

The dependency exploration found no reachable Go vulnerabilities, but it did find upgradeable Go modules, a red website `npm audit` surface, newer mutation and CI tooling pins, and newer GitHub Actions/MCP/tool releases. A single coordinated maintenance change is needed to upgrade every eligible dependency and tooling pin while keeping security fixes, product dependencies, release tooling, and CI major-version changes independently verifiable.

## What Changes

- Refresh every currently upgradeable direct Go dependency in the root module and the nested `tools/goplint` module, including `openai-go/v3`, `lipgloss/v2`, `sahilm/fuzzy`, and `golang.org/x/tools`.
- Apply safe transitive Go updates when they naturally follow from direct/tool updates, while avoiding blind `go get -u all` churn unless the implementation design explicitly justifies it.
- Refresh the website npm lockfile to address currently fixable transitive advisories reported by `npm audit`, including Babel, `shell-quote`, `ws`, `undici`, `fast-uri`, `qs`, and related dev-server/tooling dependencies.
- Update pinned repository tools and CI infrastructure where newer versions exist, including `govulncheck`, `cosign`, UPX, Context7 MCP, selected GitHub Actions major pins, and other workflow-managed tools discovered during inventory.
- Update `go-mutesting` beyond the current `v2.7.1` label-update track, preserving machine-readable reports, stable mutant IDs, and mutation baseline semantics.
- Review release-tooling pins and validation around GoReleaser, Cosign, UPX, signing, dry-run packaging, and release workflow consistency.
- Keep Node.js on the policy-supported LTS line unless the implementation audit finds a project-approved reason to move to a newer major before LTS.
- Document any dependency findings that cannot be eliminated through available compatible upgrades, including deprecated transitive modules or upstream-blocked npm advisories.
- No intentional user-facing CLI, CUE schema, runtime, module, audit, TUI, or website-content behavior changes are expected.

## Capabilities

### New Capabilities

- `dependency-tooling-maintenance`: Defines the contract for repo-wide dependency and tooling refreshes across Go modules, website npm packages, CI tools, MCP servers, GitHub Actions, lockfiles, audit evidence, and verification.

### Modified Capabilities

- `mutation-testing`: Update the mutation-testing contract to account for newer `go-mutesting` behavior after `v2.7.1`, especially changed-line PR semantics and version synchronization.
- `release-tooling-maintenance`: Expand release-tooling maintenance requirements for Cosign, UPX, and release workflow pin updates in addition to the existing GoReleaser track.

## Impact

- Root Go module: `go.mod`, `go.sum`, Go tool directives, and root affected test surfaces.
- Nested Go module: `tools/goplint/go.mod`, `tools/goplint/go.sum`, analyzer tests, and goplint gates.
- Website: `website/package-lock.json`, possibly `website/package.json` only if lockfile refresh cannot address advisories safely.
- CI/workflows: `.github/workflows/*.yml`, `.mcp.json`, `.agents/rules/version-pinning.md`, `.agents/rules/commands.md`, and related script/version checks.
- Mutation tooling: `scripts/mutation.sh`, `scripts/test_mutation.sh`, mutation docs, and current mutation OpenSpec artifacts if their version assumptions need follow-up.
- Release tooling: release and CI dry-run workflows, signing/compression setup, GoReleaser checks, and release validation commands.
