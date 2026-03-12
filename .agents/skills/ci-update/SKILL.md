---
name: ci-update
description: Audit and update CI workflow versions, tool installs, MCP servers, and pre-commit hooks. Checks latest versions, validates sync pair consistency, identifies breaking changes and optimization opportunities. Use whenever preparing a release, doing monthly CI maintenance, or investigating version-related CI failures.
user-invocable: true
disable-model-invocation: true
---

# CI Update

Audit and update all CI infrastructure versions: GitHub Actions, `go install` tools, MCP servers, inline binary installs, pre-commit hooks, and Node.js LTS status.

## When to Use

Invoke this skill (`/ci-update`) when:
- Preparing a release and want to ensure CI is on latest stable tooling
- Monthly maintenance to keep CI infrastructure current
- After a tool announces a new major version or deprecation
- Investigating CI failures that might be caused by stale tool versions
- After merging a Dependabot PR (to catch cascading sync needs)

## Authoritative Rules

This skill operates under `.agents/rules/version-pinning.md`. Key constraints:
- All external dependencies MUST use pinned versions (never `@latest`)
- GitHub Actions pin to major version tags; Dependabot handles minor/patch
- `go install` tools, MCP servers, and inline binaries require **manual** updates
- When upgrading, update ALL references simultaneously (see Sync Pairs below)

## Sync Pairs (must be updated together)

| Component | Files |
|-----------|-------|
| `golangci-lint` | `.github/workflows/lint.yml` (`version` input), `.pre-commit-config.yaml` (`rev`) |
| `gotestsum` | `.github/workflows/ci.yml`, `.github/workflows/release.yml` |
| `GoReleaser` | `.github/workflows/ci.yml` (`version` input), `.github/workflows/release.yml` (two `goreleaser-action` steps) |
| `Node.js` | `.github/workflows/deploy-website.yml`, `.github/workflows/test-website.yml`, `.github/workflows/version-docs.yml` |
| Sonar suppressions | `sonar-project.properties`, `.sonarcloud.properties` (not version, but integrity) |

## Workflow

### Step 1: Build Complete Version Inventory

Scan all tracked files and extract every version pin into a structured table.

**Workflow files** (scan for `uses:`, `go install`, `curl` installs, `version:` inputs):
- `.github/workflows/ci.yml`
- `.github/workflows/lint.yml`
- `.github/workflows/release.yml`
- `.github/workflows/pgo-benchstat.yml`
- `.github/workflows/deploy-website.yml`
- `.github/workflows/test-website.yml`
- `.github/workflows/version-docs.yml`
- `.github/workflows/validate-diagrams.yml`
- `.github/workflows/release-benchmark-asset.yml`
- `.github/workflows/claude.yml`
- `.github/workflows/claude-code-review.yml`

**Configuration files**:
- `.pre-commit-config.yaml` (golangci-lint `rev`)
- `.mcp.json` (MCP server versions)
- `.github/dependabot.yml` (verify coverage)
- `.goreleaser.yaml` (GoReleaser schema version)

**Documentation** (source of truth for documented versions):
- `.agents/rules/version-pinning.md` ("Current pinned versions")
- `.agents/rules/commands.md` (Prerequisites)

Organize into this table structure:

| Category | Tool | Current Version | Location(s) |
|----------|------|-----------------|-------------|
| Go tool | gotestsum | vX.Y.Z | ci.yml, release.yml |
| Go tool | govulncheck | vX.Y.Z | ci.yml |
| Go tool | benchstat | v0.0.0-... | pgo-benchstat.yml |
| Lint | golangci-lint | vX.Y.Z | lint.yml, .pre-commit-config.yaml |
| Binary | UPX | X.Y.Z | release.yml |
| Binary | D2 | vX.Y.Z | validate-diagrams.yml |
| Binary | Cosign | vX.Y.Z | release.yml |
| Range | GoReleaser | ~> vX.Y | ci.yml, release.yml |
| Runtime | Node.js | N | deploy-website.yml, test-website.yml, version-docs.yml |
| MCP | context7-mcp | X.Y.Z | .mcp.json |
| MCP | server-github | YYYY.M.D | .mcp.json |
| Action | (each `uses:`) | @vN | workflow files |

### Step 2: Check Sync Pair Consistency

Before checking for updates, verify existing versions are consistent across sync pairs:

1. **golangci-lint**: Version in `lint.yml` `golangci-lint-action` `version` input must match `.pre-commit-config.yaml` `rev` field.
2. **gotestsum**: Version in `ci.yml` must match `release.yml`.
3. **GoReleaser version input**: Must match across `ci.yml` and `release.yml` (the latter has two goreleaser-action steps).
4. **Node.js**: `node-version` must match across `deploy-website.yml`, `test-website.yml`, `version-docs.yml`.
5. **version-pinning.md accuracy**: Compare every "Current pinned versions" entry against actual workflow files. Flag any drift.

