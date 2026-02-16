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
| `pkg/invowkfile/invowkfile_schema.cue` | `website/docs/reference/invowkfile-schema.mdx` + affected docs/snippets |
| `pkg/invowkmod/invowkmod_schema.cue` | `website/docs/modules/` pages |
| `pkg/invowkmod/operations*.go` | `website/docs/modules/` pages (validation, create, packaging, vendoring) |
| `internal/config/config_schema.cue` | `website/docs/reference/config-schema.mdx`, `website/docs/configuration/options.mdx` |
| `internal/runtime/container*.go` | `website/docs/runtime-modes/container.mdx` |
| `cmd/invowk/init.go` | `website/docs/getting-started/quickstart.mdx`, `website/i18n/pt-BR/.../quickstart.mdx`, `website/src/components/Snippet/snippets.ts` (quickstart/* IDs), `website/src/pages/index.tsx` (terminal demo), `README.md` (Quick Start) |
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
|-- getting-started/     # Installation, quickstart, first invowkfile
|-- core-concepts/       # Invowkfile format, commands, implementations
|-- runtime-modes/       # Native, virtual, container execution
|-- dependencies/        # Tools, filepaths, capabilities, custom checks
|-- flags-and-arguments/ # CLI flags and positional arguments
|-- environment/         # Env files, env vars, precedence
|-- advanced/            # Interpreters, workdir, platform-specific
|-- modules/             # Module creation, validation, distribution
|-- tui/                 # TUI components reference
|-- configuration/       # Config file and options
`-- reference/           # CLI, invowkfile schema, config schema
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
- Update English first, then mirror the same `.mdx` path in `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/`. When backporting fixes to versioned snapshots, also update the corresponding versioned i18n path (e.g., `.../version-0.1.0/`).
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

## Version-Scoped Asset Snapshots

Versioned docs resolve snippets and diagrams from **immutable per-version snapshots** created at release time. Updates to `snippets.ts` or live SVGs never affect versioned docs.

### How It Works

1. When `scripts/version-docs.sh` runs, it calls `scripts/snapshot-version-assets.mjs <version>`.
2. The snapshot script scans `versioned_docs/version-{VERSION}/**/*.mdx` for all `<Snippet id="...">` and `<Diagram id="...">` references.
3. Referenced snippet entries are extracted into `Snippet/versions/v{VERSION}.ts`.
4. Referenced SVGs are copied to `static/diagrams/v{VERSION}/` and paths recorded in `Diagram/versions/v{VERSION}.ts`.
5. Barrel files (`versions/index.ts`) are regenerated for both components.
6. `scripts/validate-version-assets.mjs` verifies all references resolve correctly.

### Component Resolution

Both `<Snippet>` and `<Diagram>` use `useActiveDocContext('default')` to detect the doc version:
- **Versioned docs**: Resolve from the per-version snapshot first, fall back to live data.
- **Current/next docs**: Resolve from live `snippets.ts` / `svgPaths` directly.

### Migration Process (for schema/API changes that rename snippets)

1. Create the new snippet with the new ID in `snippets.ts`.
2. Update current + i18n current docs to reference the new ID.
3. **Remove the old snippet entry** from `snippets.ts` — it's already captured in version snapshots.
4. Never touch versioned docs — they resolve old IDs from their immutable snapshots.

### Backport Fixes

To add content to a versioned doc (e.g., a critical correction):
1. Edit the versioned MDX to add the new `<Snippet>` or `<Diagram>` reference.
2. Run `node scripts/snapshot-version-assets.mjs <version> --update` to add the missing entry without overwriting existing ones.

### Generated Files (auto-generated, do not edit manually)

```
website/src/components/Snippet/versions/   # Per-version snippet snapshots + barrel
website/src/components/Diagram/versions/   # Per-version diagram path maps + barrel
website/static/diagrams/v{VERSION}/        # Per-version SVG copies
```

---

## Common Pitfalls

- **Missing i18n** - Website changes require updates to both `docs/` and `i18n/pt-BR/`.
- **Stale snippets and i18n content** - When fixing factual errors in `website/docs/` (e.g., wrong version numbers, incorrect claims), also sweep `website/src/components/Snippet/snippets.ts` for matching stale values in code examples and `website/i18n/pt-BR/.../current/` for the same stale content in translations. When fixing versioned docs (e.g., `versioned_docs/version-0.1.0/`), also sweep the versioned i18n counterpart (`website/i18n/pt-BR/.../version-0.1.0/`). Snippet code blocks and i18n mirrors are easy to miss because the Documentation Sync Map covers code→doc direction, not doc-content→snippet/i18n direction.
- **Container image policy** — ALL container examples in docs must use `debian:stable-slim` as the base image. No `ubuntu:*`, no `debian:bookworm`. Language-specific images (`golang:1.26`, `python:3-slim`) are allowed for language-specific demos only. See `.claude/rules/version-pinning.md` Container Images.
- **Outdated documentation** - Check the Documentation Sync Map when modifying schemas or CLI.
- **Versioning chicken-and-egg** - `docusaurus.config.ts` `lastVersion` must reference a version that already exists in `versions.json`. If `lastVersion` is set to a version before `docs:version` creates it, Docusaurus validation fails on initialization. Fix: temporarily set `lastVersion` to an existing version, run `version-docs.sh`, which will restore `lastVersion` to the new version in step 4.
- **Doc-then-version ordering** - Always fix documentation issues in `website/docs/` BEFORE running `version-docs.sh`, since the script snapshots the current docs into `versioned_docs/`. Versioning first means the snapshot preserves bugs.
