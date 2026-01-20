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

`website/src/components/Snippet/snippets.ts`

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
- When behavior changes, create new snippet IDs for the upcoming version and keep existing snippet IDs unchanged to preserve older docs.
- When cutting a release, snapshot docs with `cd website && npx docusaurus docs:version X.Y.Z`, then continue updates in `website/docs/` for the next version.

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
npm run build    # Build all locales - must pass
npm run serve    # Test language switcher at localhost:3000
```

Any documentation change must include a successful `npm run build` run (no errors).

## Common Mistakes

1. **Forgetting to update `sidebars.ts`** - Page exists but doesn't appear in navigation
2. **Forgetting translations** - Build fails for non-English locales
3. **Using inline code blocks instead of Snippets** - Creates duplication across translations
4. **Not testing the build** - Broken links or missing translations only caught at build time

## File Naming Conventions

- Use kebab-case for file names: `interactive-mode.mdx`
- Use `.mdx` extension (not `.md`) to enable React components
- Match the file name in both English and translation directories exactly
