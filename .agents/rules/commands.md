# Commands

## Quick Reference

| Task | Command |
|------|---------|
| Build | `make build` |
| Build UPX | `make build-upx` |
| Test all | `make test` |
| Test short | `make test-short` |
| Single test | `go test -v -run TestName ./path/...` |
| Lint | `make lint` |
| License check | `make license-check` |
| Tidy deps | `make tidy` |
| PGO profile | `make pgo-profile` |
| PGO profile (short) | `make pgo-profile-short` |
| Release tag | `make release VERSION=v0.1.0` |
| Release bump | `make release-bump TYPE=minor [PRERELEASE=alpha]` |
| Version docs | `make version-docs VERSION=1.0.0` |
| GoReleaser check | `goreleaser check` |
| GoReleaser dry-run | `goreleaser release --snapshot --clean` |
| Website dev | `cd website && npm start` |
| Website build | `cd website && npm run build` |

## Prerequisites

- **Go 1.26+** - Required for building.
- **Make** - Build automation.
- **Node.js 20+** - For website development (optional).
- **Docker or Podman** - For container runtime tests (optional).
- **UPX** - For compressed builds (optional).
- **gotestsum** - Enhanced test runner with `--rerun-fails` support (optional locally, used in CI). Install: `go install gotest.tools/gotestsum@v1.13.0`.

## Internal Commands (Hidden)

- All `invowk internal *` commands and subcommands MUST remain hidden.
- Do NOT document internal commands in website docs; only mention them in `README.md` and agent-facing docs under `.claude/`.

## Build Commands

```bash
# Build the binary (default, stripped)
# On x86-64, targets x86-64-v3 microarchitecture by default (Haswell+ CPUs, 2013+)
make build

# Build with debug symbols for development
make build-dev

# Build with UPX compression (smallest size, requires UPX)
make build-upx

# Build all variants
make build-all

# Cross-compile for multiple platforms (x86-64 targets use v3 by default)
make build-cross

# Build for maximum compatibility (baseline x86-64)
make build GOAMD64=v1

# Install to $GOPATH/bin
make install

# Clean build artifacts
make clean

# Tidy dependencies
make tidy
```

Defaults to `GOAMD64=v3` (Haswell+ CPUs, 2013+). Override with `make build GOAMD64=v1` for maximum compatibility.

## Profile-Guided Optimization (PGO)

Go automatically detects `default.pgo` in the main package directory. Profile location: `default.pgo` in the repository root (committed). Benchmark source: `internal/benchmark/benchmark_test.go`.

```bash
make pgo-profile        # Full profile (includes container benchmarks)
make pgo-profile-short  # Short profile (skips container benchmarks)
```

**When to regenerate:** After major changes to hot paths (CUE parsing, runtime execution, discovery), when adding/changing runtimes, or before a major release.

## Test Commands

```bash
# Run all tests (verbose)
# Uses gotestsum with --rerun-fails when available, falls back to go test
make test

# Run tests in short mode (skips integration tests)
make test-short

# Run integration tests only
make test-integration

# Run CLI integration tests (testscript-based)
make test-cli

# Run a single test by name
go test -v -run TestFunctionName ./path/to/package/...

# Run a single test file
go test -v ./internal/config/config_test.go ./internal/config/config.go

# Run tests with coverage
go test -v -cover ./...

# Run tests for a specific package
go test -v ./internal/runtime/...
go test -v ./pkg/invowkfile/...
```

### gotestsum (CI-Level Retry and Reporting)

CI uses `gotestsum` to wrap `go test` with transient failure retry and JUnit XML reporting. Locally, `make test` auto-detects `gotestsum` and uses it when available.

```bash
# Install gotestsum
go install gotest.tools/gotestsum@v1.13.0

# Run tests with gotestsum directly (rerun up to 5 transient failures)
gotestsum --format testdox --rerun-fails --rerun-fails-max-failures 5 --packages ./... -- -v

# Run with JUnit XML output and flake report
gotestsum \
  --format testdox \
  --junitfile test-results.xml \
  --rerun-fails \
  --rerun-fails-max-failures 5 \
  --rerun-fails-report rerun-report.txt \
  --packages ./... \
  -- -v
```

