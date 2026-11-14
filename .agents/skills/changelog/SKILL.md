---
name: changelog
description: Draft a commit-derived changelog or release notes evidence from conventional commits since the latest semantic version tag. Use for pre-release summaries, reviewing changes since the last release, or collecting commit evidence for publish-release notes.
---

# Changelog Generator

Generate a commit-derived draft changelog from conventional commit history between the latest `v*` semver tag and HEAD.

This output is commit evidence that feeds the five-section release notes of the publish-release skill (`.agents/skills/publish-release/references/release-notes.md`); it is never the release body. Real releases publish the reviewed notes bound into the tag: `release.yml` runs GoReleaser with `--release-notes`, wrapped in the `.goreleaser.yaml` header and footer, so `changelog.use: github-native` and the `.github/release.yml` categories are not used.

## Workflow

### Step 1: Identify the Version Range

Collect the range and raw evidence with the bundled helper:

```bash
python3 .agents/skills/changelog/scripts/collect-changelog.py > /tmp/changelog-input.json
jq '{latest_tag, log_range, diff_range, compare_url, shortstat}' /tmp/changelog-input.json
```

The helper deliberately uses the empty Git tree for a first release. In that
case `latest_tag` and `compare_url` are `null`, `log_range` is `HEAD`, and
`diff_range` covers the entire initial history. Do not fabricate a GitHub
compare URL with an empty previous tag.

Report the range being analyzed (e.g., "Changes since v0.1.0-alpha.3").

### Step 2: Collect Commits

Read the helper's `commits` array. It retains each full hash, subject, body,
and author, so body-only `BREAKING CHANGE:` trailers are not missed.

### Step 3: Group by Conventional Commit Type

Parse each commit's subject line and group into categories:

| Prefix | Section Header |
|--------|---------------|
| `feat` | Features |
| `fix` | Bug Fixes |
| `refactor` | Refactoring |
| `perf` | Performance |
| `docs` | Documentation |
| `test` | Tests |
| `ci` | CI/CD |
| `chore` | Chores |
| `build` | Build |
| Other | Other Changes |

Within each group, include the scope in parentheses if present:
- `feat(container): add retry logic` → **container**: add retry logic

### Step 4: Identify Breaking Changes

Look for commits with:
- `BREAKING CHANGE:` in the commit body
- `!` after the type/scope (e.g., `feat!:` or `feat(config)!:`)

List these separately under a **Breaking Changes** section at the top.

### Step 5: Generate Stats

Use `shortstat`, `contributors`, and `compare_url` from the helper output.
Contributor names from git are not GitHub handles unless separately resolved
through GitHub metadata. For a first release, label the link as unavailable
instead of emitting a broken comparison.

### Step 6: Output Draft Changelog

Format as markdown. This is a draft changelog, not the release-notes structure
(that lives in publish-release `references/release-notes.md`):

```markdown
## [version] - YYYY-MM-DD

### Breaking Changes
- ...

### Features
- **scope**: description (commit-hash)

### Bug Fixes
- **scope**: description (commit-hash)

### Refactoring
- ...

### Documentation
- ...

### Other Changes
- ...

---
**Full diff**: [`previous-tag...HEAD`](compare_url) <!-- omit for a first release -->
**Contributors**: Name 1, Name 2
**Stats**: X files changed, Y insertions(+), Z deletions(-)
```

### Step 7: Offer Next Steps

After generating the draft, suggest:
1. Feed it into the publish-release release notes (five sections, semantic diff review, poetic opening)
2. Save to a `CHANGELOG.md` file only if the repo already maintains one
3. Tag with `make release VERSION=vX.Y.Z RELEASE_NOTES_FILE=<notes.md>` through the publish-release skill (`RELEASE_NOTES_FILE` is required)
