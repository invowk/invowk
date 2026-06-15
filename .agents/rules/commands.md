# Commands

## Quick Reference

| Task | Command |
|------|---------|
| Build | `make build` |
| Build UPX | `make build-upx` |
| Test all | `make test` |
| Test short | `make test-short` |
| Single test | `go test -v -run TestName ./path/...` |
| Test CLI with coverage | `make test-cli-cover` |
| Lint | `make lint` |
| Vulnerability scan | `make vulncheck` |
| Local Sonar issues | `make sonar-local` |
| Type check (DDD) | `make check-types` |
| Type check (JSON) | `make check-types-json` |
| Type check (all DDD) | `make check-types-all` |
| Type check (all JSON) | `make check-types-all-json` |
| Semantic spec check | `make check-semantic-spec` |
| IFDS compatibility check | `make check-ifds-compat` |
| Phase C refinement check | `make check-cfg-refinement` |
| Phase D alias check | `make check-cfg-alias` |
| Baseline check | `make check-baseline` |
| Baseline update | `make update-baseline` |
| Mutation dry-run | `make mutation-dry-run` |
| Mutation PR scan | `make mutation-pr` |
| Mutation full scan | `make mutation-full` |
| Mutation baseline update | `make mutation-baseline-update` |
| Mutation rerun | `make mutation-rerun MUTATION_MUTANT_ID=<id>` |
| File length check | `make check-file-length` |
| Agent docs check | `make check-agent-docs` |
| License check | `make license-check` |
| Tidy deps | `make tidy` |
| PGO profile | `make pgo-profile` |
| PGO profile (short) | `make pgo-profile-short` |
| PGO profile (parse/discovery) | `make pgo-profile-parse-discovery` |
| PGO audit | `make pgo-audit` |
| Benchmark report | `make bench-report` |
| Benchmark report (full) | `make bench-report-full` |
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
- **Node.js 24+** - For website development (optional).
- **Docker or Podman** - For container runtime tests (optional).
- **UPX** - For compressed builds (optional).
- **gotestsum** - Enhanced test runner with `--rerun-fails` support (optional locally, used in CI). Install: `go install gotest.tools/gotestsum@v1.13.0`.
- **govulncheck** - Go vulnerability scanner used by `make vulncheck` and CI. Install the pinned version from `.agents/rules/version-pinning.md`.
- **go-mutesting** - Mutation testing tool pinned through the root `go.mod` tool directive. Do not install it manually with `@latest`; use the Make targets or `go tool go-mutesting` from the repository root.

## Internal Commands (Hidden)

- All `invowk internal *` commands and subcommands MUST remain hidden.
- Do NOT document internal commands in website docs; only mention them in `README.md` and agent-facing docs under `.agents/`.

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

# Scan every tracked Go module with govulncheck
make vulncheck
```

Defaults to `GOAMD64=v3` (Haswell+ CPUs, 2013+). Override with `make build GOAMD64=v1` for maximum compatibility.

## Profile-Guided Optimization (PGO)

Go automatically detects `default.pgo` in the main package directory. Profile location: `default.pgo` in the repository root (committed). Benchmark source: `internal/benchmark/benchmark_test.go`.

```bash
make pgo-profile                  # Full profile (includes container benchmarks)
make pgo-profile-short            # Short profile (skips container benchmarks)
make pgo-profile-parse-discovery  # Focused profile for parser/discovery hot paths
make pgo-audit                    # Validate profile freshness + required symbols
```

Profile generation targets run benchmark training with `-pgo=off` so the new
profile is not biased by a previously committed one.

**When to regenerate:** After major changes to hot paths (CUE parsing, runtime execution, discovery), when adding/changing runtimes, or before a major release.

**Automation:** Three layers prevent stale profiles: (1) Claude Code PostToolUse hook warns when editing hot-path files, (2) pre-commit `pgo-staleness` hook warns when hot-path files are staged without `default.pgo`, (3) CI `pgo-sanity` job auto-regenerates and pushes a fix commit to the PR branch. Prefer local regeneration when warned.

## Benchmark Reports

Use benchmark reports for readable performance snapshots in terminal and markdown output:

```bash
make bench-report       # Startup + internal/benchmark report (short mode, no container benchmarks)
make bench-report-full  # Startup + internal/benchmark report (full mode, includes container benchmarks)
```

Reports are written to `docs/benchmarks/YYYY-MM-DD_HH-mm-ss.md` and include:
- Run metadata (commit, branch, platform, Go version, mode)
- Startup timing table (`--version`, `--help`, `cmd --help`, `cmd`)
- Parsed `internal/benchmark` table (`ns/op`, `ms/op`, estimated run/total time, `B/op`, `allocs/op`)
- Raw benchmark outputs for traceability

## Local SonarCloud Status Check

Fetch the quality gate status and unresolved issues from SonarCloud via REST API:

```bash
make sonar-local
```

Optional environment overrides:

```bash
SONAR_TOKEN=your_token           # Optional — enables auth for private projects / higher rate limits
SONAR_HOST_URL=https://sonarcloud.io
SONAR_PROJECT_KEY=invowk_invowk
SONAR_BRANCH=<branch-name>
```

Requires `curl` and `jq`. This is an API-only check — it reads results from
SonarCloud's Automatic Analysis (GitHub App) rather than running a local scan.
Reports are saved to `.sonar/reports/` (quality-gate.json, issues.json).

With pre-commit hooks installed, the `sonar-local` hook runs on changes to
Sonar configuration files and blocks the commit on quality gate failures.

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

# Run CLI integration tests with coverage (produces cli-coverage.out)
make test-cli-cover

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

## Mutation Testing

Mutation testing is a separate manual quality signal and does not run as part of `make test`, the regular CI test matrix, or PR status checks. The wrapper verifies the pinned `go-mutesting` binary before execution, resolves curated targets for the root module and `tools/goplint`, and writes reports under `artifacts/mutation/<profile>/<module>/`. The initial root full profile is a baselineable high-signal seed rather than a blanket package-level scan; the initial `tools/goplint` full profile mutates explicit analyzer source files from the nested module root rather than every support file. Broaden either profile only after local/manual advisory timing and survivor data are stable.

```bash
# Count candidate mutants without executing mutated tests
make mutation-dry-run MUTATION_MODULE=root

