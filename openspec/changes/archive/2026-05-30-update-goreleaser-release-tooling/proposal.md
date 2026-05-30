## Why

Invowk's release pipeline is one GoReleaser minor behind current upstream, missing recent GitHub publishing reliability fixes and newer Homebrew cask capabilities. Updating the release toolchain now reduces tag-day risk and lets Homebrew users receive shell completion integration through the cask.

## What Changes

- Update every GoReleaser action version range from the v2.15 track to the v2.16 track.
- Update repository version-pinning guidance so the documented GoReleaser track matches CI and release workflows.
- Add Homebrew cask completion generation so Homebrew installs can expose Invowk's existing Cobra completions for supported shells.
- Preserve existing release notes, Cosign signing, checksum, WinGet, benchmark, and install-script behavior unless validation proves a minimal adjustment is required.
- Validate the updated release configuration with the current GoReleaser check and release dry-run paths.

## Capabilities

### New Capabilities
- `release-tooling-maintenance`: Release automation stays on an explicitly pinned, validated GoReleaser track and package-manager metadata exposes supported Invowk shell integrations.

### Modified Capabilities
- None.

## Impact

- Affected workflows: `.github/workflows/ci.yml` and `.github/workflows/release.yml`.
- Affected release configuration: `.goreleaser.yaml`.
- Affected agent/version governance: `.agents/rules/version-pinning.md`.
- Affected validation: GoReleaser config check, release dry run, and agent-doc sync checks if agent guidance changes.
- User-visible impact: Homebrew cask installations gain shell completion generation using Invowk's existing `completion` command.
