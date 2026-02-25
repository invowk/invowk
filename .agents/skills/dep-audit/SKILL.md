---
name: dep-audit
description: Audit Go dependencies for vulnerabilities and available updates. Runs govulncheck and checks for stale modules.
user-invocable: true
disable-model-invocation: true
---

# Dependency Audit

Audit Go module dependencies for security vulnerabilities and available updates.

## When to Use

Invoke this skill (`/dep-audit`) when:
- Preparing a release and want to verify dependency health
- Periodically checking for known vulnerabilities
- Evaluating whether dependencies need upgrading

## Workflow

### Step 1: Check Prerequisites

Verify required tools are available:

```bash
# Required
go version

# Optional but recommended
command -v govulncheck || echo "MISSING: Install with a pinned version, e.g. go install golang.org/x/vuln/cmd/govulncheck@vX.Y.Z"
```

If `govulncheck` is missing, report it and continue with update checks only.

### Step 2: Vulnerability Scan

If `govulncheck` is available:

```bash
govulncheck ./...
```

Report:
- **Critical**: Vulnerabilities in code paths actually called by invowk
- **Informational**: Vulnerabilities in imported modules but not in called code paths

### Step 3: Check for Available Updates

```bash
# List all modules with available updates
go list -m -u -json all 2>/dev/null | jq -r 'select(.Update) | "\(.Path): \(.Version) → \(.Update.Version)"'
```

If `jq` is not available, fall back to:

```bash
go list -m -u all 2>/dev/null | grep '\['
```

### Step 4: Categorize Updates

Group available updates by impact:

| Category | Description |
|----------|-------------|
| **Security** | Updates that fix known vulnerabilities (cross-reference with govulncheck) |
| **Major** | Major version bumps (may have breaking API changes) |
| **Minor** | Minor version bumps (new features, backward-compatible) |
| **Patch** | Patch version bumps (bug fixes only) |

### Step 5: Check for Deprecated Modules

```bash
# Look for retracted or deprecated modules
go list -m -json all 2>/dev/null | jq -r 'select(.Deprecated) | "\(.Path): \(.Deprecated)"'
```

### Step 6: Verify Module Tidiness

```bash
# Check if go.mod/go.sum are tidy
go mod tidy -diff 2>&1
```

If there are differences, report that `make tidy` is needed.

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
go get module@version
```

And remind to run `make tidy` and `make test` after upgrading.
