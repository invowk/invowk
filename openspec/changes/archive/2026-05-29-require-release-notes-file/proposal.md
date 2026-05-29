## Why

Invowk release tags currently rely on GoReleaser's generated GitHub-native changelog, so maintainers cannot require a reviewed markdown release note as the body shown on the GitHub Releases page. Releases should make the human-authored notes file explicit and mandatory so the published release page is deliberate, reviewable, and tied to the exact tag being shipped.

## What Changes

- **BREAKING**: Require `RELEASE_NOTES_FILE=<path>` for the release make targets that publish or prepare a real release tag.
- Validate that the release notes path exists, is a regular non-empty markdown file, and is available before any tag is created or pushed.
- Carry the markdown content through the local tag creation and GitHub Actions release workflow so GoReleaser publishes that content on the GitHub Releases page.
- Preserve dry-run behavior as a preview path that validates inputs and shows the computed release, without creating or pushing tags.
- Keep GitHub-native changelog generation from silently replacing the required release notes file for real releases.

## Capabilities

### New Capabilities

- `release-notes-publication`: Release commands require a markdown release notes file and publish its content as the GitHub Release body.

### Modified Capabilities

- None.

## Impact

- `Makefile` release and release-bump targets, usage text, help output, and examples.
- `scripts/release.sh` argument parsing, validation, dry-run summary, signed tag creation, and tag-message handoff.
- `.github/workflows/release.yml` GoReleaser invocation, release-notes extraction, and manual-dispatch handling.
- `.goreleaser.yaml` changelog/release behavior only as needed to ensure the provided markdown file becomes the GitHub Release body.
- Shell tests or workflow validation coverage for missing, empty, invalid, and successful release notes inputs.