Report all inconsistencies as **SYNC DRIFT** findings before proceeding.

### Step 3: Check Latest Versions

For each item in the inventory, check the latest available version. Process by category:

#### 3a: Go Tools (`go install` packages)

```bash
# gotestsum — check latest tag
go list -m -versions gotest.tools/gotestsum 2>/dev/null | tr ' ' '\n' | grep '^v' | tail -1

# govulncheck — check latest tag (module is golang.org/x/vuln)
go list -m -versions golang.org/x/vuln 2>/dev/null | tr ' ' '\n' | grep '^v' | tail -1

# benchstat — check if a proper tagged release exists (module is golang.org/x/perf)
go list -m -versions golang.org/x/perf 2>/dev/null | tr ' ' '\n' | grep '^v' | tail -1
```

If `go list -m` does not return results (tool not in go.mod), use WebFetch as fallback:
- `https://pkg.go.dev/gotest.tools/gotestsum?tab=versions`
- `https://pkg.go.dev/golang.org/x/vuln?tab=versions`
- `https://pkg.go.dev/golang.org/x/perf?tab=versions`

**benchstat special case**: If still on a pseudo-version, check whether `golang.org/x/perf` has a tagged release. If yes, flag as upgrade opportunity. If no, note pseudo-version is expected.

#### 3b: Binary Tools (curl-installed)

```bash
# UPX
gh api repos/upx/upx/releases/latest --jq '.tag_name'

# D2
gh api repos/terrastruct/d2/releases/latest --jq '.tag_name'

# Cosign
gh api repos/sigstore/cosign/releases/latest --jq '.tag_name'
```

#### 3c: GitHub Actions

