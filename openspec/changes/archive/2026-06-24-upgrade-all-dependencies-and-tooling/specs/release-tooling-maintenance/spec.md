## ADDED Requirements

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
