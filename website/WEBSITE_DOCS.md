# Website Documentation Guidelines

This document contains critical information for maintaining the Invowk documentation website.

## Adding New Documentation Pages

When adding a new documentation page, you must complete **all** of these steps:

### 1. Create the MDX File

Create the page in `website/docs/<section>/<page-name>.mdx`:

```mdx
---
sidebar_position: 1
---

import Snippet from '@site/src/components/Snippet';

# Page Title

Content here...
```

### 2. Create the Translation

Create the same file structure in all locale directories:

- `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/<section>/<page-name>.mdx`

The translation file must have identical structure but translated prose.

### 3. Add to Sidebar Configuration (CRITICAL!)

**The page will NOT appear in the sidebar until you add it to `website/sidebars.ts`.**

The `sidebar_position` frontmatter only controls ordering within a category - it does NOT add the page to the sidebar.

```typescript
// website/sidebars.ts
{
  type: 'category',
  label: 'Section Name',
  items: [
    'section/existing-page',
    'section/your-new-page',  // <-- Add your page here
  ],
},
```

### 4. Add Code Snippets (if needed)

All code blocks should use the reusable `<Snippet>` component. Add snippets to:

`website/src/components/Snippet/data/*.ts`

```typescript
'section/snippet-name': {
  language: 'bash',
  code: `your code here`,
},
```

Then use in MDX:

```mdx
<Snippet id="section/snippet-name" />
```

## Sidebar Structure

The sidebar is manually configured in `website/sidebars.ts`. Current sections:

| Section | Path |
|---------|------|
| Getting Started | `getting-started/` |
| Core Concepts | `core-concepts/` |
| Runtime Modes | `runtime-modes/` |
| Dependencies | `dependencies/` |
| Flags and Arguments | `flags-and-arguments/` |
| Environment Configuration | `environment/` |
| Advanced Features | `advanced/` |
| Modules | `modules/` |
| TUI Components | `tui/` |
| Configuration | `configuration/` |
| Reference | `reference/` |

## Versioning Policy

- Treat `website/docs/` as the upcoming (unreleased) version. Only update it for changes targeting the next release.
- Never edit `website/versioned_docs/version-*/` or `website/versioned_sidebars/` except to fix a bug, mismatch, or critical clarification for an already-released version.
- When you need a backport fix, update the specific `version-*` doc and the matching translation under `website/i18n/pt-BR/docusaurus-plugin-content-docs/version-*/`.
- When behavior changes, create new snippet IDs for the upcoming version. Old snippet IDs can be safely removed from `Snippet/data/*.ts` — versioned docs resolve from immutable per-version snapshots.
- Legacy parity gaps must be recorded in `website/docs-parity-exceptions.json`. If you add new exceptions, include `docs-parity-exception-justification: <reason>` in the PR body.

### Automated Versioning (on Release)

Documentation versioning runs **automatically** on every GitHub Release (stable and pre-release) via the `version-docs.yml` workflow. No manual steps are needed.

**How it works:**

1. A release is published (e.g., `v0.1.0-alpha.1`) via tag push or workflow dispatch.
2. GoReleaser builds and publishes the release artifacts (`release.yml`).
3. The `version-docs.yml` workflow triggers on the `release: published` event.
4. The workflow runs `scripts/version-docs.sh` which:
   - Snapshots `website/docs/` into `website/versioned_docs/version-<VERSION>/`
   - Snapshots version assets (snippets + diagrams) into immutable per-version files
   - Copies i18n translations for all locales
   - Fails if locale source docs/labels are missing (unless `ALLOW_MISSING_LOCALES=1` is explicitly set)
   - Validates docs parity (`current` + all versioned docs) for every locale
   - Updates `docusaurus.config.ts` with the correct `lastVersion` and pre-release banners
   - Validates all snippet/diagram references resolve correctly
5. The versioning commit is pushed to `main`, which triggers `deploy-website.yml` to redeploy the site.

**Versioning rules:**

- All versions (stable and pre-release) are kept indefinitely.
- The default landing version (`/docs/`) is the latest **stable** release.
- If no stable release exists yet (alpha phase), the latest **pre-release** is the default.
- Pre-release versions show an "unreleased" banner in the docs, except for the version serving as `lastVersion` (to avoid a self-referential banner).
- `website/docs/` always represents the unreleased "Next" development docs at `/docs/next/`.

**Manual retry:**

If the workflow fails, re-run it via **Actions > Version Docs > Run workflow** with the release tag (e.g., `v0.1.0-alpha.1`). The script is idempotent — it skips versions that already exist in `versions.json`.

**Local testing:**

```bash
make version-docs VERSION=0.0.0-test   # Create a test version
cd website && npm run build             # Verify the build passes
git checkout -- website/                # Revert the test
```

**GitHub App secrets required:**

The workflow uses a GitHub App token (not `GITHUB_TOKEN`) so the versioning commit triggers the website deployment workflow. Required repository secrets:
- `DOCS_APP_ID` — The GitHub App's numeric ID.
- `DOCS_APP_PRIVATE_KEY` — The App's PEM private key.

## Testing Changes

### Single Locale (Fast)

```bash
cd website
npm start                    # English only
npm start -- --locale pt-BR  # Portuguese only
```

### All Locales (Required Before Commit)

```bash
cd website
npm run docs:parity  # Enforce EN <-> locale docs/snippet/diagram parity
npm run build    # Build all locales - must pass
npm run serve    # Test language switcher at localhost:3000
```

Any documentation change must include successful `npm run docs:parity` and `npm run build` runs (no errors).

## Common Mistakes

1. **Forgetting to update `sidebars.ts`** - Page exists but doesn't appear in navigation
2. **Forgetting translations** - Docs parity validation fails in CI/release pipelines
3. **Using inline code blocks instead of Snippets** - Creates duplication across translations
4. **Not running parity + build checks** - Drift and broken links are only caught in CI

## File Naming Conventions

- Use kebab-case for file names: `interactive-mode.mdx`
- Use `.mdx` extension (not `.md`) to enable React components
- Match the file name in both English and translation directories exactly
