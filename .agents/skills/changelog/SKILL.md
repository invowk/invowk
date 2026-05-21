---
name: changelog
description: Draft commit-derived release notes from conventional commits since the latest semantic version tag. Use for pre-release summaries, reviewing changes since the last release, or preparing text to compare with GitHub-native GoReleaser notes.
---

# Changelog Generator

Generate commit-derived draft release notes from conventional commit history between the latest `v*` semver tag and HEAD.

Invowk releases use GoReleaser with GitHub-native release notes (`.goreleaser.yaml` and `.github/release.yml`). Treat this skill's output as a draft/checklist, not the canonical generated GitHub release notes.

## Workflow

### Step 1: Identify the Version Range

```bash
LATEST_TAG=$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n1)

if [ -z "$LATEST_TAG" ]; then
  RANGE="HEAD"
  DIFF_RANGE="$(git hash-object -t tree /dev/null)..HEAD"
else
  RANGE="${LATEST_TAG}..HEAD"
  DIFF_RANGE="$RANGE"
fi
```

Report the range being analyzed (e.g., "Changes since v0.1.0-alpha.3").

### Step 2: Collect Commits

```bash
git log --no-merges --format='%H%x09%s%x09%b%x1e' ${RANGE}
```

Use the full subject/body output so body-only `BREAKING CHANGE:` trailers are not missed.

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

```bash
# Files changed
git diff --stat ${DIFF_RANGE} | tail -1

# Contributors
git log --format='%aN' ${RANGE} | sort -u

# Compare URL
origin_url=$(git remote get-url origin)
repo_slug=$(printf '%s\n' "$origin_url" |
  sed -E 's#^git@github.com:##; s#^https://github.com/##; s#\.git$##')
compare_url="https://github.com/${repo_slug}/compare/${LATEST_TAG}...HEAD"
```

Contributor names from git are not GitHub handles unless separately resolved through GitHub metadata.

### Step 6: Output Release Notes

Format as markdown:

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
**Full diff**: [`previous-tag...HEAD`](https://github.com/invowk/invowk/compare/previous-tag...HEAD)
**Contributors**: Name 1, Name 2
**Stats**: X files changed, Y insertions(+), Z deletions(-)
```

### Step 7: Offer Next Steps

After generating the notes, suggest:
1. Compare against GitHub-native release notes produced by the release workflow
2. Save to a `CHANGELOG.md` file only if the repo already maintains one
3. Use with `make release VERSION=vX.Y.Z` when preparing the release tag
