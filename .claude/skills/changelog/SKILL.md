---
name: changelog
description: Generate release notes from conventional commits since the last tag. Groups changes by type (feat, fix, refactor, etc.) and drafts markdown release notes.
user-invocable: true
disable-model-invocation: true
---

# Changelog Generator

Generate release notes from conventional commit history between the last tag and HEAD.

## When to Use

Invoke this skill (`/changelog`) when:
- Preparing a new release and need draft release notes
- Reviewing what changed since the last release
- Want a summary of all commits on a feature branch

## Workflow

### Step 1: Identify the Version Range

```bash
# Get the latest tag
LATEST_TAG=$(git describe --tags --abbrev=0 2>/dev/null)

# If no tag exists, use the initial commit
if [ -z "$LATEST_TAG" ]; then
  RANGE="HEAD"
else
  RANGE="${LATEST_TAG}..HEAD"
fi
```

Report the range being analyzed (e.g., "Changes since v0.1.0-alpha.3").

### Step 2: Collect Commits

```bash
git log --oneline --no-merges ${RANGE}
```

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
git diff --stat ${RANGE} | tail -1

# Contributors
git log --format='%aN' ${RANGE} | sort -u
```

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
**Full diff**: [`previous-tag...HEAD`](repo-url/compare/previous-tag...HEAD)
**Contributors**: @name1, @name2
**Stats**: X files changed, Y insertions(+), Z deletions(-)
```

### Step 7: Offer Next Steps

After generating the notes, suggest:
1. Copy to clipboard for pasting into a GitHub Release
2. Save to a `CHANGELOG.md` file (if one exists, prepend)
3. Use with `make release VERSION=vX.Y.Z` to tag and release
