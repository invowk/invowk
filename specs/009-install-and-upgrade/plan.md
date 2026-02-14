> **Status Update (2026-02-13):** User Story 3 (Self-Upgrade via CLI) has been removed
> from this branch. The `internal/selfupdate/` package and `cmd/invowk/upgrade.go` were
> deleted. Phase 3 (Self-Upgrade Command) of this plan is no longer applicable.
> This document is retained as design history. See the branch commit log for details.

# Implementation Plan: Installation Methods & Self-Upgrade

**Branch**: `009-install-and-upgrade` | **Date**: 2026-02-13 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/009-install-and-upgrade/spec.md`

## Summary

Implement three official installation methods (shell script, Homebrew, `go install`) and a self-upgrade CLI command for invowk. The prerequisite module path migration from `invowk-cli` to `github.com/invowk/invowk` enables `go install` support and gives the project a canonical import path. The upgrade command queries GitHub Releases for the latest stable version, verifies integrity via SHA256 checksums, and performs atomic binary replacement — detecting package-manager installations (Homebrew, `go install`) to suggest the appropriate upgrade path instead.

## Technical Context

**Language/Version**: Go 1.26+
**Primary Dependencies**: `net/http` (GitHub Releases API), `crypto/sha256`, `runtime/debug` (build info), `archive/tar` + `compress/gzip` (asset extraction), `github.com/spf13/cobra` (CLI), `golang.org/x/mod/semver` (version comparison — decided in research.md R5)
**Storage**: Filesystem (binary replacement, temp files for atomic install)
**Testing**: `go test` (unit), `testscript` (CLI integration), manual install script testing on Linux/macOS
**Target Platform**: Linux/macOS (amd64, arm64) for full support; Windows (amd64) for `go install` only
**Project Type**: Single Go CLI project
**Performance Goals**: Install < 30s, upgrade < 60s (network-dependent), version check < 5s
**Constraints**: POSIX sh for install script (no bash), stdlib-only HTTP for upgrade (no third-party GitHub client), atomic binary replacement, SHA256 verification mandatory
**Scale/Scope**: ~111 Go files for import migration, ~5 new Go files for upgrade command, 1 new shell script, GoReleaser config changes, README rewrite

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Idiomatic Go & Schema-Driven Design | PASS | New Go files follow SPDX headers, decorder, error handling patterns. No CUE schema changes needed. |
| II. Comprehensive Testing Discipline | PASS | Unit tests for upgrade logic (version comparison, install detection, checksum verification). Testscript tests for `invowk upgrade --check` and error paths. Install script tested manually on Linux/macOS. |
| III. Consistent User Experience | PASS | `invowk upgrade` follows CLI patterns. Styled output with clear guidance. `--check`/`--yes` flags follow existing conventions. |
| IV. Single-Binary Performance | PASS | No heavy dependencies. Stdlib HTTP client. Lazy network calls (only when upgrade runs). No startup latency impact for other commands. |
| V. Simplicity & Minimalism | PASS | Direct implementation using stdlib. No abstraction layers. Single `internal/selfupdate/` package with clear boundaries. |
| VI. Documentation Synchronization | PASS | README rewrite, website docs update, CLI help text — all in scope. |
| VII. Pre-Existing Issue Resolution | WATCH | The `invowk-cli` module path is a pre-existing limitation blocking `go install`. Addressed as Phase 1 of this plan (in-scope per spec clarification). The README placeholder URL (`yourusername/invowk`) is also fixed as part of documentation updates. |

**Gate Result**: PASS — no violations, one pre-existing issue (module path) addressed in-scope.

## Project Structure

### Documentation (this feature)

```text
specs/009-install-and-upgrade/
├── plan.md              # This file
├── research.md          # Phase 0: Technical research findings
├── data-model.md        # Phase 1: Entity and type design
├── quickstart.md        # Phase 1: Developer quickstart guide
├── contracts/
│   ├── cli-upgrade.md   # CLI command contract (flags, output, exit codes)
│   └── github-api.md    # GitHub Releases API interaction contract
└── tasks.md             # Phase 2 output (/speckit.tasks — NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Module path migration (Phase 1) — changes across entire codebase
go.mod                              # Module path: invowk-cli → github.com/invowk/invowk
Makefile                            # ldflags path update
.goreleaser.yaml                    # ldflags path update + uncomment brews section
cmd/invowk/*.go                     # Import path updates (~40 files)
internal/**/*.go                    # Import path updates (~50 files)
pkg/**/*.go                         # Import path updates (~20 files)
tests/cli/*.go                      # Import path updates (~3 files)

# Version display enhancement (Phase 2)
cmd/invowk/root.go                  # Add debug.ReadBuildInfo() fallback in getVersionString()

# Self-upgrade command (Phase 3)
internal/selfupdate/                # New package
├── selfupdate.go                   # Updater type: Check(), Apply(), platform detection
├── selfupdate_test.go              # Unit tests
├── github.go                       # GitHub Releases API client (list releases, download assets)
├── github_test.go                  # Unit tests with HTTP test server
├── detect.go                       # Install method detection (Homebrew, go install, script, manual)
├── detect_test.go                  # Detection unit tests
├── checksum.go                     # SHA256 checksum verification
└── checksum_test.go                # Checksum unit tests
cmd/invowk/upgrade.go               # Cobra command: invowk upgrade [version] [--check] [--yes]
cmd/invowk/upgrade_test.go          # Command-level tests
cmd/invowk/root.go                  # Register upgrade command

# Install script (Phase 4)
scripts/install.sh                  # POSIX sh install script

# Homebrew tap (Phase 5) — separate repository
# invowk/homebrew-tap (GitHub)
#   └── Formula/invowk.rb           # Auto-generated by GoReleaser

# GoReleaser config (Phase 5)
.goreleaser.yaml                    # Uncomment brews section

# Documentation (Phase 6)
README.md                           # Rewrite installation section
website/docs/getting-started/       # Installation guide (if exists)
```

**Structure Decision**: The upgrade logic lives in a dedicated `internal/selfupdate/` package to separate GitHub API interaction, checksum verification, and binary replacement from CLI orchestration. The CLI layer (`cmd/invowk/upgrade.go`) handles Cobra integration, user prompts, and styled output — following the existing `commandService` / `ServiceError` separation pattern. The install script is a standalone POSIX sh file with no Go dependencies.

## Complexity Tracking

No constitution violations require justification. The `internal/selfupdate/` package is a natural boundary, not premature abstraction — it encapsulates network I/O, crypto verification, and filesystem operations that are independently testable.

## Implementation Phases

### Phase 1: Module Path Migration (Prerequisite)

**Scope**: Change `go.mod` module path from `invowk-cli` to `github.com/invowk/invowk`. Update all 217 import references across 111 Go files, plus ldflags in Makefile and GoReleaser.

**Strategy** (per MEMORY.md "Large-scale sed rename safety"):
1. Update `go.mod` module line
2. `sed -i 's|invowk-cli/|github.com/invowk/invowk/|g'` across all `.go` files
3. Update Makefile ldflags paths
4. Update `.goreleaser.yaml` ldflags paths
5. Run `goimports` + `gofumpt` on all Go files (fixes import grouping)
6. Run `go build ./...` to verify compilation
7. Run `make test` to verify all tests pass
8. Update `.claude/rules/`, `.claude/agents/`, spec files referencing old path

**Risk**: Import grouping — `goimports` enforces grouping rules and may need manual fixup for project imports that span `internal/` and `pkg/`.

### Phase 2: Version Display Enhancement

**Scope**: Enhance `getVersionString()` in `cmd/invowk/root.go` to use `debug.ReadBuildInfo()` as fallback when `-ldflags` version is `"dev"`. This makes `go install` binaries display their module version.

**Changes**:
- Read `debug.ReadBuildInfo()` → extract `Main.Version`
- If ldflags `Version != "dev"`, use ldflags (normal build)
- If ldflags `Version == "dev"` and build info has version, use build info version
- If both are empty, show `"dev (built from source)"`

### Phase 3: Self-Upgrade Command

**Scope**: Implement `invowk upgrade [version] [--check] [--yes]`.

**Subcomponents**:
1. **GitHub Releases client** (`internal/selfupdate/github.go`): List releases, find latest stable, download assets. Uses `net/http` with `Accept: application/vnd.github+json` header. Supports optional `GITHUB_TOKEN` for rate limit relief.
2. **Install method detection** (`internal/selfupdate/detect.go`): Path heuristics (Homebrew cellar paths, GOPATH/bin) + `debug.ReadBuildInfo()` for go-install confirmation + optional build-time ldflags hint.
3. **Checksum verification** (`internal/selfupdate/checksum.go`): Download `checksums.txt`, parse SHA256 entries, verify downloaded archive.
4. **Binary replacement** (`internal/selfupdate/selfupdate.go`): Atomic write-to-temp-then-rename pattern. Preserve file permissions. Handle platform differences.
5. **CLI command** (`cmd/invowk/upgrade.go`): Cobra command with `--check` (dry run), `--yes` (skip confirmation), version argument. Styled output showing current → target version. Detects Homebrew/go-install and suggests appropriate upgrade path.

### Phase 4: Install Script

**Scope**: Create `scripts/install.sh` — a POSIX sh script for one-line installation.

**Features**:
- Platform detection via `uname -s` / `uname -m`
- SHA256 checksum verification (`sha256sum` on Linux, `shasum -a 256` on macOS)
- Download via `curl` or `wget` (auto-detected)
- Default install to `~/.local/bin`, override via `INSTALL_DIR`
- Version override via `INVOWK_VERSION`, default to latest stable
- Entire script body wrapped in `main()` function (FR-009: prevents partial execution from piped downloads)
- PATH setup instructions if install dir not in PATH
- Clean error handling: no partial installations on failure

### Phase 5: Homebrew Tap

**Scope**: Enable Homebrew distribution via GoReleaser's `brews` integration.

**Steps**:
1. Create `invowk/homebrew-tap` GitHub repository
2. Add `HOMEBREW_TAP_TOKEN` secret to main repo
3. Uncomment `brews:` section in `.goreleaser.yaml`
4. Verify formula supports macOS (Intel + Apple Silicon) and Linux (amd64 + arm64)
5. Test with a pre-release to validate the pipeline

### Phase 6: Documentation

**Scope**: Rewrite installation documentation across all touchpoints.

**Files**:
- `README.md`: Complete rewrite of installation section with all three methods + verification steps
- Website docs: Installation guide with tabbed instructions
- CLI `--help` text: Verify `invowk upgrade --help` is clear and complete

## Dependencies

```text
Phase 1 (Module Path) ──→ Phase 2 (Version Display) ──→ Phase 3 (Upgrade Command)
                      └──→ Phase 4 (Install Script)
                      └──→ Phase 5 (Homebrew Tap)
Phase 3 + 4 + 5 ──→ Phase 6 (Documentation)
```

Phase 1 is the prerequisite for everything. Phases 2–5 can partially overlap. Phase 6 follows after all features are implemented.