Check whether any action has a newer **major** version (minor/patch are Dependabot's job):

```bash
gh api repos/actions/checkout/releases/latest --jq '.tag_name'
gh api repos/actions/setup-go/releases/latest --jq '.tag_name'
gh api repos/actions/setup-node/releases/latest --jq '.tag_name'
gh api repos/actions/upload-artifact/releases/latest --jq '.tag_name'
gh api repos/actions/configure-pages/releases/latest --jq '.tag_name'
gh api repos/actions/upload-pages-artifact/releases/latest --jq '.tag_name'
gh api repos/actions/deploy-pages/releases/latest --jq '.tag_name'
gh api repos/actions/create-github-app-token/releases/latest --jq '.tag_name'
gh api repos/golangci/golangci-lint-action/releases/latest --jq '.tag_name'
gh api repos/goreleaser/goreleaser-action/releases/latest --jq '.tag_name'
gh api repos/sigstore/cosign-installer/releases/latest --jq '.tag_name'
gh api repos/mikepenz/action-junit-report/releases/latest --jq '.tag_name'
gh api repos/anthropics/claude-code-action/releases/latest --jq '.tag_name'
```

Only flag findings where the **major version** changed. Minor/patch within the same major are handled by Dependabot.

#### 3d: MCP Servers (npm packages)

```bash
npm view @upstash/context7-mcp version 2>/dev/null
npm view @modelcontextprotocol/server-github version 2>/dev/null
```

If `npm` is not available, use WebFetch against the npm registry:
- `https://registry.npmjs.org/@upstash/context7-mcp/latest`
- `https://registry.npmjs.org/@modelcontextprotocol/server-github/latest`

#### 3e: GoReleaser Version Track

Check the latest GoReleaser v2 release:

```bash
gh api repos/goreleaser/goreleaser/releases/latest --jq '.tag_name'
```

If the latest is newer than the current range floor (e.g., `v2.18` vs `~> v2.14`), check the changelog for v2.x breaking changes and recommend whether to widen the range.

#### 3f: Node.js LTS Status

Verify the pinned Node.js major version is still in Active LTS. Use WebFetch:

```
https://endoflife.date/api/nodejs.json
```

If the current pin is approaching End of Life, flag the next LTS version as upgrade target.

### Step 4: Breaking Change Analysis

For each item where the latest version differs from the current, classify the risk:

| Risk | Criteria |
|------|----------|
| **Safe** | Patch bump, or minor bump with no noted breaking changes |
| **Low** | Minor bump with new features but backward-compatible |
| **Medium** | New major version with documented migration path |
| **High** | New major version with significant API changes or removed features |

For Medium/High risk items, use WebFetch to read the release notes or CHANGELOG from the project's GitHub repository. Summarize breaking changes relevant to our usage.

### Step 5: CI Optimization Scan

While reviewing workflow files, check for these optimization opportunities:

1. **Caching**: Are all `setup-go` steps using `cache: true`? npm caching configured?
2. **Concurrency groups**: Are long-running workflows using `concurrency` to cancel stale runs?
3. **Timeout guards**: Do ALL jobs have explicit `timeout-minutes`?
4. **Permissions**: Are all jobs using least-privilege, job-level permissions?
5. **Matrix deduplication**: Any redundant matrix entries?
6. **New action features**: Have `actions/setup-go`, `golangci-lint-action`, or `goreleaser-action` added useful new inputs since the current pin?
7. **Runner images**: Are `ubuntu-latest`, `macos-15`, `windows-latest` still current recommended images?

### Step 6: Generate Report

Present findings as a structured report:

```markdown
## CI Update Report

**Generated**: YYYY-MM-DD
**Scanned**: N workflow files, N config files

### Sync Drift (fix immediately)
| Component | Expected | Actual (file) | Action |
|-----------|----------|---------------|--------|
| ... | ... | ... | ... |

(or "No sync drift detected.")

### Available Updates

#### Safe Updates (recommend applying)
| Tool | Current | Latest | Risk | Files to Update |
|------|---------|--------|------|-----------------|
| ... | ... | ... | Safe | ... |

#### Updates Requiring Review
| Tool | Current | Latest | Risk | Breaking Changes | Files to Update |
|------|---------|--------|------|------------------|-----------------|
| ... | ... | ... | Medium | ... | ... |

#### No Update Available
| Tool | Current | Notes |
|------|---------|-------|
| ... | ... | Up to date |

### Pseudo-Version Audit
| Tool | Current Pseudo-Version | Tagged Release Available? | Recommendation |
|------|------------------------|---------------------------|----------------|
| benchstat | v0.0.0-... | Yes/No | ... |

### Node.js LTS Status
- Current pin: `N`
- LTS status: Active LTS until YYYY-MM-DD / Maintenance / EOL
- Recommendation: ...

### CI Optimizations
- (bulleted list of findings, or "No optimizations identified.")

### Documentation Updates Required
After applying updates, these files need version bumps:
- `.agents/rules/version-pinning.md` — "Current pinned versions" section
- `.agents/rules/commands.md` — Prerequisites section (if tool versions changed)
```

### Step 7: Get User Approval

Present the report and **STOP**. Ask the user which updates to apply:

- "Apply all safe updates"
- "Apply specific updates" (let user pick)
- "Skip for now" (report only)

**Do NOT modify any files until the user explicitly approves.**

### Step 8: Apply Approved Updates

For each approved update, modify files **simultaneously** across all sync pairs:

1. **Workflow files**: Update version strings in all affected `.github/workflows/*.yml` files.
2. **Pre-commit config**: Update `.pre-commit-config.yaml` if golangci-lint changed.
3. **MCP config**: Update `.mcp.json` if MCP server versions changed.
4. **Documentation**:
   - Update `.agents/rules/version-pinning.md` "Current pinned versions" with new versions.
   - Update `.agents/rules/commands.md` Prerequisites if tool versions appear there.

After all edits, re-read all modified files to verify sync pair consistency.

### Step 9: Verify Updates

```bash
# Validate YAML syntax of modified workflows
for f in .github/workflows/*.yml; do
  python3 -c "import yaml; yaml.safe_load(open('$f'))" 2>&1 || echo "INVALID: $f"
done

# Validate JSON syntax of .mcp.json (if modified)
python3 -c "import json; json.load(open('.mcp.json'))" 2>&1 || echo "INVALID: .mcp.json"

# Verify agent docs integrity
make check-agent-docs
```

If any validation fails, fix the issue before proceeding.

### Step 10: Summary and Next Steps

Output a summary of changes:

```markdown
## Changes Applied

| Tool | Old Version | New Version | Files Modified |
|------|-------------|-------------|----------------|
| ... | ... | ... | ... |

## Manual Follow-Up Required
- [ ] Run affected CI workflows to verify (push to a branch or use workflow_dispatch)
- [ ] If golangci-lint was updated: run `make lint` locally to verify no new findings
- [ ] If gotestsum was updated: run `make test` locally to verify compatibility
- [ ] If Node.js version changed: run `cd website && npm run build` to verify
```

## Dependabot Overlap

This skill complements Dependabot, which handles:
- GitHub Actions major version bumps (weekly Monday PRs, grouped)
- Go module dependency updates (`go.mod`)
- npm dependency updates for the website (`package-lock.json`)

This skill fills the gaps Dependabot does NOT cover:
- `go install` tool versions in workflow `run:` blocks
- Inline `curl`-installed binaries (UPX, D2)
- Action `version:` input pins (Cosign via `cosign-release`, golangci-lint, GoReleaser)
- MCP server versions in `.mcp.json`
- GoReleaser semver range track
- Node.js LTS lifecycle tracking
- Cross-file sync pair consistency
- Documentation accuracy in `version-pinning.md` and `commands.md`
- CI optimization opportunities

## Frequency Recommendation

Run `/ci-update` at least:
- Monthly for routine maintenance
- Before each release
- After merging Dependabot PRs (to catch cascading sync needs)
- When a CI failure suggests a version incompatibility
