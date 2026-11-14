---
name: dep-audit
description: Audit Go dependencies for vulnerabilities (govulncheck, make vulncheck), stale, deprecated or retracted modules, and available updates across the root module and nested Go modules. Use before releases, for periodic dependency health checks, or when evaluating dependency upgrades.
---

# Dependency Audit

Audit Go module dependencies for security vulnerabilities and available updates.

## Workflow

### Step 1: Check Prerequisites

Verify required tools are available:

```bash
# Required
go version

# Optional but recommended
command -v govulncheck || echo "MISSING: Install govulncheck using the pin in .agents/rules/version-pinning.md"
```

If `govulncheck` is missing, report it and continue with update checks only.

Build the module list once from tracked `go.mod` files (currently the root module only; the pinned `github.com/invowk/goplint` tool dependency rides the root module graph):

```bash
mapfile -t go_modules < <(git ls-files 'go.mod' '*/go.mod' | xargs -n1 dirname | sort -u)
printf '%s\n' "${go_modules[@]}"
```

### Step 2: Vulnerability Scan

If `govulncheck` is available:

```bash
make vulncheck
```

The `make vulncheck` target uses `scripts/govulncheck-all.sh`, discovers tracked
Go modules, logs each module before scanning it, and runs `govulncheck ./...`
from that module root. To debug a single module, run the same scan by hand:

```bash
for mod in "${go_modules[@]}"; do
  echo "==> govulncheck: $mod"
  (cd "$mod" && govulncheck ./...)
done
```

Report:
- **Critical**: Vulnerabilities in code paths actually called by invowk
- **Informational**: Vulnerabilities in imported modules but not in called code paths

### Step 3: Check for Updates, Deprecations and Retractions

Run `go list` once per module and apply both filters to the saved output:

```bash
work=$(mktemp -d)
for mod in "${go_modules[@]}"; do
  out="$work/$(printf '%s' "$mod" | tr '/' '_').json"
  if ! (cd "$mod" && go list -m -u -retracted -json all) >"$out"; then
    echo "INCOMPLETE: go list failed for module $mod" >&2
    continue
  fi
  echo "==> $mod: available updates"
  jq -r 'select(.Update) | "\(.Path): \(.Version) → \(.Update.Version)"' "$out"
  echo "==> $mod: deprecated or retracted"
  jq -r 'select(.Deprecated or .Retracted) | "\(.Path): deprecated=\(.Deprecated // "-") retracted=\(.Retracted // [])"' "$out"
done
```

If `jq` is not available, fall back to the following. It lists available updates
only, not deprecated or retracted modules, so report those as unchecked:

```bash
for mod in "${go_modules[@]}"; do
  (cd "$mod" && go list -m -u all)
done | grep '\['
```

If `go list` fails for any module, report that module as incomplete evidence instead of hiding the error.

### Step 4: Categorize Updates

Group available updates by impact:

| Category | Description |
|----------|-------------|
| **Security** | Updates that fix known vulnerabilities (cross-reference with govulncheck) |
| **Major** | Major version bumps (may have breaking API changes) |
| **Minor** | Minor version bumps (new features, backward-compatible) |
| **Patch** | Patch version bumps (bug fixes only) |

### Step 5: Report Deprecated Modules

List every module the Step 3 deprecated/retracted filter printed, with its
deprecation message or retracted versions.

### Step 6: Verify Module Tidiness

```bash
# Check if go.mod/go.sum are tidy
for mod in "${go_modules[@]}"; do
  (cd "$mod" && go mod tidy -diff 2>&1)
done
```

If the root module differs, report that `make tidy` is needed.

### Step 7: Generate Report

Output a summary table:

```markdown
## Dependency Audit Report

### Vulnerabilities
- X critical (in call graph)
- Y informational (in dependencies)

### Available Updates
| Module | Current | Latest | Type |
|--------|---------|--------|------|
| ... | v1.2.3 | v1.3.0 | minor |

### Deprecated Modules
- (none or list)

### Module Tidiness
- ✓ go.mod is tidy / ✗ go.mod needs tidying

### Recommended Actions
1. (prioritized list of suggested upgrades)
```

### Step 8: Offer Upgrade Commands

For each recommended upgrade, provide the exact command:

```bash
(cd <module-dir> && go get module@version)
```

Use `make tidy` for the root module. Run `make test` after upgrading.

Do not upgrade these with a bare `go get`:

- Go tool dependencies (golangci-lint, go-mutesting, goplint): use
  `go get -tool <module>/cmd/<tool>@vX.Y.Z` and update the wrapper scripts and
  pins together (`.agents/rules/version-pinning.md` "Go Tool Dependencies"; the
  ci-update skill's "Required Sync Pairs").
- The CUE library (`cuelang.org/go`): follow the upgrade process in the cue
  skill (`.agents/skills/cue/SKILL.md` "Upgrade Process").
