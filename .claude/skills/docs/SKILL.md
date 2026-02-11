---
name: docs
description: Documentation workflow for website/ directory, Docusaurus, MDX snippets, i18n localization. Use when editing docs/, creating documentation pages, or updating WEBSITE_DOCS.md.
disable-model-invocation: false
---

# Docs and Website

This skill covers updating documentation and the Docusaurus website for Invowk.

Use this skill when working on:
- `website/` - Docusaurus documentation site
- `website/docs/` - Documentation pages
- `website/i18n/` - Internationalization
- Schema changes that require documentation updates

---

## Required Workflow

- Read `website/WEBSITE_DOCS.md` before any website edits.
- Use MDX + `<Snippet>` for all code/CLI/CUE blocks.
- Define snippets in `website/src/components/Snippet/snippets.ts` and reuse IDs across locales.
- Escape `${...}` inside snippets as `\${...}`.

---

## Documentation Sync Map

| Change | Update |
| --- | --- |
| `pkg/invkfile/invkfile_schema.cue` | `website/docs/reference/invkfile-schema.mdx` + affected docs/snippets |
| `pkg/invkmod/invkmod_schema.cue` | `website/docs/modules/` pages |
| `pkg/invkmod/operations*.go` | `website/docs/modules/` pages (validation, create, packaging, vendoring) |
| `internal/config/config_schema.cue` | `website/docs/reference/config-schema.mdx`, `website/docs/configuration/options.mdx` |
| `internal/runtime/container*.go` | `website/docs/runtime-modes/container.mdx` |
| `cmd/invowk/*.go` | `website/docs/reference/cli.mdx` + relevant feature docs |
| `cmd/invowk/module*.go` | `website/docs/modules/` pages + `website/docs/reference/cli.mdx` |
| `cmd/invowk/cmd_validate*.go` | `website/docs/dependencies/` pages |
| `cmd/invowk/tui_*.go` | `website/docs/tui/` pages + snippets |
| New features | Add/update docs under `website/docs/` and snippets as needed |

---

## Diagram Sync Map

When changes affect architectural behavior, evaluate and update these diagrams:

| Change | Diagrams to Evaluate |
| --- | --- |
| `internal/discovery/` changes | flowchart-discovery.md, sequence-execution.md |
| `internal/runtime/` changes | flowchart-runtime-selection.md, sequence-execution.md |
| `cmd/invowk/` command changes | sequence-execution.md |
| New package or component | c4-container.md |
| External integration changes | c4-context.md |
| Server (SSH/TUI) changes | sequence-execution.md (container/virtual variants) |

**Workflow**: Edit `.d2` source → `d2 validate` → `make render-diagrams` → commit both source and SVG.

---

## Documentation Structure

```
website/docs/
|-- getting-started/     # Installation, quickstart, first invkfile
|-- core-concepts/       # Invkfile format, commands, implementations
|-- runtime-modes/       # Native, virtual, container execution
|-- dependencies/        # Tools, filepaths, capabilities, custom checks
|-- flags-and-arguments/ # CLI flags and positional arguments
|-- environment/         # Env files, env vars, precedence
|-- advanced/            # Interpreters, workdir, platform-specific
|-- modules/             # Module creation, validation, distribution
|-- tui/                 # TUI components reference
|-- configuration/       # Config file and options
`-- reference/           # CLI, invkfile schema, config schema
```

---

## Documentation Style Guide

- Use a friendly, approachable tone with occasional humor.
- Follow progressive disclosure: start simple, add complexity gradually.
- Include practical examples for each feature.
- Use admonitions for important callouts.
- Keep code examples concise and focused.

---

## Docs + i18n Checklist

- Always use `.mdx` (not `.md`) in `website/docs/` and translations.
- Treat `website/docs/` as the upcoming version; only touch versioned docs for backport fixes (see `website/WEBSITE_DOCS.md`).
- Update English first, then mirror the same `.mdx` path in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`.
- Keep translations prose-only and reuse identical snippet IDs.
- Regenerate translation JSON when UI strings change: `cd website && npx docusaurus write-translations --locale pt-BR`.

---

## Documentation Testing

```bash
# Single locale development
cd website && npm start

# Brazilian Portuguese locale
cd website && npm start -- --locale pt-BR

# Full build (tests all locales)
cd website && npm run build

# Serve built site (for locale switching)
cd website && npm run serve
```

---

## Frozen Snippets Convention

Docusaurus versioned docs are independent MDX snapshots that **share React components** via `@site/`. There is no built-in version-specific component override. When a snippet ID is renamed or removed during a schema/API migration, old versioned docs still reference the old ID through the shared `<Snippet>` component.

### Rules

- Old snippet entries are preserved with their **exact original content** -- no deprecation notes, no modifications.
- Old entries live in a clearly marked `// FROZEN SNIPPETS` section at the end of `snippets.ts`.
- Each frozen entry gets a brief maintainer comment noting when it was frozen and what replaced it (e.g., `// Frozen in v0.1.0-alpha.3. Current: 'new-id'`). This is for maintainer context only and is not rendered to users.
- Current docs use new IDs; versioned docs keep referencing old IDs naturally.

### Migration Process (for any schema/API change that renames snippets)

1. Create the new snippet with the new ID.
2. Move the old snippet entry to the Frozen section (preserve exact original content).
3. Add a maintainer comment: `// Frozen in vX.Y.Z. Current: 'new-id'` (or `(command removed)` / `(unchanged content)` as appropriate).
4. Update current + i18n current docs to reference new IDs.
5. Never touch versioned docs -- they reference old IDs and the frozen entries serve them.

### Scaling

When the frozen section grows beyond ~30 entries, extract it to a separate `frozenSnippets.ts` file and merge it into the Snippet component's lookup map.

---

## Common Pitfalls

- **Missing i18n** - Website changes require updates to both `docs/` and `i18n/pt-BR/`.
- **Outdated documentation** - Check the Documentation Sync Map when modifying schemas or CLI.
