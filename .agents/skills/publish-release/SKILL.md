---
name: publish-release
description: Publish new Invowk releases through GitHub Releases and every supported release channel. Use when preparing, dry-running, tagging, publishing, monitoring, troubleshooting, rolling back, or repairing a release; drafting release notes; validating Homebrew, WinGet, install-script, Go install, docs-versioning, Cosign, Bencher, or GoReleaser release behavior; or investigating release workflow failures.
---

# Publish Release

## Overview

Use this skill to ship Invowk releases deliberately: derive user-facing notes from both commits and semantic diffs, bind reviewed notes into the signed tag, publish with the repository release pipeline, verify every channel, and handle partial failures without making the situation worse.

## Source Map

- `Makefile`: maintainer entry points `make release`, `make release-bump`, and `make version-docs`.
- `scripts/release.sh`: semver, prerelease, promotion, clean-tree, tag, and push helper.
- `scripts/release-notes.sh`: release-note file validation and annotated-tag extraction.
- `.github/workflows/release.yml`: cross-platform tests, tag creation for manual dispatch, Bencher release tracking, GoReleaser, Homebrew, WinGet, and dry runs.
- `.goreleaser.yaml`: artifacts, checksums, Cosign bundle, GitHub Release settings, Homebrew cask, and WinGet config.
- `scripts/enhance-winget-manifest.sh` and `scripts/winget/`: WinGet post-processing and manifest expectations.
- `.github/workflows/version-docs.yml`: release-triggered documentation versioning.
- `README.md` and `website/docs/getting-started/installation.mdx`: supported installation channels users see.

For details, load only what the task needs:

- `references/channels.md`: supported release channels and verification.
- `references/release-notes.md`: release-note structure, semantic analysis, tone, and poem checks.
- `references/recovery.md`: failure triage, rollback, and repair playbooks.

## Mandatory Flow

1. Confirm the intended release target: exact tag, bump type, prerelease label, or promotion. If the target is ambiguous, ask before creating or pushing a tag.
2. Inspect current state: `git status --short`, current branch, `git fetch --tags origin`, latest semantic tag, and whether `origin/main` matches local `main`.
3. Create or review the release notes before any real tag. Release notes are mandatory for both `make release` and `make release-bump`.
4. Analyze both commit history and semantic diff since the previous release. Do not rely only on conventional commits or GitHub-native notes.
5. Use subagents when useful and available:
   - Ask one read-only subagent to summarize user-visible behavior from the code diff.
   - Ask one read-only subagent to audit release channels and docs/install impact.
   - Ask one read-only subagent to review release notes for clarity, required sections, manual actions, warnings, deprecations, and poem reuse.
   Keep subagents read-only unless the user explicitly asks for delegated edits, and require plan approval before any teammate/worker edits.
6. Validate locally before publishing: run the narrow release-script checks first, then broader gates as risk warrants.
7. Prefer dry runs for risky changes: `make release-bump TYPE=<type> RELEASE_NOTES_FILE=<file> DRY_RUN=1`.
8. Publish by creating a signed annotated tag with reviewed notes: `make release VERSION=<tag> RELEASE_NOTES_FILE=<file>` or `make release-bump TYPE=<type> RELEASE_NOTES_FILE=<file>`.
9. Monitor GitHub Actions until the release workflow reaches a terminal state. Do not stop at "tag pushed".
10. Verify GitHub Release, artifacts, checksums, signature bundle, supported package channels, docs versioning, and the WinGet PR/enhancement path.
11. If anything fails after a tag exists, load `references/recovery.md` before deleting, moving, or reusing tags.

## Release Notes

Always load `references/release-notes.md` before drafting or approving notes.

Required headers, in this order:

1. `## Poetic Opening`
2. `## What's Changed`
3. `## Manual Actions Needed`
4. `## Warnings and Deprecations`
5. `## Bug Fixes`

Write in a warm, collaborative tone. Explain changes in clear user language, name risks plainly, and include "Nothing required" or "No known deprecations" when a section has no items.

The poetic opening must include a full stanza/verse from Rimbaud, Oscar Wilde, Baudelaire, Edgar Allan Poe, Shakespeare, or Florbela Espanca. Do not use a stray line, partial stanza, or excerpt that cuts through a stanza. For non-English originals, include the original-language stanza and an English rendering. Use public-domain text or an agent-authored working translation. Never repeat a stanza across releases; run:

```bash
python3 .agents/skills/publish-release/scripts/check-poetic-opening.py release-notes.md
```

Then manually double-check prior GitHub Releases or tag bodies for the poem title, author, and first line before publishing.

## Publish Commands

Use a reviewed release notes file:

```bash
make release VERSION=v1.2.3 RELEASE_NOTES_FILE=release-notes.md
make release-bump TYPE=minor RELEASE_NOTES_FILE=release-notes.md
make release-bump TYPE=minor RELEASE_NOTES_FILE=release-notes.md PRERELEASE=alpha
make release-bump TYPE=minor RELEASE_NOTES_FILE=release-notes.md PROMOTE=1
make release-bump TYPE=patch RELEASE_NOTES_FILE=release-notes.md DRY_RUN=1
```

Manual workflow dispatch is available from GitHub Actions and requires release-notes markdown input. Prefer the Makefile path when publishing from a maintainer workstation because it creates a signed annotated tag.

## Verification

For local edits to release machinery or this skill:

```bash
bash scripts/test_release.sh
sh scripts/test_enhance_winget_manifest.sh
make check-agent-docs
```

For an actual release, also verify:

- `gh run list --workflow Release --limit 5` and `gh run view <run-id> --log`.
- `gh release view <tag> --json tagName,isPrerelease,assets,body` and compare
  its tag with `gh release view --json tagName --jq .tagName` when latest-release
  status matters. `isLatest` is not a supported `gh release view` JSON field.
- `cosign verify-blob --bundle checksums.txt.sigstore.json --certificate-identity-regexp='https://github\.com/invowk/invowk/.*' --certificate-oidc-issuer='https://token.actions.githubusercontent.com' checksums.txt`.
- Stable releases update or create the Homebrew cask and WinGet PR; prereleases intentionally skip those uploads.
- `version-docs.yml` runs after GitHub Release publication and commits docs with the GitHub App token.

## Failure Discipline

- Before publication, prefer aborting, fixing, and rerunning.
- After a tag is pushed but before public artifacts exist, deleting the tag can be acceptable if the team agrees.
- After public artifacts or package-channel updates exist, do not move or reuse the tag. Repair forward with a new patch/prerelease unless the user explicitly approves a coordinated yank.
- Close or repair package-channel PRs rather than leaving misleading installation paths open.
- Preserve user trust in the release notes: update the GitHub Release body with known issues and remediation if a release is partially broken.
