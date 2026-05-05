---
name: docs
description: Documentation editing workflow for website/ directory, Docusaurus, MDX snippets, i18n localization, and versioning. Use when editing docs/, creating pages, updating snippets, or running version-docs. For reviewing documentation accuracy, use /review-docs instead.
disable-model-invocation: false
---

# Docs and Website Editing

This skill covers creating and editing documentation for the Invowk Docusaurus website.
For reviewing documentation accuracy against the codebase, use the `/review-docs` skill instead.

## Before You Edit

- Read `website/WEBSITE_DOCS.md` before any website edits.
- Check `.agents/skills/review-docs/references/consolidated-sync-map.md` for code → docs mappings
  (the canonical sync map covering both website docs and diagrams).

---

## Syncing Docs After Code Changes

When code changes require documentation updates:

1. **Identify affected docs**: Cross-reference the consolidated sync map (code → website docs + diagrams)
2. **Read the code change**: Understand what behavior changed
3. **Update English docs first**: Modify the relevant `.mdx` pages in `website/docs/`
4. **Update snippets**: If code examples changed, update `Snippet/data/*.ts`. Old entries
   superseded by new IDs can be removed — versioned docs use immutable snapshots.
5. **Update i18n**: Mirror changes to `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`
6. **Update diagrams**: If architecture changed, use the Diagram Editing workflow below
7. **Verify parity**: `cd website && npm run docs:parity`
8. **Verify build**: `cd website && npm run build`

---

## Snippet System

All code blocks use the `<Snippet>` component — never inline fenced code blocks for reusable content.

- Define snippets in `website/src/components/Snippet/data/*.ts` (12 section files)
- Reference by ID: `<Snippet id="section/name" />`
- Reuse identical snippet IDs across EN and pt-BR translations
- Escape `${...}` as `\${...}` inside snippets
- When adding CUE snippets with implementation blocks, always include `platforms:` —
  see `.agents/skills/review-docs/references/cue-drift-patterns.md` for correct values per runtime

---

## New Page Checklist

1. Create `.mdx` file in `website/docs/<section>/`
2. Create pt-BR translation at `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/<section>/`
3. **Add to `website/sidebars.ts`** — `sidebar_position` only controls ordering, NOT inclusion
4. Add `<Snippet>` entries to `Snippet/data/*.ts` if code examples are needed
5. Run `cd website && npm run docs:parity` to verify parity

---

## i18n Checklist

1. Use `.mdx` extension (not `.md`) in `website/docs/` and translations
2. Treat `website/docs/` as the upcoming version; only touch versioned docs for backport fixes
3. Update English first, then mirror to `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`
4. When backporting, also update the corresponding versioned i18n path (e.g., `.../version-0.1.0/`)
5. Keep translations prose-only — reuse identical snippet IDs
6. Regenerate translation JSON when UI strings change: `cd website && npx docusaurus write-translations --locale pt-BR`

---

## Style Guide

- Friendly, approachable tone with occasional humor
- Progressive disclosure: start simple, add complexity gradually
- Practical examples for each feature
- Use admonitions (`:::info`, `:::warning`, `:::danger`) for callouts

---

## Documentation Testing

```bash
cd website && npm start                    # English dev server
cd website && npm start -- --locale pt-BR  # pt-BR dev server
cd website && npm run docs:parity          # Enforce EN ↔ pt-BR parity
cd website && npm run build                # Full build (all locales, catches broken links)
cd website && npm run serve                # Test language switcher
```

---

## Version-Scoped Asset Snapshots

Versioned docs resolve snippets and diagrams from **immutable per-version snapshots**.
Updates to `Snippet/data/*.ts` or live SVGs never affect versioned docs.

### Snippet/Diagram Migration (schema changes that rename IDs)

1. Create new snippet ID in `Snippet/data/*.ts`
2. Update current + i18n docs to reference the new ID
3. Remove old ID from data file — versioned docs resolve from immutable snapshots
4. Never touch versioned docs for ID renames

### Backport Fixes

1. Edit the versioned MDX to add the new reference
2. Run `node scripts/snapshot-version-assets.mjs <version> --update`

### Generated Files (do not edit manually)

```
website/src/components/Snippet/versions/   # Per-version snippet snapshots + barrel
website/src/components/Diagram/versions/   # Per-version diagram path maps + barrel
website/static/diagrams/v{VERSION}/        # Per-version SVG copies
```

---

## Versioning Pitfalls

- **Chicken-and-egg**: `docusaurus.config.ts` `lastVersion` must reference a version already in
  `versions.json`. Fix: temporarily use an existing version, run `version-docs.sh`, which restores
  the correct `lastVersion` in step 5.
- **Doc-then-version**: Fix docs in `website/docs/` BEFORE running `version-docs.sh` — the script
  snapshots current state into `versioned_docs/`.
- **New diagrams break old validation**: Adding SVGs to `Diagram/index.tsx` triggers failures for
  old version snapshots. Fix: `node scripts/snapshot-version-assets.mjs <old-version> --update`
  per affected version.
- **Retroactive versioning**: `version-docs.sh` handles out-of-order safely (sorts `versions.json`
  by semver, uses semver comparison for `lastVersion`).

---

## Diagram Editing

Edit `.d2` source → `d2 fmt` → `d2 validate` → `make render-diagrams` → commit both source and SVG.
For D2 syntax, readability rules, and update triggers, use the `/d2-diagrams` skill.

---

## Important Rules

- All `invowk internal *` commands are hidden — do NOT document in website docs
- All container examples must use `debian:stable-slim` (never Alpine, never Windows containers)