**Key flags:**
- `--rerun-fails`: Re-run only failing tests after the full suite completes.
- `--rerun-fails-max-failures N`: Skip reruns if more than N tests fail (real regression, not flakiness).
- `--rerun-fails-report FILE`: Log which tests needed reruns (flake signal).
- `--junitfile FILE`: JUnit XML for GitHub Actions test reporting.
- `--format testdox`: Human-readable output (test names as sentences).

## Releasing

Releases are automated using [GoReleaser](https://goreleaser.com) and GitHub Actions.

### How to Create a Release

There are three paths to create a release:

#### Option 1: Tag Push (recommended for production releases)

Produces GPG/SSH-signed tags. Use this for stable releases.

1. **Ensure all tests pass** on the `main` branch.
2. **Create and push a signed version tag**:
   ```bash
   git tag -s v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```
3. **GitHub Actions will automatically**:
   - Run tests on all platforms (Ubuntu, Windows, macOS).
   - Build binaries for all target platforms (with UPX compression).
   - Sign checksums with Cosign (keyless).
   - Create a GitHub Release with artifacts.

#### Option 2: Workflow Dispatch (good for pre-releases and quick iteration)

Creates lightweight tags from the CI runner. Use this for alpha/beta/RC releases or when you want a dry-run first.

1. Go to **Actions > Release > Run workflow** in the GitHub UI.
2. Enter the version (e.g., `v0.1.0-alpha.1`) and optionally enable **dry run**.
3. The workflow validates the version format, checks the branch is `main`, and verifies the tag doesn't already exist.
4. After tests pass, it creates the tag and runs GoReleaser (or builds without publishing in dry-run mode).

**Dry run** builds all artifacts locally without creating a GitHub Release. Use this to validate the release pipeline before committing to a real release.

#### Option 3: Makefile Targets (convenience wrapper for Option 1)

Uses `scripts/release.sh` to validate, tag, and push in one command.

```bash
# Tag a specific version
make release VERSION=v1.0.0

# Compute next version and tag (auto-bumps from latest stable tag)
make release-bump TYPE=minor                        # v1.0.0 -> v1.1.0
make release-bump TYPE=minor PRERELEASE=alpha       # v1.0.0 -> v1.1.0-alpha.1
make release-bump TYPE=minor PRERELEASE=alpha       # v1.1.0-alpha.1 -> v1.1.0-alpha.2
make release-bump TYPE=minor PROMOTE=1              # Promote prerelease to v1.1.0

# Preview without creating tags
make release-bump TYPE=patch DRY_RUN=1

# Skip confirmation prompt
make release VERSION=v1.0.0 YES=1
```

**Parameters:**
- `VERSION` - Exact semver version (for `release` target).
- `TYPE` - Bump type: `major`, `minor`, or `patch` (for `release-bump` target).
- `PRERELEASE` - Pre-release label: `alpha`, `beta`, or `rc` (optional).
- `PROMOTE=1` - Required when a stable bump would overlap with existing prerelease tags.
- `YES=1` - Skip the confirmation prompt.
- `DRY_RUN=1` - Show computed version and summary without creating tags.

### Release Artifacts

Each release includes:
- **Binaries**: UPX-compressed for Linux; stripped (uncompressed) for macOS and Windows (amd64, arm64).
- **Archives**: `.tar.gz` for Linux/macOS, `.zip` for Windows.
- **Checksums**: SHA256 checksums in `checksums.txt`.
- **Signatures**: Cosign signatures for verification.

### Local Testing

Test the release process locally before pushing a tag:

```bash
# Validate GoReleaser configuration
goreleaser check

# Dry-run release (builds locally, no publishing)
goreleaser release --snapshot --clean
```

### CI/CD Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Push/PR to main (Go code/build changes) | Run tests, build verification, license check |
| `lint.yml` | Push/PR to main (Go code/lint config changes) | Advisory golangci-lint run |
| `release.yml` | Tag push (v*) or manual dispatch | Validate, test, then build and publish release |
| `test-website.yml` | PR to main (website/diagram/script changes) | Validate version assets + build website |

Other workflows: `version-docs.yml` (doc versioning on release), `validate-diagrams.yml` (D2 syntax checks), `deploy-website.yml` (GitHub Pages deployment).

### Versioning

- Use [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`.
- Pre-releases: `v1.0.0-alpha.1`, `v1.0.0-beta.1`, `v1.0.0-rc.1`.
- Tags from tag-push path should be GPG/SSH-signed (`git tag -s`).
- Tags from workflow dispatch are lightweight (CI runners lack signing keys).
