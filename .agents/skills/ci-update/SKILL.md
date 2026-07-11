---
name: ci-update
description: >-
  Audit and update CI workflow versions, tool installs, MCP servers,
  Dependabot-covered action pins, inline binary installs, pre-commit hooks,
  Node.js LTS pins, and CI-created commit mechanisms. Use when preparing a
  release, doing monthly CI maintenance, investigating version-related CI
  failures, or reviewing workflow/tooling pin drift.
---

# CI Update

Audit CI versions and workflow hygiene from the current checkout. Treat
`.agents/rules/version-pinning.md` as authoritative: pin every external
dependency, let Dependabot manage covered updates, and update manual pins and
all synchronized references together.

## Documentation and Version Sources

Before interpreting current CLI, action, API, or tool behavior, use Context7:

1. Resolve the exact library/tool ID with the full maintenance question.
2. Query the selected ID for the relevant current syntax, compatibility, or
   migration behavior.
3. Use official release APIs, package registries, and local module metadata for
   latest-version facts. If Context7 has no suitable source, use the tool's
   official documentation or release notes and state the fallback.

Do not use remembered syntax or secondary posts as evidence. Load
[`references/version-discovery.md`](references/version-discovery.md) when
checking latest versions or breaking changes.

## Required Sync Pairs

Derive the live inventory first; this table identifies known coupling, not a
closed allowlist.

| Component | Files or constraints |
|-----------|----------------------|
| `golangci-lint` | Root `go.mod`, `scripts/golangci-lint.sh`, normalized workflow/hook targets, `.agents/rules/version-pinning.md` |
| `gotestsum` | `.github/workflows/ci.yml`, `.github/workflows/release.yml` |
| GoReleaser | Every `goreleaser-action` `version:` input in CI and release workflows |
| Node.js | Every workflow `node-version:` reference |
| Cosign | Installer action major and `cosign-release` input compatibility in CI and release workflows |
| UPX | Every inline `UPX_VERSION` installation |
| GitHub Actions | Every repeated `uses:` target found by inventory |
| Documented pins | `.agents/rules/version-pinning.md` and prerequisites in `.agents/rules/commands.md` |
| Sonar suppressions | `sonar-project.properties` and `.sonarcloud.properties` must remain synchronized |

## Workflow

### 1. Build the Live Inventory

```bash
find .github/workflows -maxdepth 1 -type f -name '*.yml' -print | sort
rg -n 'uses:|go install|node-version|UPX_VERSION|cosign-release|version:' \
  .github/workflows .pre-commit-config.yaml .mcp.json \
  .agents/rules/version-pinning.md .agents/rules/commands.md
```

Also inspect `go.mod`, `scripts/golangci-lint.sh`, `.github/dependabot.yml`,
`.goreleaser.yaml`, and every install script discovered by the scan. Record the
category, tool, current version, and every location. Do not silently omit a pin
because it is absent from an example in this skill.

### 2. Detect Sync Drift Before Updates

Compare every repeated pin and coupling constraint. At minimum verify:

- Root `golangci-lint` tool version equals the wrapper expectation; workflows
  and pre-commit route through normalized Make/wrapper targets.
- `gotestsum`, GoReleaser, Node.js, Cosign, and UPX agree everywhere.
- Every repeated action uses the repository-approved major consistently.
- Cosign installer and binary majors are compatible.
- Documented current pins match executable configuration.

Report inconsistencies as `SYNC DRIFT` before discussing available upgrades.

### 3. Discover Current Versions

Follow [`references/version-discovery.md`](references/version-discovery.md).
Query each item found in Step 1, not a static action list. Separate:

- official latest-version facts,
- Dependabot-covered minor/patch drift,
- manual-update gaps,
- intentional branch or pseudo-version exceptions.

If a query fails, report incomplete evidence for that item. Do not present a
cached or remembered version as current.

### 4. Analyze Compatibility

Classify each available update:

| Risk | Meaning |
|------|---------|
| Safe | Patch or documented compatible maintenance update |
| Low | Backward-compatible minor update with relevant new behavior |
| Medium | Major update with a documented migration path |
| High | Major update with removed behavior or material workflow changes |

Use current primary documentation for Medium/High findings. Relate migrations
to Invowk's actual inputs and workflow usage rather than reproducing a generic
changelog.

### 5. Audit Workflow Hygiene

Check every workflow for:

- caching on setup steps where it is safe and useful,
- concurrency groups and the correct cancellation policy,
- explicit job timeouts,
- least-privilege job-level permissions,
- redundant matrices,
- current runner-image support,
- consistent concurrency group naming,
- every commit-producing job's signing mechanism.

For commit-producing jobs, derive the inventory each run:

```bash
rg -n 'createCommitOnBranch|git commit|git push|git tag' .github/workflows
```

Load [`references/verified-bot-commits.md`](references/verified-bot-commits.md)
when a workflow creates or changes commits. Do not classify tag creation as a
commit-signing failure.

### 6. Report and Stop for Approval

Report:

1. Scan scope and evidence gaps.
2. Sync drift requiring immediate repair.
3. Safe updates.
4. Updates requiring review, with relevant compatibility impact.
5. Pseudo-version and Node.js LTS status.
6. Workflow-hygiene findings.
7. Commit-signing inventory.
8. Exact files each update would modify.

Stop after the report. Do not modify files until the user explicitly approves
which updates to apply.

### 7. Apply Approved Updates Atomically

Update every location in each approved sync pair in the same change. Re-read
modified files and rerun the live inventory to prove no old pin remains. Update
the documented pin tables and prerequisites whenever they name the changed
version.

### 8. Verify

Validate modified YAML and JSON with a real parser. Do not treat grep presence
as syntax validation.

```bash
if python3 -c 'import yaml' 2>/dev/null; then
  for file in .github/workflows/*.yml; do
    python3 -c 'import sys, yaml; yaml.safe_load(open(sys.argv[1]))' "$file"
  done
elif command -v yq >/dev/null 2>&1; then
  for file in .github/workflows/*.yml; do yq '.' "$file" >/dev/null; done
else
  echo 'error: install PyYAML or yq to validate workflow syntax' >&2
  exit 1
fi
python3 -c "import json; json.load(open('.mcp.json'))"
make check-agent-docs
```

Run the behavior gate appropriate to each update: `make lint` for
golangci-lint, `make test` for test tooling, and the website build for Node.js
changes. Report CI workflows that still require a pushed-branch or manual
dispatch check.

## Dependabot Boundary

Dependabot normally covers GitHub Actions, Go modules, and website npm
dependencies. This skill owns uncovered inline installs, action input versions,
MCP versions, runtime lifecycle, cross-file consistency, and CI optimization.
Verify `.github/dependabot.yml` before relying on that division.