# Changed-line pull request profile; advisory by default
make mutation-pr MUTATION_BASE_REF=origin/main MUTATION_MODE=advisory

# Curated broad scan for local/manual use
make mutation-full MUTATION_MODULE=all MUTATION_MODE=advisory

# Intentionally regenerate accepted-survivor baselines
make mutation-baseline-update MUTATION_MODULE=root

# Rerun one escaped mutant by stable id from go-mutesting-agentic.json
make mutation-rerun MUTATION_MODULE=goplint MUTATION_MUTANT_ID=<id>
```

Profiles:
- `dry-run` counts candidates and does not mutate source files.
- `pr` mutates changed eligible Go lines relative to `MUTATION_BASE_REF` and exits successfully when no eligible mutations exist. It is available as a local/manual command, not an automatic PR gate.
- `full` runs the curated root-module and/or `tools/goplint` target manifests.
- `baseline-update` rewrites `tools/mutation/baselines/<module>-baseline.json` intentionally.
- `rerun` executes only one stable escaped-mutant ID.

Defaults:
- `MUTATION_MODULE=all` (`root`, `goplint`, or `all`).
- `MUTATION_MODE=advisory`; use `blocking` only after the baseline and runtime signal are stable.
- `MUTATION_REPORT_DIR=artifacts/mutation`.
- `MUTATION_WORKERS=0` locally unless overridden; the manual GitHub Actions workflow sets a bounded worker count.

Default mutation profiles use package-level Go tests with `-short`, even when a manifest selects explicit source files. They do not pass `-race`, do not run CLI `testscript` suites, and do not run container-engine profiles unless a future opt-in profile documents those costs. Local mutating profiles reject tracked dirty work outside mutation baselines/reports and restore mutated package sources after the tool exits.

Baselines:
- Root module baseline: `tools/mutation/baselines/root-baseline.json`.
- `tools/goplint` baseline: `tools/mutation/baselines/goplint-baseline.json`.
- Baselines contain accepted survivors from reviewed full-scan reports. Update them only as an intentional review step after killing worthwhile survivors.
- Manual workflow behavior is advisory. Blocking mode fails only on new escaped mutants outside the selected baseline and should be used only for explicit experiments.

Reports:
- `go-mutesting.log`: full wrapper/tool output.
- `go-mutesting-summary.json`: compact machine-readable metrics when emitted by the tool.
- `go-mutesting-agentic.json`: escaped-mutant details with stable IDs and context when emitted by the tool.
- `resolved-targets.txt`, `excluded-packages.txt`, and `not-covered-packages.txt`: target-selection evidence.

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
- `-v` (on the `go test` side): **Required** when using `--rerun-fails` with parallel subtests. Without `-v`, `gotestsum` doesn't receive per-subtest PASS/FAIL lines and may misreport parent test status (false FAILs when all subtests pass).

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
   - Attach a generated benchmark report asset (`invowk_<version>_bench-report.md`).

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
- **Benchmark report**: Generated markdown benchmark snapshot attached as `invowk_<version>_bench-report.md`.

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
| `ci.yml` | Push/PR to main (Go code/build changes) | Run tests, build verification, license check, all-module govulncheck |
| `lint.yml` | Push/PR to main (Go code/lint config changes) | **Required** normalized root + `tools/goplint` golangci-lint, formatter/config checks, agent docs integrity, goplint baseline gate, goplint exception governance, and goplint behavior gates + advisory goplint full scan |
| `release.yml` | Tag push (v*) or manual dispatch | Validate, test, then build and publish release |
| `release-benchmark-asset.yml` | Manual dispatch only | Fallback: attach `make bench-report` output to an existing (non-immutable) release |
| `mutation-testing.yml` | Manual dispatch only | Run curated mutation profiles and upload reports; not a PR or scheduled gate |
| `pgo-benchstat.yml` | Weekly schedule + manual dispatch | Compare `pgo=off` vs `pgo=on` with `benchstat` and upload raw/report artifacts |
| `test-website.yml` | PR to main (website/diagram/script changes) | Validate version assets + build website |

Other workflows: `version-docs.yml` (doc versioning on release), `validate-diagrams.yml` (D2 syntax checks), `deploy-website.yml` (GitHub Pages deployment).

### CI Workflow Hygiene

- GitHub Actions step-level `env:` values are scoped only to that step. If a shell variable is used by multiple steps, define it at job-level `env:` or repeat it on every step that references it. This matters especially for `set -u` scripts, where a report-only step can fail after the real build/test command already succeeded.
- Generated reports must be written under ignored artifact directories unless they are intentional release/docs assets. Before committing workflow or tool changes that move reports, run `git ls-files` for the report filenames and make sure root-level tool outputs such as `report.json`, `go-mutesting-summary.json`, `go-mutesting-agentic.json`, `go-mutesting-gitlab.json`, and `go-mutesting-report.html` are not tracked accidentally.

### Versioning

- Use [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`.
- Pre-releases: `v1.0.0-alpha.1`, `v1.0.0-beta.1`, `v1.0.0-rc.1`.
- Tags from tag-push path should be GPG/SSH-signed (`git tag -s`).
- Tags from workflow dispatch are lightweight (CI runners lack signing keys).
