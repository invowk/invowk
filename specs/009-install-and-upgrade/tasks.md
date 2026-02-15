> **Status Update (2026-02-13):** User Story 3 (Self-Upgrade via CLI) has been removed
> from this branch. The `internal/selfupdate/` package and `cmd/invowk/upgrade.go` were
> deleted. Phase 5 tasks (T018–T028, T033–T035) are no longer applicable.
> This document is retained as design history. See the branch commit log for details.

# Tasks: Installation Methods & Self-Upgrade

**Input**: Design documents from `/specs/009-install-and-upgrade/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Included. The plan explicitly lists test files (`*_test.go`) and the spec mandates unit tests for upgrade logic and testscript CLI integration tests.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. The module path migration (US4 prerequisite) and version display enhancement are promoted to the Foundational phase because they block ALL user stories.

**Phase mapping** (tasks.md → plan.md): Tasks Phase 2 = Plan Phases 1+2 (Migration + Version), Tasks Phase 3 = Plan Phase 4 (Install Script), Tasks Phase 4 = Plan Phase 5 (Homebrew), Tasks Phase 5 = Plan Phase 3 (Self-Upgrade), Tasks Phase 6 = Plan Phase 2 verification (Go Install), Tasks Phase 7 = Plan Phase 6 (Documentation).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- Single Go CLI project at repository root
- New package: `internal/selfupdate/`
- New CLI command: `cmd/invowk/upgrade.go`
- New script: `scripts/install.sh`

---

## Phase 1: Setup

**Purpose**: No new project setup needed — this is an existing Go CLI project with established tooling (Make, Go 1.26+, Cobra, GoReleaser).

*(No tasks — proceed directly to Foundational phase)*

---

## Phase 2: Foundational — Module Path Migration & Version Enhancement

**Purpose**: Migrate the Go module path from `invowk-cli` to `github.com/invowk/invowk` and enhance version display for `go install` binaries. This is a prerequisite for ALL user stories: it enables `go install` (US4), the install script and Homebrew tap reference the correct GitHub path (US1, US2), and the upgrade command needs accurate version info (US3).

**Strategy** (per MEMORY.md "Large-scale sed rename safety"): Process longer patterns before shorter ones. Run `goimports` + `gofumpt` after sed. Verify with `go build` before `make test`.

- [X] T001 Update go.mod module path from `invowk-cli` to `github.com/invowk/invowk` and run `go mod tidy` in go.mod
- [X] T002 Update all Go file imports (~111 files, ~217 references) from `"invowk-cli/` to `"github.com/invowk/invowk/` using `sed -i` across cmd/, internal/, pkg/, and tests/cli/ directories
- [X] T003 [P] Update Makefile ldflags `-X` paths from `invowk-cli/cmd/invowk` to `github.com/invowk/invowk/cmd/invowk` in Makefile
- [X] T004 [P] Update .goreleaser.yaml ldflags `-X` paths from `invowk-cli/cmd/invowk` to `github.com/invowk/invowk/cmd/invowk` in .goreleaser.yaml
- [X] T005 Run `goimports -w .` and `gofumpt -w .` on all Go files to fix import grouping after bulk replacement (all project imports must stay in the same group — no blank line between internal/ and pkg/ imports)
- [X] T006 Verify compilation with `go build ./...` and resolve any remaining import reference errors
- [X] T007 [P] Update documentation references from `invowk-cli` to `github.com/invowk/invowk` in .claude/rules/, .claude/agents/, .claude/skills/, and specs/ markdown files
- [X] T008 Enhance `getVersionString()` with `debug.ReadBuildInfo()` fallback in cmd/invowk/root.go — if ldflags Version is "dev" and build info has a module version, use the module version; otherwise show "dev (built from source)"
- [X] T009 Run `make test` to verify all tests pass after module path migration and version enhancement

**Checkpoint**: Module path migration complete — `go install github.com/invowk/invowk@latest` is now possible and all existing functionality preserved.

---

## Phase 3: User Story 1 — One-Line Shell Script Installation (Priority: P1)

**Goal**: Provide a single `curl | sh` command that downloads, verifies, and installs invowk on Linux/macOS with zero prerequisites.

**Independent Test**: Run the one-liner on a fresh Linux/macOS system and verify `invowk --version` outputs the expected version.

### Implementation for User Story 1

- [X] T010 [US1] Create scripts/install.sh with POSIX sh shebang (`#!/bin/sh`), `set -eu` strict mode, `main()` function wrapper (FR-009: prevents partial execution from piped downloads), and `trap cleanup EXIT` for temp directory cleanup
- [X] T011 [US1] Implement platform detection and download logic in scripts/install.sh: `uname -s`/`uname -m` normalization (x86_64 to amd64, aarch64 to arm64), download tool auto-detection (curl with `-fsSL` or wget with `-qO-`), GitHub Releases `/releases/latest` endpoint for latest stable version discovery (or `/releases/tags/{tag}` when `INVOWK_VERSION` is set), and asset URL construction (`invowk_{version_without_v_prefix}_{os}_{arch}.tar.gz` — GoReleaser convention strips the `v` prefix)
- [X] T012 [US1] Implement verification and atomic installation in scripts/install.sh: SHA256 checksum download and verification (`sha256sum` on Linux, `shasum -a 256` on macOS), binary extraction from tar.gz to temp directory, atomic install to `INSTALL_DIR` (default `~/.local/bin`) with `mv`, PATH detection and setup instructions if install dir is not in `$PATH`
- [X] T013 [US1] Implement environment overrides and error handling in scripts/install.sh: `INVOWK_VERSION` for specific version (default latest stable), `INSTALL_DIR` for custom path, unsupported platform detection (Windows via uname, FreeBSD, 32-bit architectures) with clear error messages suggesting alternative methods, network error handling, and checksum mismatch handling — ensure no partial installations on any failure path
- [X] T014 [US1] Validate scripts/install.sh with `shellcheck` linting (no bashisms: no `[[ ]]`, no `local`, no arrays) and manual local testing with `INSTALL_DIR=$(mktemp -d) sh scripts/install.sh`

