> **Status Update (2026-02-13):** User Story 3 (Self-Upgrade via CLI) has been removed
> from this branch. The `UpgradeCheck`, `InstallMethod`, and `ChecksumEntry` types described
> here were deleted along with `internal/selfupdate/`.
> This document is retained as design history. See the branch commit log for details.

# Data Model: Installation Methods & Self-Upgrade

**Phase 1 Output** | **Date**: 2026-02-13

## Entities

### Release

Represents a published version of invowk on GitHub Releases.

```go
// Release represents a GitHub Release with its assets.
type Release struct {
    TagName    string   // Semantic version tag, e.g., "v1.0.0"
    Name       string   // Human-readable release name
    Prerelease bool     // True for alpha/beta/RC releases
    Draft      bool     // True for unpublished drafts
    Assets     []Asset  // Downloadable artifacts
    HTMLURL    string   // Browser URL for the release page
    CreatedAt  string   // ISO 8601 timestamp
}
```

**Validation rules**:
- `TagName` must be valid semver prefixed with `v`
- Stable releases: `Prerelease == false && Draft == false`
- Must have at least one asset matching the current platform

### Asset

Represents a downloadable artifact within a release.

```go
// Asset represents a single downloadable file in a GitHub Release.
type Asset struct {
    Name               string // Filename, e.g., "invowk_1.0.0_linux_amd64.tar.gz"
    BrowserDownloadURL string // Direct download URL
    Size               int64  // File size in bytes
    ContentType        string // MIME type
}
```

**Naming convention** (from GoReleaser):
- Binary archives: `invowk_{version}_{os}_{arch}.tar.gz` (Linux/macOS), `.zip` (Windows)
- Checksums: `checksums.txt`

### InstallMethod

Enum representing how invowk was installed on the current system.

```go
// InstallMethod identifies how invowk was installed.
type InstallMethod int

const (
    InstallMethodUnknown   InstallMethod = iota // Unknown or manual download
    InstallMethodScript                          // Shell script (~/.local/bin)
    InstallMethodHomebrew                        // Homebrew (managed by brew)
    InstallMethodGoInstall                       // go install (managed by go toolchain)
)
```

**State transitions**: None — InstallMethod is detected at runtime and is immutable for the lifetime of the binary.

**Detection priority**:
1. Build-time ldflags hint (`-X installMethod=...`)
2. Path heuristics (Homebrew cellar paths, GOPATH/bin)
3. `debug.ReadBuildInfo()` module path (confirms go-install)
4. Fallback to `Unknown`

### UpgradeCheck

Result of checking for available upgrades.

```go
// UpgradeCheck contains the result of checking for an available upgrade.
type UpgradeCheck struct {
    CurrentVersion string        // Currently running version
    LatestVersion  string        // Latest stable release version
    TargetRelease  *Release      // Full release info for the target version (nil if up-to-date)
    InstallMethod  InstallMethod // How invowk was installed
    UpgradeAvailable bool        // True if an upgrade is available and applicable
    Message        string        // Human-readable status message
}
```

**Relationships**:
- `TargetRelease` → `Release` (nullable: nil when already up-to-date)
- `InstallMethod` → `InstallMethod` enum

### ChecksumEntry

A single line from `checksums.txt`.

```go
// ChecksumEntry represents a SHA256 checksum for a release asset.
type ChecksumEntry struct {
    Hash     string // Hex-encoded SHA256 hash
    Filename string // Asset filename this hash applies to
}
```

**Validation rules**:
- `Hash` must be exactly 64 hex characters (SHA256)
- `Filename` must match a known asset naming pattern

## Relationships

```text
Release 1──* Asset          (a release contains multiple platform-specific assets)
Release 1──1 ChecksumEntry  (each asset has exactly one checksum in checksums.txt)
UpgradeCheck *──1 Release   (check references the target release, if any)
UpgradeCheck 1──1 InstallMethod (check includes detected install method)
```

## Type Design Notes

- All types are in `internal/selfupdate/` — they are internal to the upgrade subsystem and not exported to other packages.
- `Release` and `Asset` mirror the GitHub API response shape but only include fields we need (not the full 30+ field GitHub release object). This is intentional — mapping the full API response would violate Principle V (Simplicity).
- `InstallMethod` is an `int` enum rather than a `string` to enable exhaustive switch statements and avoid typo-class bugs.
- `UpgradeCheck` is a value type returned by `Updater.Check()` — it bundles all the information the CLI layer needs to display status and decide whether to proceed.
