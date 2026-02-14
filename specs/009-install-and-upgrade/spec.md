> **Status Update (2026-02-13):** User Story 3 (Self-Upgrade via CLI) has been removed
> from this branch. The `internal/selfupdate/` package and `cmd/invowk/upgrade.go` were
> deleted. This document is retained as design history. See the branch commit log for details.

# Feature Specification: Installation Methods & Self-Upgrade

**Feature Branch**: `009-install-and-upgrade`
**Created**: 2026-02-13
**Status**: Implemented (US3 removed; US1, US2, US4 complete)
**Input**: User description: "Design and implement 3 officially supported installation methods (go install, homebrew, install.sh script) and an invowk upgrade command to update to the latest stable release."

## Clarifications

### Session 2026-02-13

- Q: Is the module path migration (invowk-cli → github.com/invowk/invowk) in-scope for this feature or a separate prerequisite? → A: In-scope — implemented as part of this feature branch.
- Q: How should the upgrade command detect the install method (Homebrew, go install, script)? → A: Hybrid — path-based heuristics as primary detection (Homebrew cellar prefixes, GOPATH/bin), with an optional build-time hint via -ldflags as override for edge cases.
- Q: Should the upgrade command verify Cosign signatures in addition to SHA256 checksums? → A: SHA256-only — verify against checksums.txt fetched over HTTPS from GitHub Releases. No external tool dependency (cosign not required). Cosign verification remains available for manual use by security-conscious users.
- Q: Should `invowk upgrade` support upgrading to a specific version, not just latest? → A: Yes — `invowk upgrade [version]` accepts an optional target version argument, defaulting to latest stable when omitted. No pre-release opt-in flag.
- Q: Should the upgrade command depend on `creativeprojects/go-selfupdate` or implement the logic directly? → A: Reference only — implement upgrade logic directly using Go stdlib + GitHub Releases API, informed by go-selfupdate's patterns. This avoids fighting library opinions on detection/verification and keeps dependencies lean.

## User Scenarios & Testing *(mandatory)*

### User Story 1 — One-Line Shell Script Installation (Priority: P1)

A user discovers invowk and wants to install it immediately with a single terminal command on their Linux or macOS machine. They expect the installation to detect their platform automatically, download the correct binary, verify its integrity, and place it in a usable location on their PATH.

**Why this priority**: This is the lowest-friction installation path that works on all Unix-like platforms without requiring any prerequisites (no Go toolchain, no Homebrew). First thing new users will try.

**Independent Test**: Run the one-liner on a fresh Linux/macOS system and verify `invowk --version` outputs the expected version.

**Acceptance Scenarios**:

1. **Given** a macOS (Intel or Apple Silicon) or Linux (x86-64 or ARM64) machine with `curl`/`wget` and a POSIX shell, **When** the user runs `curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh`, **Then** the invowk binary is downloaded, verified via SHA256 checksum, and installed to `~/.local/bin`.
2. **Given** the script is run for the first time and `~/.local/bin` is not in the user's PATH, **When** installation completes, **Then** the script prints instructions for adding the directory to PATH.
3. **Given** the user wants a specific version, **When** they set `INVOWK_VERSION=v1.0.0` before running the script, **Then** that exact version is installed instead of the latest.
4. **Given** the user wants to install to a custom location, **When** they set `INSTALL_DIR=/opt/bin` before running, **Then** the binary is placed in that directory.
5. **Given** the download or checksum verification fails, **When** the script encounters the error, **Then** it exits with a non-zero code, a clear error message, and leaves no partial installation.
6. **Given** the user's platform is Windows or an unsupported architecture, **When** they run the script, **Then** it exits with a clear error message suggesting alternative installation methods.

---

### User Story 2 — Homebrew Installation (Priority: P2)

A macOS or Linux user with Homebrew wants to install invowk using the familiar `brew install` workflow, and update it via `brew upgrade` as with any other Homebrew package.

**Why this priority**: Homebrew is the de facto package manager for macOS developers and increasingly used on Linux. It provides automatic updates and a familiar experience.

**Independent Test**: Run `brew install invowk/tap/invowk` on macOS/Linux with Homebrew and verify the binary works.

**Acceptance Scenarios**:

1. **Given** Homebrew is installed on macOS or Linux, **When** the user runs `brew install invowk/tap/invowk`, **Then** the latest stable version of invowk is installed and available on PATH.
2. **Given** a new stable version is released, **When** the user runs `brew upgrade invowk`, **Then** the binary is updated to the latest stable version.
3. **Given** a new stable GitHub Release is published, **When** the release pipeline completes, **Then** the Homebrew formula in the tap repository is automatically updated with new version, checksums, and download URLs — no manual intervention.
4. **Given** both Intel and Apple Silicon Macs exist, **When** the formula is installed on either architecture, **Then** the correct binary for that architecture is downloaded.

---

### User Story 3 — Self-Upgrade via CLI (Priority: P3)

