## Context

`make release` and `make release-bump` currently create and push a signed tag, then GitHub Actions runs GoReleaser to publish the GitHub Release. The local command only passes version-selection fields into `scripts/release.sh`, while the release workflow later runs `goreleaser release --clean` with `changelog.use: github-native`.

The release notes file starts on the maintainer's machine, but the GitHub Release body is produced in CI. A plain local path is therefore not enough: the workflow needs a durable, tag-bound handoff that survives the push and can be audited after publication.

## Goals / Non-Goals

**Goals:**

- Require `RELEASE_NOTES_FILE=<path>` for real release tag creation from both explicit-version and bump-based make targets.
- Ensure the markdown content from that file is the content passed to GoReleaser for the GitHub Releases page.
- Bind the notes content to the signed tag so CI does not depend on an uncommitted local file.
- Keep dry runs useful by validating the release notes file and showing what would be published without mutating remote state.
- Fail early with actionable errors when the release notes file is missing, empty, not a regular file, or not a markdown file.

**Non-Goals:**

- Generating release notes automatically.
- Replacing GoReleaser as the publisher of release assets.
- Changing installer scripts, package metadata, benchmark publication, or version-docs behavior except where release workflow arguments must be threaded through.
- Adding a persistent release-notes directory unless a later change wants committed release-note history.

## Decisions

1. Use `RELEASE_NOTES_FILE` as the make-level interface.

   Alternatives considered: `NOTES`, `RELEASE_NOTES`, positional shell arguments. `RELEASE_NOTES_FILE` is explicit, hard to confuse with inline text, and matches the existing uppercase make variable style.

2. Require the argument for both `release` and `release-bump`.

   Alternatives considered: only requiring it for `release-bump`. That would leave `make release VERSION=...` as an accidental bypass for publishing generated release notes, so both real release entry points should share the same contract.

3. Store the markdown content in the signed annotated tag message.

   Alternatives considered: commit the notes file before release, upload it as a workflow artifact, dispatch CI manually with a large text input, or fetch it from a branch path. The signed tag message is already the object that triggers the release, is available to CI after checkout with full history, and ties the release notes to the exact release tag without adding repository churn.

4. Extract the tag annotation in CI into a temporary markdown file and pass it to GoReleaser with `--release-notes=<file>`.

   GoReleaser supports a release-notes file via the `--release-notes` flag. Using a temporary file keeps the workflow shell-safe for multiline markdown and avoids placing large markdown content directly in YAML inputs.

5. Preserve the current release header/footer only if they still compose with custom release notes.

   If GoReleaser applies configured `release.header` and `release.footer` around the supplied release notes, keep them. If they do not compose as intended, make the workflow/config explicit so the GitHub Release page still contains the provided notes file as the authoritative body and retains the performance-history link if required.

6. Treat manual `workflow_dispatch` publishing separately from local make-based releases.

   Manual dispatch does not have access to a local `RELEASE_NOTES_FILE`. It should either be limited to dry-run use or gain its own required release-notes input/path before it can publish. The implementation should not allow manual publishing to silently fall back to generated notes.

## Risks / Trade-offs

- Signed tag annotations can contain multiline markdown, but tooling that displays tag messages may now show full release notes instead of a short `Release vX.Y.Z` summary. Mitigation: prefix the annotation with a short title if needed, and ensure CI extracts the intended markdown content consistently.
- Existing release tags used short tag messages. Mitigation: this is forward-only release process behavior; old tags remain valid.
- CI extraction could accidentally include the tag signature block or omit part of the notes. Mitigation: use git plumbing that returns the tag subject/body content without the signature, and cover extraction with a shell test or workflow-adjacent script test.
- GoReleaser custom notes behavior may interact with configured header/footer. Mitigation: verify with a dry-run or focused workflow test and adjust `.goreleaser.yaml` only as needed to keep the provided markdown as the GitHub Release body.
- Manual dispatch may become less convenient. Mitigation: make the constraint explicit and prefer make-driven signed releases for real publishing; add a manual notes input only if maintainers need direct workflow publishing.
