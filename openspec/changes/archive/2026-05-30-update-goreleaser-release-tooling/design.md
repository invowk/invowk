## Context

Invowk publishes release artifacts through GoReleaser in both the general CI dry-run path and the tag/manual release workflow. Every GoReleaser action invocation currently uses `version: "~> v2.15"`, while upstream GoReleaser has moved to the v2.16 release line with release-publishing fixes relevant to Invowk's GitHub Release, prerelease, Homebrew, and WinGet flow.

The current `.goreleaser.yaml` already uses `homebrew_casks`, signs `checksums.txt` with Cosign, publishes WinGet manifests, preserves reviewed release notes with `--release-notes`, and keeps Homebrew upload limited to stable releases. Invowk also already exposes `invowk completion bash|zsh|fish|powershell`, but the Homebrew cask only installs the binary and does not generate completions for Homebrew users.

## Goals / Non-Goals

**Goals:**
- Move every GoReleaser action invocation and the version-pinning rule to the same v2.16 range.
- Add Homebrew cask completion generation using Invowk's existing Cobra completion command.
- Preserve current release notes, signing, checksums, WinGet enhancement, benchmark, and install-script contracts.
- Verify the updated config with GoReleaser v2.16 and the release dry-run path before implementation is considered complete.

**Non-Goals:**
- Add GoReleaser Node, Flatpak, Source RPM, Nix, Docker v2 image publishing, or `xz` release archives.
- Change GoReleaser action major version, Cosign version, UPX version, or release-note publication behavior.
- Replace the existing WinGet enhancement script unless GoReleaser validation shows the manifest shape changed.
- Add signed/notarized macOS artifacts or remove the existing cask quarantine-removal hook.

## Decisions

### Keep a semver range, but advance it to v2.16

Use `version: "~> v2.16"` everywhere GoReleaser runs. This preserves the repository's current range-based maintenance model while allowing v2.16 patch releases to bring release-pipeline fixes. The alternative was pinning `v2.16.0` exactly, which would improve byte-for-byte repeatability but conflict with the existing `.agents/rules/version-pinning.md` policy that treats GoReleaser as a semver-track action input.

### Update all GoReleaser references as a sync pair

Change `.github/workflows/ci.yml`, `.github/workflows/release.yml`, and `.agents/rules/version-pinning.md` together. The release and dry-run workflows exercise the same `.goreleaser.yaml`; allowing one path to lag would make pull-request validation less predictive of tag-day behavior.

### Generate Homebrew completions at install time

Use `homebrew_casks.generate_completions_from_executable` with `shell_parameter_format: cobra`, `args: ["completion"]`, and explicit shells for bash, zsh, fish, and PowerShell. This uses Invowk's existing completion command without committing generated completion files or adding another release artifact type.

Alternative considered: pre-generate completion files during release and list them under `homebrew_casks.completions`. That would add more release outputs and tests while duplicating behavior already available from the binary.

### Keep release artifacts and channels stable

Do not adopt v2.16's new `xz` archive support or Docker v2 publishing in this change. Archive format changes would touch install scripts, checksum expectations, docs, and users' existing download assumptions. Docker v2 is promising for a future official image, but Invowk's current Docker use in release workflows is Bencher image packaging, not a GoReleaser-published end-user artifact.

## Risks / Trade-offs

- GoReleaser v2.16 changes release behavior in ways not covered by `goreleaser check` -> Run the snapshot release dry-run path that exercises archive, checksum, signing setup, Homebrew template expansion, and WinGet template expansion.
- Homebrew completion generation might not compose with the existing `postflight` quarantine hook -> Validate with GoReleaser v2.16 and inspect the generated cask in `dist/` during release dry run if validation output changes.
- PowerShell completion generation may be less useful to Homebrew users than bash/zsh/fish -> Keep it explicit because Invowk supports it and GoReleaser's Cobra completion format supports `pwsh`; remove only if generated cask validation rejects it.
- Updating `.agents/rules/version-pinning.md` can create agent-doc drift -> Run `make check-agent-docs` after editing agent guidance.

## Migration Plan

1. Update every GoReleaser action `version` input from `~> v2.15` to `~> v2.16`.
2. Update `.agents/rules/version-pinning.md` to document the v2.16 GoReleaser track.
3. Add Homebrew cask completion generation to `.goreleaser.yaml`.
4. Run GoReleaser v2.16 config validation, release dry-run validation, WinGet enhancement tests, and agent-doc validation.
5. If any release dry-run artifact shape changes unexpectedly, keep the version bump but defer cask completion generation behind a follow-up fix only if necessary.

Rollback is straightforward before publication: restore the GoReleaser range and cask completion stanza. After publication, repair forward with a patch release if release artifacts were already created.

## Open Questions

- Should Homebrew cask completion generation include PowerShell immediately, or should the first implementation limit the cask to bash, zsh, and fish if generated `pwsh` install behavior is noisy?