> **Status: Removed/Deferred.** This user story was removed from the implementation scope.
> The `internal/selfupdate/` package and `cmd/invowk/upgrade.go` were deleted.
> The content below is retained as design history.

A user who already has invowk installed (via any method) wants to check for and install updates directly from the command line, without needing to remember which installation method they used.

**Why this priority**: Once users are invested in invowk, frictionless updates keep them on the latest stable version. The upgrade command is the natural complement to installation.

**Independent Test**: Install an older version, run `invowk upgrade`, and verify it updates to the latest stable.

**Acceptance Scenarios**:

1. **Given** the user has an older stable version, **When** they run `invowk upgrade`, **Then** the binary is updated to the latest stable release and the new version is displayed.
2. **Given** the user already has the latest stable version, **When** they run `invowk upgrade`, **Then** a message confirms they are already up to date.
3. **Given** the user has a pre-release version (e.g., v1.1.0-alpha.1) whose semver is higher than the latest stable (e.g., v1.0.0), **When** they run `invowk upgrade`, **Then** no downgrade occurs and the user is informed they are on a pre-release ahead of the latest stable.
4. **Given** the user installed invowk via Homebrew, **When** they run `invowk upgrade`, **Then** the command detects the Homebrew-managed installation and suggests using `brew upgrade invowk` instead.
5. **Given** the binary is in a location requiring elevated permissions, **When** the user runs `invowk upgrade` without sufficient permissions, **Then** a clear error message explains the permission issue and suggests running with appropriate privileges.
6. **Given** a network error occurs during upgrade, **When** the error happens, **Then** the original binary is left intact and the user sees a clear error message.
7. **Given** the user wants to see what's available without upgrading, **When** they run `invowk upgrade --check`, **Then** the latest available version is displayed without installing it.
8. **Given** the user wants a specific stable version, **When** they run `invowk upgrade v1.2.0`, **Then** that exact version is downloaded, verified, and installed (subject to the same atomic replacement and integrity checks).

---

### User Story 4 — Go Install (Priority: P4)

A Go developer wants to install invowk using `go install`, the standard Go toolchain method.

**Why this priority**: While `go install` is the idiomatic Go method, it has inherent limitations (no version metadata injection, no UPX compression, requires Go toolchain). It's a convenience for Go users, not the primary path.

**Prerequisite**: The Go module path must be changed from `invowk-cli` to `github.com/invowk/invowk`. This is a large-scale refactor tracked as a dependency within this spec.

**Independent Test**: Run `go install github.com/invowk/invowk@latest` and verify the binary works.

**Acceptance Scenarios**:

1. **Given** Go 1.26+ is installed, **When** the user runs `go install github.com/invowk/invowk@latest`, **Then** the invowk binary is installed to `$GOPATH/bin`.
2. **Given** the binary was installed via `go install`, **When** the user runs `invowk --version`, **Then** the version displays the module version (not `dev`) by reading Go build info from the binary.
3. **Given** the binary was installed via `go install`, **When** the user runs `invowk upgrade`, **Then** the command detects the go-install method and suggests using `go install github.com/invowk/invowk@latest` instead.

---

### Edge Cases

- **Unsupported platform**: Script run on FreeBSD, 32-bit Linux, or other unsupported OS/arch exits with clear error and link to supported platforms.
- **GitHub API rate limit**: Upgrade check on rate-limited connection fails gracefully with suggestion to try later or set a GitHub token.
- **Disk full**: Original binary preserved (atomic replacement pattern); clear error message.
- **Multiple binaries on PATH**: Upgrade command updates only the running binary's path (resolved via `/proc/self/exe` on Linux, `os.Executable()` cross-platform).
- **Pre-release force downgrade**: A `--force` flag could override pre-release protection, but the default behavior is safe (no downgrade).
- **Interrupted download**: Function wrapping ensures no partial state; temp files are cleaned up on exit.
- **Missing checksums.txt**: Install script and upgrade command refuse to proceed without integrity verification.
- **Homebrew tap unavailable**: Standard Homebrew error handling applies.
- **Windows install script**: The POSIX `install.sh` script detects Windows and exits with a message to use the PowerShell installer (`scripts/install.ps1`), `go install`, or manual binary download. The PowerShell installer (`install.ps1`) provides native Windows support (amd64 only).
- **Windows upgrade with manual binary**: On Windows, if install method detection returns Unknown (manual download), the upgrade command MUST NOT attempt direct binary replacement (Windows cannot rename a running executable). Instead, it should suggest downloading the new version manually from GitHub Releases or using `go install`.

## Requirements *(mandatory)*

### Functional Requirements

