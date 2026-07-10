# Consolidated Documentation Sync Map

This is the canonical change-oriented sync map for documentation work (review and editing).
The `/docs` skill and `/review-docs` skill both reference this file.

For audit coverage, `doc-ownership.json` is authoritative because it assigns every current MDX
page one exact semantic owner. This change-oriented map identifies the documentation surfaces to
reevaluate after code changes.

## Code → Website Docs Map

| Code Change | Website Docs to Update |
|---|---|
| `pkg/invowkfile/invowkfile_schema.cue` | `website/docs/reference/invowkfile-schema.mdx` + affected docs/snippets |
| `pkg/invowkfile/command.go`, `internal/discovery/` | `website/docs/core-concepts/` |
| `pkg/invowkfile/flag.go`, `pkg/invowkfile/argument.go`, CLI input binding | `website/docs/flags-and-arguments/` |
| `pkg/invowkfile/env.go`, `internal/runtime/env*.go`, `internal/runtime/dotenv.go` | `website/docs/environment/` |
| `pkg/invowkfile/interpreter_spec.go`, runtime interpreter resolution | `website/docs/advanced/interpreters.mdx` |
| `pkg/invowkfile/workdir.go`, runtime workdir resolution | `website/docs/advanced/workdir.mdx` |
| `pkg/invowkfile/runtime.go`, `pkg/platform/`, runtime selection | `website/docs/advanced/platform-specific.mdx` |
| Interactive runtime/adapters/TUI session changes | `website/docs/advanced/interactive-mode.mdx` |
| `pkg/invowkmod/invowkmod_schema.cue` | `website/docs/modules/` pages, module reference docs |
| `internal/app/moduleops/`, `internal/app/modulesync/`, `pkg/invowkmod/` | `website/docs/modules/` pages (validation, create, packaging, vendoring, dependency sync/tidy) |
| `internal/config/config_schema.cue` | `website/docs/reference/config-schema.mdx`, `website/docs/configuration/options.mdx` |
| `internal/config/types.go` (`DefaultConfig()` derives schema defaults) | `website/docs/reference/config-schema.mdx` (default values), `website/docs/configuration/options.mdx` (default values), pt-BR mirrors |
| `internal/runtime/container*.go`, `internal/container/`, `internal/containerplan/`, `internal/provision/` | `website/docs/runtime-modes/container.mdx` |
| `internal/runtime/native*.go`, `internal/runtime/script_resolver.go` | `website/docs/runtime-modes/native.mdx` |
| `internal/runtime/sh.go`, `internal/runtime/virtual_*.go`, `internal/uroot/` | `website/docs/runtime-modes/virtual.mdx` |
| `internal/runtime/lua*.go`, `internal/runtime/virtual_policy.go` | `website/docs/runtime-modes/virtual-lua.mdx` |
| Runtime registry or execution orchestration | `website/docs/runtime-modes/overview.mdx` |
| `cmd/invowk/init.go` | `website/docs/getting-started/quickstart.mdx`, i18n mirror, `getting-started.ts` snippets, `index.tsx` terminal demo, `README.md` Quick Start |
| `cmd/invowk/*.go` (general CLI) | `website/docs/reference/cli.mdx` + relevant feature docs |
| `cmd/invowk/module*.go` | `website/docs/modules/` pages + `website/docs/reference/cli.mdx` |
| `cmd/invowk/validate.go` | `website/docs/reference/cli.mdx` (validate command) |
| `cmd/invowk/cmd_validate*.go` | `website/docs/dependencies/` pages |
| `cmd/invowk/tui_*.go` | `website/docs/tui/` pages + snippets |
| `cmd/invowk/audit.go`, `internal/audit/`, `internal/auditllm/` | `website/docs/security/audit.mdx`, `website/src/components/Snippet/data/security.ts`, `website/docs/reference/cli.mdx`, `README.md` Security Auditing |
| `cmd/invowk/agent.go`, `internal/agentcmd/`, shared LLM flags/config | `website/docs/advanced/llm-assisted-authoring.mdx`, `website/src/components/Snippet/data/cli.ts`, `website/docs/reference/cli.mdx`, `README.md` LLM-Assisted Agent Authoring |
| `.agents/skills/review-docs/`, `.agents/commands/review-docs.md` | `.agents/skills/docs/SKILL.md`, `AGENTS.md`, review-docs workflow references |
| Benchmark workflows, BMF tooling, Bencher image, or benchmark package | `website/docs/performance/benchmark-history.mdx` |
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
| `internal/container/` component boundary changes | `c4-component-container.md`, `c4/component-container.d2` |
| `internal/runtime/` component boundary changes | `c4-component-runtime.md`, `c4/component-runtime.d2` |

Diagram sources live in the `docs/diagrams/` live inventory (`*.d2`, excluding experiments).
Rendered SVGs live in `docs/diagrams/rendered/`.
Architecture prose docs in `docs/architecture/` reference these SVGs. Website pages in
`website/docs/architecture/` serve them via Docusaurus static directories.

## Drift-Prone Areas (ranked by historical frequency)

1. **CUE snippet schema drift** — Missing `platforms` field in implementation blocks is the #1 pattern. See `references/cue-drift-patterns.md`.
2. **Dual-prefix config snippets** — `config/*` and `reference/config/*` in `Snippet/data/config.ts` both need updates when config changes.
3. **Config default drift** — `internal/config/config_schema.cue` is the source of truth for default values, and `DefaultConfig()` must be CUE-derived. Docs at `config-schema.mdx` and `options.mdx` can lag behind.
4. **Container image policy violations** — All examples must use `debian:stable-slim`. Language-specific images allowed only for language-specific demos.
5. **pt-BR diagram ID divergence** — Diagram IDs in i18n MDX files can silently diverge from English. `npm run docs:parity` catches this.
6. **Stale i18n content** — When fixing factual errors in English docs, the same stale content often persists in pt-BR translations and snippet data files.
7. **Security/audit contract drift** — Audit docs span CLI flags, checker categories, JSON DTOs, LLM provider behavior, and CI examples; review `cmd/invowk/audit.go`, `internal/audit/`, and `security.ts` together.
8. **LLM contract drift** — `invowk audit` and LLM-backed `invowk agent cmd`/`invowk agent mod` create/change share LLM flags but differ in opt-in/default behavior; verify docs against `llm_flags.go`, `llmconfig`, and `internal/agentcmd`.
9. **Agent workflow coverage drift** — New docs sections or snippet data files can outgrow review surfaces; run a live inventory before manual review.