**Checkpoint**: Users can install invowk with `curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh` on any supported Unix platform.

### PowerShell Installer (Windows)

- [X] T037 [US1] Create scripts/install.ps1 with PowerShell installer for Windows (amd64 only): GitHub Releases API for latest version discovery (or specific version via `INVOWK_VERSION`), SHA256 checksum verification, install to `$env:LOCALAPPDATA\Programs\invowk` (override via `INSTALL_DIR`), automatic User PATH modification (opt-out via `INVOWK_NO_MODIFY_PATH=1`), optional `GITHUB_TOKEN` for API rate limit relief
- [X] T038 [US1] Implement PowerShell 5.1 compatibility in scripts/install.ps1: `[char]27` for ESC sequences (no `$([char]0x1b)` or `` `e ``), `$ProgressPreference='SilentlyContinue'` for fast downloads, `throw` instead of `exit` for irm|iex safety, `$env:PROCESSOR_ARCHITEW6432` for 32-on-64 detection
- [X] T039 [US1] Validate scripts/install.ps1 with manual testing and verify error paths: checksum mismatch, network failure, unsupported architecture (ARM64), existing installation overwrite

**Checkpoint**: Windows users can install invowk with `irm https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.ps1 | iex`.

---

## Phase 4: User Story 2 — Homebrew Installation (Priority: P2)

**Goal**: Enable `brew install invowk/tap/invowk` on macOS and Linux, with automatic formula updates on each release.

**Independent Test**: Run `brew install invowk/tap/invowk` on macOS/Linux with Homebrew and verify `invowk --version` works.

### Implementation for User Story 2

- [X] T015 [US2] Create `invowk/homebrew-tap` GitHub repository with README.md and Casks/ directory. Required: `HOMEBREW_TAP_TOKEN` fine-grained PAT with contents:write scope on the tap repo, to be added as a secret in the main repo
- [X] T016 [US2] Configure `homebrew_casks:` section in .goreleaser.yaml (migrated from deprecated `brews:` to `homebrew_casks` per GoReleaser v2.10+): tap repository (`invowk/homebrew-tap`), token (`HOMEBREW_TAP_TOKEN`), cask name, binaries list, macOS quarantine removal hook, skip_upload auto (skips pre-releases), and multi-platform support (macOS Intel + Apple Silicon, Linux amd64 + arm64)
- [X] T017 [US2] Validate GoReleaser Homebrew cask generation with `goreleaser check` (clean validation, no warnings) and `goreleaser release --snapshot --clean` (builds + archives succeed for all 5 targets; resolved config.yaml confirms correct cask settings)

**Checkpoint**: GoReleaser will auto-push the cask to `invowk/homebrew-tap` on each stable release. Users can `brew install invowk/tap/invowk`.

---

## Phase 5: User Story 3 — Self-Upgrade via CLI (Priority: P3)

**Goal**: Provide `invowk upgrade [version] [--check] [--yes]` that checks for and installs updates, detects managed installations, and ensures atomic binary replacement.

**Independent Test**: Install an older version, run `invowk upgrade`, and verify it updates to the latest stable with SHA256 verification.

### Implementation for User Story 3

- [-] T018 [P] [US3] [DEFERRED] Create internal/selfupdate/github.go with GitHub Releases API client using stdlib `net/http`: `ListReleases()` (paginated, client-side filter for stable releases — skip draft/prerelease, sort by semver), `GetReleaseByTag()` for specific version lookup, `DownloadAsset()` for binary download, common headers (`Accept: application/vnd.github+json`, `X-GitHub-Api-Version: 2022-11-28`, `User-Agent: invowk/{version}`), optional `GITHUB_TOKEN` auth via `Authorization: Bearer` header, and rate limit handling (parse `X-RateLimit-Remaining`/`X-RateLimit-Reset` headers, format reset time as human-readable UTC)
- [-] T019 [P] [US3] [DEFERRED] Create internal/selfupdate/github_test.go with `httptest.NewServer`-based tests: ListReleases filtering (stable only, skips drafts/prereleases), ListReleases pagination, GetReleaseByTag success and 404, rate limit error response with formatted reset time, authenticated requests include Bearer token, and correct User-Agent header
- [-] T020 [P] [US3] [DEFERRED] Create internal/selfupdate/detect.go with `InstallMethod` enum (`Unknown`, `Script`, `Homebrew`, `GoInstall`) and `DetectInstallMethod(execPath string)` function: (1) check build-time ldflags hint (highest priority), (2) path heuristics — Homebrew cellar paths (`/opt/homebrew/`, `/usr/local/Cellar/`, `/home/linuxbrew/.linuxbrew/`), GOPATH/bin (`$GOPATH/bin/` or `~/go/bin/`), (3) `debug.ReadBuildInfo()` module path confirmation for go-install, (4) fallback to `Unknown`
- [-] T021 [P] [US3] [DEFERRED] Create internal/selfupdate/detect_test.go with detection tests for all install methods: Homebrew path patterns (macOS ARM, macOS Intel, Linux), GOPATH/bin with mock build info, script install path (~/.local/bin), unknown/manual paths, and ldflags override taking priority over path heuristics
- [-] T022 [P] [US3] [DEFERRED] Create internal/selfupdate/checksum.go with `ParseChecksums(reader io.Reader) ([]ChecksumEntry, error)` — parse `checksums.txt` format (`{sha256_hex}  {filename}`, two-space separator) and `VerifyFile(filepath string, expectedHash string) error` — compute SHA256 of file and compare with expected hash
- [-] T023 [P] [US3] [DEFERRED] Create internal/selfupdate/checksum_test.go with tests: parse valid checksums.txt with multiple entries, parse malformed entries (wrong hash length, missing filename, empty lines), VerifyFile with matching hash, VerifyFile with mismatched hash, and asset not found in checksums.txt
- [-] T024 [US3] [DEFERRED] Create internal/selfupdate/selfupdate.go with `Updater` struct (holds GitHub client config, current version) and methods: `Check(targetVersion string) (*UpgradeCheck, error)` — resolve current version, query GitHub for target (latest stable or specific tag), compare with `golang.org/x/mod/semver`, determine upgrade eligibility (stable-only targets, pre-release protection, install method routing), return `UpgradeCheck` result; `Apply(release *Release) error` — resolve executable path via `os.Executable()` + `filepath.EvalSymlinks()`, download platform asset (`invowk_{version}_{os}_{arch}.tar.gz` — note: version in asset name strips the `v` prefix per GoReleaser convention), download and parse checksums.txt, verify archive SHA256, extract binary from tar.gz, atomic replacement (temp file in target dir → `os.Rename()`), preserve permissions via `os.Chmod()`, cleanup temp on any error via defer. **Windows guard**: if `runtime.GOOS == "windows"` and install method is `Unknown`, do NOT attempt binary replacement (Windows cannot rename running executables); instead return guidance to download manually or use `go install`
- [-] T025 [US3] [DEFERRED] Create internal/selfupdate/selfupdate_test.go with `Updater` tests: Check returns upgrade available (older→newer stable), Check returns up-to-date (same version), Check returns pre-release-ahead (v1.1.0-alpha.1 with latest stable v1.0.0), Check routes to Homebrew suggestion, Check routes to GoInstall suggestion, Apply success with checksum verification, Apply rolls back on checksum mismatch, Apply handles permission error gracefully
- [-] T026 [US3] [DEFERRED] Create cmd/invowk/upgrade.go with Cobra command `newUpgradeCommand()`: `invowk upgrade [version] [--check/-c] [--yes/-y]`, styled output showing current→target version, interactive confirmation prompt (skip with `--yes`), `--check` mode (display only, no install), install method detection routing (Homebrew → suggest `brew upgrade invowk`, GoInstall → suggest `go install github.com/invowk/invowk@latest`), exit codes (0=success/up-to-date/managed-install-guidance, 1=user error, 2=internal error), and error rendering via ServiceError pattern
- [-] T027 [US3] [DEFERRED] Create cmd/invowk/upgrade_test.go with command-level tests for output scenarios per CLI contract: upgrade available (interactive), already up-to-date, pre-release ahead of stable, --check mode, Homebrew detected, GoInstall detected, permission denied, network error, checksum mismatch, rate limited, and specific version target
- [-] T028 [US3] [DEFERRED] Register upgrade command in cmd/invowk/root.go by adding `rootCmd.AddCommand(newUpgradeCommand())` alongside existing command registrations

**Checkpoint**: Users can run `invowk upgrade` to check for and install updates, with full integrity verification and managed-install detection.

---

## Phase 6: User Story 4 — Go Install (Priority: P4)

**Goal**: Enable `go install github.com/invowk/invowk@latest` and ensure version display works correctly for go-install binaries.

**Independent Test**: Run `go install github.com/invowk/invowk@latest` and verify `invowk --version` displays the module version, not "dev".

### Implementation for User Story 4

> Note: The core implementation (module path migration and version display enhancement) was completed in Phase 2 (Foundational). This phase validates the user story end-to-end.

- [X] T029 [US4] Verify `go install github.com/invowk/invowk@latest` produces a working binary and `invowk --version` displays the module version (not "dev") when built via `go install` — test with `go install ./... && $(go env GOPATH)/bin/invowk --version`
- [X] T030 [US4] Add unit test for `getVersionString()` `debug.ReadBuildInfo()` fallback behavior in cmd/invowk/root_test.go — verify ldflags version takes priority, verify build info version used when ldflags is "dev", verify "dev (built from source)" shown when both are empty

**Checkpoint**: Go developers can install invowk via the standard Go toolchain and see correct version information.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Documentation updates, final integration testing, and compliance checks.

- [X] T031 [P] Rewrite installation section in README.md with all three methods (shell script one-liner, Homebrew tap, go install), verification steps (`invowk --version`), platform support matrix (Linux/macOS for script + Homebrew, all platforms for go install), and upgrade instructions
- [X] T032 [P] Create or update website installation guide in website/docs/getting-started/ with tabbed instructions for each installation method, using Docusaurus Tabs component
- [-] T033 [P] [DEFERRED] Add SPDX MPL-2.0 license headers to all new Go files: internal/selfupdate/github.go, internal/selfupdate/detect.go, internal/selfupdate/checksum.go, internal/selfupdate/selfupdate.go, and cmd/invowk/upgrade.go
- [-] T034 [DEFERRED] Create testscript (txtar) CLI integration tests for `invowk upgrade --check` and error paths in tests/cli/ (note: may require helper binary or env var for GitHub API mocking in testscript context)
- [-] T035 [DEFERRED] Verify `invowk upgrade --help` output matches CLI contract specification in specs/009-install-and-upgrade/contracts/cli-upgrade.md
- [X] T036 Run `make test`, `make lint`, `make license-check`, and `make tidy` for final verification of full test suite, linting, license compliance, and dependency tidiness

---

## Dependencies & Execution Order

### Phase Dependencies

```text
Phase 2 (Foundational: Migration + Version) ──→ Phase 3 (US1: Install Script)
                                              ──→ Phase 4 (US2: Homebrew)
                                              ──→ Phase 5 (US3: Self-Upgrade)
                                              ──→ Phase 6 (US4: Go Install verification)
Phases 3 + 4 + 5 + 6 ──→ Phase 7 (Polish & Documentation)
```

### User Story Dependencies

- **US1 (P1 — Install Script)**: Depends on Foundational (Phase 2) only — no dependencies on other user stories
- **US2 (P2 — Homebrew)**: Depends on Foundational (Phase 2) only — no dependencies on other user stories
- **US3 (P3 — Self-Upgrade)**: Depends on Foundational (Phase 2) for version info — no dependencies on US1 or US2
- **US4 (P4 — Go Install)**: Implementation is IN Foundational (Phase 2) — verification phase has no story dependencies; T030 (upgrade detection test) implicitly verifies after US3 exists

### Within Each User Story

- Implementation files marked [P] within the same story can be created in parallel
- Test files can be created alongside their implementation files
- Integration/registration tasks (e.g., T028 register command) depend on implementation tasks
- Verification tasks come after implementation

### Within Phase 2 (Foundational)

```text
T001 (go.mod) ──→ T002 (imports) ──→ T005 (goimports) ──→ T006 (build) ──→ T008 (version) ──→ T009 (test)
              ──→ T003 (Makefile) ──↗
              ──→ T004 (GoReleaser) ──↗
T007 (docs) is independent of all other foundational tasks
```

### Parallel Opportunities

**Phase 2**: T003, T004, T007 can all run in parallel (different files, independent of import updates)

**Phase 3-6**: All four user story phases can run in parallel after Foundational completes (if team capacity allows)

**Within US3 (Phase 5)**: T018-T023 (six files: github.go/test, detect.go/test, checksum.go/test) can all be created in parallel — they are independent packages within `internal/selfupdate/`

**Phase 7**: T031, T032, T033 can run in parallel (different files)

---

## Parallel Example: User Story 3 (Self-Upgrade)

```bash
# Launch all independent selfupdate package files together:
Task: "Create internal/selfupdate/github.go — GitHub Releases API client"
Task: "Create internal/selfupdate/github_test.go — API client tests"
Task: "Create internal/selfupdate/detect.go — install method detection"
Task: "Create internal/selfupdate/detect_test.go — detection tests"
Task: "Create internal/selfupdate/checksum.go — SHA256 verification"
Task: "Create internal/selfupdate/checksum_test.go — checksum tests"

# Then sequentially (depends on all above):
Task: "Create internal/selfupdate/selfupdate.go — Updater type (uses github, detect, checksum)"
Task: "Create internal/selfupdate/selfupdate_test.go — Updater tests"

# Then CLI layer (depends on selfupdate package):
Task: "Create cmd/invowk/upgrade.go — Cobra command"
Task: "Create cmd/invowk/upgrade_test.go — command tests"
Task: "Register upgrade command in cmd/invowk/root.go"
```

---

## Implementation Strategy

### MVP First (Foundational + Install Script)

1. Complete Phase 2: Foundational (Module Path Migration + Version Enhancement)
2. Complete Phase 3: US1 — Install Script
3. **STOP and VALIDATE**: Test install script on Linux/macOS, verify `invowk --version`
4. This gives users a working installation path immediately

### Incremental Delivery

1. Foundational → Migration complete, `go install` works (US4)
2. Add US1 (Install Script) → Test independently → Users can `curl | sh` install
3. Add US3 (Self-Upgrade) → Test independently → Users can `invowk upgrade`
4. Add US2 (Homebrew) → Test independently → Users can `brew install`
5. Polish & Documentation → Complete the story
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers after Foundational completes:
- **Developer A**: US1 (Install Script) — standalone shell script
- **Developer B**: US3 (Self-Upgrade) — Go package + CLI command (heaviest workload)
- **Developer C**: US2 (Homebrew) — GoReleaser config + tap repo setup
- Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- Each user story is independently completable and testable after Foundational phase
- The module path migration (Phase 2) is the highest-risk phase — it touches ~111 files. Run `go build ./...` early to catch errors.
- The install script (US1) is a standalone POSIX sh file with no Go dependencies — it can be developed and tested independently.
- US2 (Homebrew) requires manual GitHub repository creation and secret configuration — these are documented in T015 but cannot be automated.
- All new Go files MUST have SPDX MPL-2.0 license headers (T033).
- Commit after each task or logical group to maintain clean history.
