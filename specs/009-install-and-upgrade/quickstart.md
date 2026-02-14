> **Status Update (2026-02-13):** User Story 3 (Self-Upgrade via CLI) has been removed
> from this branch. The Phase 3 (Self-Upgrade) quickstart section is no longer applicable.
> This document is retained as design history. See the branch commit log for details.

# Developer Quickstart: Installation Methods & Self-Upgrade

**Phase 1 Output** | **Date**: 2026-02-13

## Prerequisites

- Go 1.26+ (for building and testing)
- Make (build automation)
- Podman or Docker (for container runtime tests, optional)
- `shellcheck` (for install script linting, optional but recommended)

## Development Workflow

### Phase 1: Module Path Migration

```bash
# 1. Update go.mod module path
sed -i 's|^module invowk-cli$|module github.com/invowk/invowk|' go.mod

# 2. Update all Go imports
find . -name '*.go' -exec sed -i 's|"invowk-cli/|"github.com/invowk/invowk/|g' {} +

# 3. Update Makefile ldflags
sed -i 's|invowk-cli/cmd/invowk|github.com/invowk/invowk/cmd/invowk|g' Makefile

# 4. Update GoReleaser ldflags
sed -i 's|invowk-cli/cmd/invowk|github.com/invowk/invowk/cmd/invowk|g' .goreleaser.yaml

# 5. Fix import formatting
goimports -w .
gofumpt -w .

# 6. Verify
go build ./...
make test
```

### Phase 2: Version Display

After migration, modify `cmd/invowk/root.go`:

```bash
# Test the fallback behavior
go install ./...
$(go env GOPATH)/bin/invowk --version
# Should show the module version, not "dev"
```

### Phase 3: Self-Upgrade Command

```bash
# Create the new package
mkdir -p internal/selfupdate

# Run tests for the new package
go test -v ./internal/selfupdate/...

# Test the CLI command
go test -v -run TestUpgrade ./cmd/invowk/...

# Manual test: build an old version, then upgrade
make build VERSION=v0.0.1
./bin/invowk upgrade --check
```

### Phase 4: Install Script

```bash
# Lint the script
shellcheck scripts/install.sh

# Test on local machine (uses a temp install dir)
INSTALL_DIR=$(mktemp -d) sh scripts/install.sh
ls -la $(mktemp -d)/invowk

# Test with specific version
INVOWK_VERSION=v0.1.0 INSTALL_DIR=/tmp/invowk-test sh scripts/install.sh
```

### Phase 5: Homebrew Tap

```bash
# Validate GoReleaser config
goreleaser check

# Dry-run to verify formula generation
goreleaser release --snapshot --clean
# Check dist/ for generated formula
cat dist/homebrew/Formula/invowk.rb
```

## Key Files to Understand

| File | Purpose |
|------|---------|
| `cmd/invowk/root.go` | Version variables, root command setup |
| `.goreleaser.yaml` | Release configuration, Homebrew tap |
| `Makefile` | Build flags, ldflags injection |
| `scripts/release.sh` | Existing release script (pattern reference) |
| `internal/container/engine.go` | Example of sentinel error pattern |
| `cmd/invowk/service_error.go` | ServiceError pattern for CLI error rendering |

## Testing Strategy

| Component | Test Type | Command |
|-----------|-----------|---------|
| GitHub API client | Unit (httptest server) | `go test -v ./internal/selfupdate/ -run TestGitHub` |
| Checksum verification | Unit | `go test -v ./internal/selfupdate/ -run TestChecksum` |
| Install detection | Unit | `go test -v ./internal/selfupdate/ -run TestDetect` |
| Version comparison | Unit | `go test -v ./internal/selfupdate/ -run TestVersion` |
| Upgrade command | Unit + testscript | `go test -v ./cmd/invowk/ -run TestUpgrade` |
| Install script | Manual | `shellcheck scripts/install.sh && INSTALL_DIR=/tmp/test sh scripts/install.sh` |
| Full suite | Integration | `make test` |

## Common Pitfalls

1. **Import grouping after migration**: `goimports` may add blank lines between `github.com/invowk/invowk/internal/...` and `github.com/invowk/invowk/pkg/...` imports. All project imports must stay in the same group.

2. **POSIX sh vs bash**: The install script must not use bashisms (`[[ ]]`, `${var,,}`, arrays, `local` keyword in some shells). Use `[ ]`, `tr`, and positional parameters.

3. **Atomic rename across filesystems**: `os.Rename()` fails across filesystem boundaries. Always create the temp file in the same directory as the target binary.

4. **GitHub API pagination**: The releases endpoint returns 30 results per page. For finding the latest stable, the first page is usually sufficient, but pagination should be implemented for completeness.

5. **Pre-release version comparison**: `v1.1.0-alpha.1` is LESS than `v1.1.0` in semver. But it's GREATER than `v1.0.0`. The upgrade command must handle this correctly — a user on `v1.1.0-alpha.1` with latest stable `v1.0.0` should see "pre-release ahead of stable", not "upgrade available".
