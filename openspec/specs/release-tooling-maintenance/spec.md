# release-tooling-maintenance Specification

## Purpose
Define the release tooling maintenance contract so GoReleaser version tracks, package-manager metadata, and release validation stay synchronized across CI, release publishing, and repository governance.
## Requirements
### Requirement: GoReleaser track is synchronized
The release automation SHALL use one documented GoReleaser v2 version range across all workflow invocations that validate, dry-run, or publish release artifacts.

#### Scenario: GoReleaser track is updated
- **WHEN** maintainers update the GoReleaser version track
- **THEN** every `goreleaser/goreleaser-action` invocation in CI and release workflows MUST use the same version range
- **AND** `.agents/rules/version-pinning.md` MUST document that same current track

#### Scenario: Pull request validation matches release publishing
- **WHEN** CI runs GoReleaser config checks or release dry runs
- **THEN** those jobs MUST resolve the same GoReleaser track used by the real release publishing job

### Requirement: GoReleaser configuration is validated before adoption
Changes to the GoReleaser track or release packaging configuration SHALL be validated before they are considered ready for implementation completion.

#### Scenario: GoReleaser config is checked
- **WHEN** the release tooling change updates GoReleaser versioning or `.goreleaser.yaml`
- **THEN** maintainers MUST run a GoReleaser configuration check using the updated GoReleaser track
- **AND** the check MUST complete without deprecation or schema errors

#### Scenario: Release dry run exercises packaging
- **WHEN** the updated release tooling is validated
- **THEN** maintainers MUST run a snapshot release dry run that exercises archive generation, checksum generation, signing setup, Homebrew template expansion, and WinGet template expansion
- **AND** unexpected release artifact shape changes MUST be fixed or explicitly deferred before implementation tasks are marked complete

### Requirement: Homebrew cask exposes Invowk completions
The Homebrew cask release metadata SHALL generate shell completions from Invowk's installed executable using the existing Cobra completion command.

#### Scenario: Homebrew cask is generated
- **WHEN** GoReleaser generates the Homebrew cask for a stable release
- **THEN** the cask MUST configure completion generation from the installed `invowk` executable
- **AND** the completion command MUST use Invowk's existing `completion` subcommand rather than committed generated completion files

#### Scenario: Supported completion shells are configured
- **WHEN** the Homebrew cask completion configuration is evaluated
- **THEN** it MUST include Invowk-supported shells for bash, zsh, fish, and PowerShell unless GoReleaser or Homebrew validation rejects one of those shells

### Requirement: Existing release contracts remain stable
The GoReleaser update SHALL preserve existing release notes, signing, checksum, WinGet, benchmark, and installer download contracts unless a focused validation failure requires a minimal adjustment.

#### Scenario: Reviewed release notes are published
- **WHEN** the release workflow invokes GoReleaser after the update
- **THEN** it MUST continue passing the recovered release notes file through GoReleaser's release-notes input

#### Scenario: Checksums remain signed
- **WHEN** GoReleaser creates release checksums
- **THEN** the release configuration MUST continue signing `checksums.txt` with the existing Cosign keyless signing flow

#### Scenario: WinGet enhancement remains in place
- **WHEN** a stable non-dry-run release creates or updates a WinGet pull request
- **THEN** the workflow MUST continue running the repository's WinGet manifest enhancement step after GoReleaser publishes the WinGet manifests

### Requirement: Release binary tool pins are synchronized
The release automation SHALL keep inline binary tool versions synchronized across CI release checks and real release publishing.

#### Scenario: UPX version is updated
- **WHEN** maintainers update the pinned UPX version used for release artifact compression
- **THEN** every workflow that installs UPX for release checks or release publishing MUST use the same version
- **THEN** `.agents/rules/version-pinning.md` MUST document that same current version

#### Scenario: Cosign version is updated
- **WHEN** maintainers update the pinned Cosign release used for artifact signing
- **THEN** every workflow that installs Cosign for release checks or release publishing MUST use the same `cosign-release` value
- **THEN** the selected `sigstore/cosign-installer` action version MUST support that Cosign release
- **THEN** `.agents/rules/version-pinning.md` MUST document the current Cosign version

### Requirement: Release tooling upgrades preserve artifact contracts
Release tooling upgrades SHALL preserve existing signing, checksum, compression, notes, package-manager, and benchmark publication behavior unless a separate proposal changes those contracts.

#### Scenario: Snapshot release dry run validates artifact shape
- **WHEN** release tooling pins for GoReleaser, Cosign, UPX, or release workflow actions change
- **THEN** maintainers MUST run a GoReleaser configuration check and snapshot release dry run
- **THEN** unexpected changes to archive names, checksums, signatures, package-manager metadata, or release notes handling MUST be fixed or documented before completion

#### Scenario: Signing verification remains available
- **WHEN** release signing tooling is updated
- **THEN** the release workflow MUST continue producing signed checksums through the existing keyless signing flow
- **THEN** user-facing checksum verification instructions MUST remain accurate or be updated in the same change

### Requirement: Release workflow action updates are reviewed as release changes
GitHub Actions updates that affect release validation or publishing SHALL be treated as release-tooling maintenance, not generic workflow churn.

#### Scenario: Release workflow actions move majors
- **WHEN** a GitHub Action used by release checks or release publishing moves to a newer major version
- **THEN** every release workflow invocation of that action MUST be reviewed for input, permission, output, and runner behavior changes
- **THEN** release dry-run or equivalent workflow validation MUST cover the updated action path before completion
