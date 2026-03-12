# Consolidated Documentation Sync Map

This is the canonical superset sync map for documentation review. It merges the maps
from `.agents/skills/docs/SKILL.md` (13 rows) and `.agents/agents/doc-updater.md` (10 rows),
adding the 3 rows missing from the doc-updater and consolidating diagram triggers.

## Code → Website Docs Map

| Code Change | Website Docs to Update |
|---|---|
| `pkg/invowkfile/invowkfile_schema.cue` | `website/docs/reference/invowkfile-schema.mdx` + affected docs/snippets |
| `pkg/invowkmod/invowkmod_schema.cue` | `website/docs/modules/` pages, module reference docs |
| `pkg/invowkmod/operations*.go` | `website/docs/modules/` pages (validation, create, packaging, vendoring) |
| `internal/config/config_schema.cue` | `website/docs/reference/config-schema.mdx`, `website/docs/configuration/options.mdx` |
| `internal/config/types.go` (`DefaultConfig()`) | `website/docs/reference/config-schema.mdx` (default values), `website/docs/configuration/options.mdx` (default values), pt-BR mirrors |
| `internal/runtime/container*.go` | `website/docs/runtime-modes/container.mdx` |
| `cmd/invowk/init.go` | `website/docs/getting-started/quickstart.mdx`, i18n mirror, `getting-started.ts` snippets, `index.tsx` terminal demo, `README.md` Quick Start |
| `cmd/invowk/*.go` (general CLI) | `website/docs/reference/cli.mdx` + relevant feature docs |
| `cmd/invowk/module*.go` | `website/docs/modules/` pages + `website/docs/reference/cli.mdx` |
| `cmd/invowk/validate.go` | `website/docs/reference/cli.mdx` (validate command) |
| `cmd/invowk/cmd_validate*.go` | `website/docs/dependencies/` pages |
| `cmd/invowk/tui_*.go` | `website/docs/tui/` pages + snippets |
| New features | Add/update docs under `website/docs/` and snippets as needed |

## Code → Diagram Map

When code changes affect architectural behavior, evaluate and update these diagrams:

| Code Change | Diagrams to Evaluate |
|---|---|
| `internal/discovery/` changes | `flowchart-discovery.md`, `sequence-execution.md` |
| `internal/runtime/` changes | `flowchart-runtime-selection.md`, `sequence-execution.md` |
| `cmd/invowk/` command changes | `sequence-execution.md` |
| `cmd/invowk/app.go` (discovery cache) | `flowcharts/discovery-cache.d2` |
| New package or component | `c4-container.md` |
| External integration changes | `c4-context.md` |
| Server (SSH/TUI) changes | `sequence-execution.md` (container/virtual variants) |

Diagram sources live in `docs/diagrams/` (23 `.d2` files). Rendered SVGs in `docs/diagrams/rendered/`.
Architecture prose docs in `docs/architecture/` reference these SVGs. Website pages in
`website/docs/architecture/` serve them via Docusaurus static directories.

## Drift-Prone Areas (ranked by historical frequency)

1. **CUE snippet schema drift** — Missing `platforms` field in implementation blocks is the #1 pattern. See `references/cue-drift-patterns.md`.
2. **Dual-prefix config snippets** — `config/*` and `reference/config/*` in `Snippet/data/config.ts` both need updates when config changes.
3. **DefaultConfig() drift** — `internal/config/types.go` is the source of truth for default values. Docs at `config-schema.mdx` and `options.mdx` can lag behind.
4. **Container image policy violations** — All examples must use `debian:stable-slim`. Language-specific images allowed only for language-specific demos.
5. **pt-BR diagram ID divergence** — Diagram IDs in i18n MDX files can silently diverge from English. `npm run docs:parity` catches this.
6. **Stale i18n content** — When fixing factual errors in English docs, the same stale content often persists in pt-BR translations and snippet data files.