**Shell Install Script:**
- **FR-001**: System MUST provide a shell install script downloadable via a single `curl` or `wget` command from `https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh`.
- **FR-002**: The install script MUST detect the user's operating system (Linux, macOS) and CPU architecture (x86-64, ARM64) automatically.
- **FR-003**: The install script MUST verify the downloaded binary's integrity using SHA256 checksums from the release.
- **FR-004**: The install script MUST install to `~/.local/bin` by default, with an override via `INSTALL_DIR` environment variable.
- **FR-005**: The install script MUST support installing a specific version via `INVOWK_VERSION` environment variable, defaulting to the latest stable release.
- **FR-006**: The install script MUST NOT leave partial installations on failure (atomic install-or-nothing).
- **FR-007**: The install script MUST print PATH setup instructions if the install directory is not in the user's PATH.
- **FR-008**: The install script MUST refuse to run on unsupported platforms with a clear error message.
- **FR-009**: The entire script body MUST be wrapped in a function to prevent partial execution from incomplete piped downloads.

**Homebrew:**
- **FR-010**: System MUST maintain a Homebrew tap repository (`invowk/homebrew-tap`) with a formula/cask for invowk.
- **FR-011**: The Homebrew formula MUST be automatically updated when a new stable GitHub Release is published (via GoReleaser integration).
- **FR-012**: The Homebrew formula MUST support macOS (Intel + Apple Silicon) and Linux (x86-64 + ARM64).

**Self-Upgrade Command:**
- **FR-013**: System MUST provide an `invowk upgrade [version]` command that updates the binary to the latest stable release (when no version is specified) or to an explicitly specified stable version.
- **FR-014**: The upgrade command MUST only consider stable releases (pre-releases are excluded from upgrade targets).
- **FR-015**: The upgrade command MUST NOT downgrade when the current version is a pre-release with semver higher than the latest stable.
- **FR-016**: The upgrade command MUST detect if invowk was installed via a package manager (Homebrew, go install) and suggest the appropriate upgrade method instead of performing a direct binary replacement. Detection uses a hybrid approach: (1) path-based heuristics as primary — Homebrew cellar prefixes (`/opt/homebrew/`, `/home/linuxbrew/`, `/usr/local/Cellar/`), `$GOPATH/bin` for go install, `debug.ReadBuildInfo` module path for go-install confirmation; (2) optional build-time `-ldflags` hint as override for edge cases.
- **FR-017**: The upgrade command MUST verify downloaded binary integrity via SHA256 checksum (from `checksums.txt` fetched over HTTPS from GitHub Releases) before replacing the current binary. Cosign signature verification is not required — it remains available for manual verification by security-conscious users.
- **FR-018**: The upgrade command MUST preserve the current binary if the upgrade fails at any point (atomic replacement).
- **FR-019**: The upgrade command MUST provide a `--check` flag that reports the latest available version without installing.
- **FR-020**: The upgrade command MUST display a summary (current version -> new version) and confirm before proceeding (unless `--yes` flag is passed).

**Go Install (Prerequisite: Module Path Migration):**
- **FR-021**: The Go module path MUST be changed from `invowk-cli` to `github.com/invowk/invowk` to enable `go install` support.
- **FR-022**: When installed via `go install`, `invowk --version` MUST display the module version (not `dev`) by reading Go build info from the binary.

**Documentation:**
- **FR-023**: Installation documentation (website + README) MUST be updated to describe all three official methods with clear instructions.
- **FR-024**: Each installation method MUST include verification steps (e.g., `invowk --version`).

### Key Entities

- **Release**: A published version of invowk on GitHub Releases, containing platform-specific archives and SHA256 checksums (with optional Cosign signatures available for manual verification).
- **Install Method**: The mechanism by which invowk was installed (shell script, Homebrew, go install, manual). The upgrade command uses this to determine the appropriate update path.
- **Version**: A semantic version string (e.g., v1.0.0, v1.1.0-alpha.1) that determines upgrade eligibility. Only stable versions (no pre-release suffix) are upgrade targets.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can install invowk on macOS or Linux in under 30 seconds using a single terminal command (shell script or Homebrew).
- **SC-002**: All installation methods produce a working binary that reports the correct version via `invowk --version`.
- **SC-003**: Users can upgrade to the latest stable release with a single `invowk upgrade` command in under 60 seconds.
- **SC-004**: The upgrade command never corrupts or loses the existing binary, even on failure (100% atomic replacement).
- **SC-005**: 100% of stable releases automatically update the Homebrew formula without manual intervention.
- **SC-006**: The install script correctly detects and installs for all 4 supported Unix platform/architecture combinations (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64).
- **SC-007**: The upgrade command correctly prevents downgrade from pre-release to older stable 100% of the time.

## Assumptions

- The `invowk/homebrew-tap` GitHub repository will be created as part of this work.
- A `HOMEBREW_TAP_TOKEN` secret with write access to the tap repo will be configured in the main repo's CI.
- The install script will use POSIX sh (not bash) for maximum portability.
- The upgrade command will be implemented directly using Go stdlib + GitHub Releases API (no third-party upgrade library). `creativeprojects/go-selfupdate` serves as a design reference only.
- The module path migration (`invowk-cli` -> `github.com/invowk/invowk`) is in-scope for this feature branch. It is a prerequisite for `go install` support and will be implemented as an early phase of this work, touching all Go import paths across the codebase.
- Windows users can use the PowerShell installer (`scripts/install.ps1`), `go install`, or manual binary download (no Homebrew support on Windows).
