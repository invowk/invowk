# Commands

## Quick Reference

| Task | Command |
|------|---------|
| Build | `make build` |
| Build UPX | `make build-upx` |
| Test all | `make test` |
| Test short | `make test-short` |
| Single test | `go test -v -run TestName ./path/...` |
| License check | `make license-check` |
| Tidy deps | `make tidy` |
| PGO profile | `make pgo-profile` |
| PGO profile (short) | `make pgo-profile-short` |
| Release tag | `make release VERSION=v0.1.0` |
| Release bump | `make release-bump TYPE=minor [PRERELEASE=alpha]` |
| GoReleaser check | `goreleaser check` |
| GoReleaser dry-run | `goreleaser release --snapshot --clean` |
| Website dev | `cd website && npm start` |
| Website build | `cd website && npm run build` |

## Prerequisites

- **Go 1.25+** - Required for building.
- **Make** - Build automation.
- **Node.js 20+** - For website development (optional).
- **Docker or Podman** - For container runtime tests (optional).
- **UPX** - For compressed builds (optional).

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

### x86-64 Microarchitecture Levels

The project defaults to `GOAMD64=v3` for x86-64 builds, targeting CPUs from 2013+ (Intel Haswell, AMD Excavator). This enables AVX, AVX2, BMI1/2, FMA, and other modern instructions for better performance.

Available levels:
- `v1` - Baseline x86-64 (maximum compatibility, any 64-bit x86 CPU).
- `v2` - Nehalem+ (2008+): SSE4.2, POPCNT.
- `v3` - Haswell+ (2013+): AVX, AVX2, BMI1/2, FMA (default).
- `v4` - Skylake-X+ (2017+): AVX-512.

## Profile-Guided Optimization (PGO)

Invowk uses PGO to optimize builds based on representative runtime profiles. Go 1.20+ automatically detects `default.pgo` in the main package directory.

```bash
# Generate full PGO profile (includes container benchmarks)
# Takes several minutes; produces default.pgo
make pgo-profile

# Generate short PGO profile (skips container benchmarks)
# Faster but may result in less comprehensive optimization
make pgo-profile-short

# Verify PGO is active during builds
GODEBUG=pgoinstall=1 make build 2>&1 | grep -i pgo
```

**When to regenerate profiles:**
- After major changes to hot paths (CUE parsing, runtime execution, discovery)
- When adding new runtimes or significantly changing existing ones
- Before a major release to ensure optimizations are up-to-date

**Profile location:** `default.pgo` in the repository root (committed).

**Benchmark source:** `internal/benchmark/benchmark_test.go` contains benchmarks covering:
- CUE parsing and schema validation
- Module and command discovery
- Native and virtual shell execution
- Container runtime (when not in short mode)
- Full end-to-end pipeline

## Test Commands

```bash
# Run all tests (verbose)
make test

# Run tests in short mode (skips integration tests)
make test-short

# Run integration tests only
make test-integration

# Run a single test by name
go test -v -run TestFunctionName ./path/to/package/...

# Run a single test file
go test -v ./internal/config/config_test.go ./internal/config/config.go

# Run tests with coverage
go test -v -cover ./...

# Run tests for a specific package
go test -v ./internal/runtime/...
go test -v ./pkg/invkfile/...
```

## Releasing

Releases are automated using [GoReleaser](https://goreleaser.com) and GitHub Actions.

### How to Create a Release

There are two paths to create a release:

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
- **Binaries**: UPX-compressed executables for Linux/macOS/Windows (amd64, arm64).
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

### Verifying Signatures

Users can verify release artifacts using Cosign:

```bash
cosign verify-blob \
  --certificate checksums.txt.pem \
  --signature checksums.txt.sig \
  --certificate-identity-regexp='https://github\.com/invowk/invowk/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  checksums.txt
```

### CI/CD Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Push/PR to main (non-website changes) | Run tests, build verification, license check |
| `lint.yml` | Push/PR to main (non-website changes) | Advisory golangci-lint run |
| `release.yml` | Tag push (v*) or manual dispatch | Validate, test, then build and publish release |
| `test-website.yml` | PR to main (website changes) | Build website for PR validation |
| `deploy-website.yml` | Push to main (website changes) or manual | Build and deploy GitHub Pages site |

### Versioning

- Use [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`.
- Pre-releases: `v1.0.0-alpha.1`, `v1.0.0-beta.1`, `v1.0.0-rc.1`.
- Tags from tag-push path should be GPG/SSH-signed (`git tag -s`).
- Tags from workflow dispatch are lightweight (CI runners lack signing keys).
