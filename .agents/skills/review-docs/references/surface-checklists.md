# Surface Verification Checklists

Each surface has a numbered checklist of verification items. Subagents report PASS, FAIL, or N/A
for every item — no item may be skipped. Findings are generated from FAIL items only.

Severity is pre-assigned per item to eliminate subjective classification. The severity levels
(ERROR, WARNING, INFO) follow the definitions in `structured-output-format.md`.

Finding Type is also pre-assigned per item. Use the listed type exactly; do not invent or
reinterpret categories during review.

---

## §S1: README.md

**File scope**: `README.md` (~2870 lines)

**Source of truth mapping**: See `readme-sync-map.md` for exact line numbers and source files.

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S1-C01 | Features list (L5) accurately describes current capabilities | `pkg/`, `internal/`, `cmd/` code | WARNING | source-drift |
| S1-C02 | Installation section (L36) has correct URLs, platform table, Cosign verify command, and install paths | `scripts/install.sh`, `scripts/install.ps1`, `.goreleaser.yml` | ERROR | cli-contract-drift |
| S1-C03 | Quick Start (L151) matches `invowk init` generated invowkfile content and CLI output format | `cmd/invowk/init.go`, run `invowk init --help` | ERROR | cli-contract-drift |
| S1-C04 | Invowkfile Format (L199) field names, types, and constraints match schema | `pkg/invowkfile/invowkfile_schema.cue` | ERROR | schema-drift |
| S1-C05 | Module Metadata (L369) field names and `requires` structure match schema | `pkg/invowkmod/invowkmod_schema.cue` | ERROR | schema-drift |
| S1-C06 | Dependencies (L486) dependency types (`tools`, `filepaths`, `capabilities`, `custom`) and syntax match `#DependencyConfig` | `pkg/invowkfile/invowkfile_schema.cue` | ERROR | schema-drift |
| S1-C07 | Command Flags (L722) flag types, syntax, and validation options match `#FlagConfig` | `pkg/invowkfile/invowkfile_schema.cue` | ERROR | schema-drift |
| S1-C08 | Command Arguments (L986) argument types, positional syntax, and validation match `#ArgumentConfig` | `pkg/invowkfile/invowkfile_schema.cue` | ERROR | schema-drift |
| S1-C09 | Platform Compatibility (L1362) uses "macos" (not "darwin"), struct format matches `#PlatformConfig` | `pkg/invowkfile/invowkfile_schema.cue` | ERROR | schema-drift |
| S1-C10 | Script Sources (L1493) `source_file` syntax and path resolution match `#Implementation` | `pkg/invowkfile/invowkfile_schema.cue` | ERROR | schema-drift |
| S1-C11 | Interpreter Support (L1554) config format matches `#Interpreter` | `pkg/invowkfile/invowkfile_schema.cue` | ERROR | schema-drift |
| S1-C12 | Modules (L1711) module structure, RDNS naming, and file layout match conventions | `pkg/invowkmod/`, module directory conventions | WARNING | source-drift |
| S1-C13 | Module Dependencies (L1977) `requires` syntax uses `git_url` (not `git`), lock file format correct, no `v` prefix in versions | `pkg/invowkmod/invowkmod_schema.cue` `#RequiredModule` | ERROR | schema-drift |
| S1-C14 | Runtime Modes (L2157) runtime descriptions, capabilities, and selection logic match code | `internal/runtime/` | WARNING | source-drift |
| S1-C15 | Configuration (L2301) config fields, default values, and file location match source of truth | `internal/config/config_schema.cue`, `internal/config/types.go` `DefaultConfig()` | ERROR | source-drift |
| S1-C16 | Shell Completion (L2379) supported shells and generation commands match code | `cmd/invowk/completion.go` | WARNING | cli-contract-drift |
| S1-C17 | Command Examples (L2416) are accurate and produce described output | Actual CLI behavior | WARNING | cli-contract-drift |
| S1-C18 | Interactive TUI Components (L2502) component list, flags, and behavior descriptions match code | `cmd/invowk/tui_*.go` | WARNING | source-drift |
| S1-C19 | Project Structure (L2739) directory descriptions match actual layout | Actual directory layout (`ls`) | WARNING | source-drift |
| S1-C20 | Dependencies/Go (L2786) dependency list and versions match | `go.mod` | WARNING | source-drift |
| S1-C21 | Performance and PGO (L2812) description accuracy | `Makefile`, `default.pgo`, `internal/benchmark/` | INFO | source-drift |
| S1-C22 | Local SonarCloud (L2833) command accuracy | `scripts/sonar-local.sh` | INFO | cli-contract-drift |

**Total items**: 22

---

## §S2: Website Docs (next version only)

**File scope**: `website/docs/` (59+ MDX pages). Never review `website/versioned_docs/`.

**Source of truth mapping**: See `consolidated-sync-map.md` for the full code→docs map.

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S2-C01 | Invowkfile schema reference page matches current schema field names, types, and constraints | `pkg/invowkfile/invowkfile_schema.cue` → `website/docs/reference/invowkfile-schema.mdx` | ERROR | schema-drift |
| S2-C02 | Invowkmod schema reference matches current module schema | `pkg/invowkmod/invowkmod_schema.cue` → `website/docs/modules/` pages | ERROR | schema-drift |
| S2-C03 | Module operation pages (validation, create, packaging, vendoring) match Go implementation | `pkg/invowkmod/operations*.go` → `website/docs/modules/` pages | WARNING | source-drift |
| S2-C04 | Config schema reference default values and field names match schema and DefaultConfig() | `internal/config/config_schema.cue` → `website/docs/reference/config-schema.mdx` | ERROR | source-drift |
| S2-C05 | Configuration options page default values match DefaultConfig() | `internal/config/types.go` → `website/docs/configuration/options.mdx` | ERROR | source-drift |
| S2-C06 | Container runtime page matches current container implementation | `internal/runtime/container*.go` → `website/docs/runtime-modes/container.mdx` | WARNING | source-drift |
| S2-C07 | Quickstart page matches `invowk init` output and getting-started snippets | `cmd/invowk/init.go` → `website/docs/getting-started/quickstart.mdx` | ERROR | cli-contract-drift |
| S2-C08 | CLI reference page matches current command structure, flags, and subcommands | `cmd/invowk/*.go` → `website/docs/reference/cli.mdx` | ERROR | cli-contract-drift |
| S2-C09 | Module CLI pages match module command implementation | `cmd/invowk/module*.go` → `website/docs/modules/` + `website/docs/reference/cli.mdx` | WARNING | cli-contract-drift |
| S2-C10 | Validate command documentation matches implementation | `cmd/invowk/validate.go` → `website/docs/reference/cli.mdx` | WARNING | cli-contract-drift |
| S2-C11 | Dependencies pages match validate command implementation | `cmd/invowk/cmd_validate*.go` → `website/docs/dependencies/` pages | WARNING | cli-contract-drift |
| S2-C12 | TUI pages match TUI command implementation and flags | `cmd/invowk/tui_*.go` → `website/docs/tui/` pages | WARNING | cli-contract-drift |
| S2-C13 | No broken internal links (verified by `npm run build` in Step 1) | Build output | ERROR | navigation-drift |
| S2-C14 | `website/sidebars.ts` includes every intentional current doc page in a deterministic category | `website/docs/`, `website/sidebars.ts` | ERROR | navigation-drift |

**Total items**: 14

---

## §S3: Snippet Data and CUE Schema Drift

**File scope** (all 12 snippet data files):
- `website/src/components/Snippet/data/core-concepts.ts`
- `website/src/components/Snippet/data/dependencies.ts`
- `website/src/components/Snippet/data/advanced.ts`
- `website/src/components/Snippet/data/modules.ts`
- `website/src/components/Snippet/data/runtime-modes.ts`
- `website/src/components/Snippet/data/config.ts`
- `website/src/components/Snippet/data/getting-started.ts`
- `website/src/components/Snippet/data/flags-args.ts`
- `website/src/components/Snippet/data/environment.ts`
- `website/src/components/Snippet/data/tui.ts`
- `website/src/components/Snippet/data/cli.ts`
- `website/src/components/Snippet/data/security.ts`

**Source of truth**: `pkg/invowkfile/invowkfile_schema.cue`, `pkg/invowkmod/invowkmod_schema.cue`,
`internal/config/config_schema.cue`. See `cue-drift-patterns.md` for the 6 patterns.

**Important exception**: Partial/fragment snippets that show individual CUE fields (e.g., just
`runtimes:` config) are intentionally incomplete. Only flag snippets that show a full `cmds` entry
with an implementation block but lack required fields.

### Pattern 1: Missing `platforms` field (per-file checks)

| ID | Check | Severity | Finding Type |
|---|---|---|---|
| S3-C01 | `core-concepts.ts` — All CUE snippets with `implementations:` blocks have `platforms:` (unless partial fragment) | ERROR | snippet-drift |
| S3-C02 | `dependencies.ts` — Same check | ERROR | snippet-drift |
| S3-C03 | `advanced.ts` — Same check | ERROR | snippet-drift |
| S3-C04 | `modules.ts` — Same check | ERROR | snippet-drift |
| S3-C05 | `runtime-modes.ts` — Same check | ERROR | snippet-drift |
| S3-C06 | `config.ts` — Same check | ERROR | snippet-drift |
| S3-C07 | `getting-started.ts` — Same check | ERROR | snippet-drift |
| S3-C08 | `flags-args.ts` — Same check | ERROR | snippet-drift |
| S3-C09 | `environment.ts` — Same check | ERROR | snippet-drift |
| S3-C10 | `tui.ts` — Same check | ERROR | snippet-drift |
| S3-C11 | `cli.ts` — Same check | ERROR | snippet-drift |
| S3-C12 | `security.ts` — Same check | ERROR | snippet-drift |

### Patterns 2–6: Cross-file checks (apply to all 12 files)

| ID | Check | Severity | Finding Type |
|---|---|---|---|
| S3-C13 | Pattern 2: `cmds` uses list syntax `[{...}]`, not map syntax `{...}` | ERROR | snippet-drift |
| S3-C14 | Pattern 3: `runtimes` and `platforms` are struct lists `[{name: "..."}]`, not string arrays `["..."]` | ERROR | snippet-drift |
| S3-C15 | Pattern 4: Module requires use `git_url:`, not `git:` | ERROR | snippet-drift |
| S3-C16 | Pattern 5: Version constraints have no `v` prefix (`"1.0.0"` not `"v1.0.0"`) | ERROR | snippet-drift |
| S3-C17 | Pattern 6: Config `includes` paths end with `.invowkmod` | ERROR | snippet-drift |

### General snippet quality

| ID | Check | Severity | Finding Type |
|---|---|---|---|
| S3-C18 | CUE field names in snippets match current schema field names (cross-reference against `.cue` files) | ERROR | schema-drift |
| S3-C19 | Snippet IDs referenced in MDX pages exist in the corresponding data file | ERROR | snippet-drift |

**Total items**: 19

---

## §S4: i18n Parity

**File scope**: `website/i18n/pt-BR/` (mirrors `website/docs/`)

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S4-C01 | `npm run docs:parity` passes (file parity, Snippet ID parity, Diagram ID parity) | Step 1 programmatic results | ERROR | i18n-structural-drift |
| S4-C02 | All `<Snippet id="...">` references in pt-BR MDX files match English counterparts | English `website/docs/` files | ERROR | i18n-structural-drift |
| S4-C03 | All `<Diagram id="...">` references in pt-BR MDX files match English counterparts | English `website/docs/` files | ERROR | i18n-structural-drift |
| S4-C04 | No missing pt-BR mirrors for English pages added in the last 30 days | `git log --since="30 days ago" --diff-filter=A -- website/docs/` | WARNING | i18n-structural-drift |
| S4-C05 | pt-BR pages modified more than 60 days after their English counterpart have stale-prose risk — spot-check 3 most recently modified English pages | `git log --diff-filter=M -- website/docs/` vs `git log -- website/i18n/pt-BR/` | WARNING | i18n-prose-staleness |
| S4-C06 | Exceptions in `website/docs-parity-exceptions.json` have valid justifications and refer to existing files | Exception file contents | INFO | i18n-structural-drift |

**Total items**: 6

---

## §S5: Architecture Diagrams

**File scope**:
- D2 source files: `docs/diagrams/` (23 `.d2` files)
- Architecture prose: `docs/architecture/` (8 `.md` files)
- Website architecture pages: `website/docs/architecture/`

**Source of truth mapping**: See `consolidated-sync-map.md` (diagram section).

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S5-C01 | D2 syntax validates for all 23 files (Step 1 `d2 validate` results) | Step 1 programmatic results | ERROR | diagram-drift |
| S5-C02 | Diagram readability checks pass (`check-diagram-readability.sh` results) | Step 1 programmatic results | WARNING | diagram-drift |
| S5-C03 | Discovery-related diagrams (`flowchart-discovery.md`, `sequence-execution.md`) match current `internal/discovery/` package structure | `internal/discovery/` package names, exported types | WARNING | diagram-drift |
| S5-C04 | Runtime-related diagrams (`flowchart-runtime-selection.md`, `sequence-execution.md`) match current `internal/runtime/` structure | `internal/runtime/` package names, runtime types | WARNING | diagram-drift |
| S5-C05 | Command flow diagrams (`sequence-execution.md`) match current `cmd/invowk/` command structure | `cmd/invowk/` command files | WARNING | diagram-drift |
| S5-C06 | Discovery cache diagram (`flowcharts/discovery-cache.d2`) matches `cmd/invowk/app.go` cache implementation | `cmd/invowk/app.go` | WARNING | diagram-drift |
| S5-C07 | C4 container diagram (`c4-container.md`) includes all current packages and components | Actual package structure | WARNING | diagram-drift |
| S5-C08 | C4 context diagram (`c4-context.md`) reflects current external integrations | External integration points in code | WARNING | diagram-drift |
| S5-C09 | Server diagrams match current SSH/TUI server implementations | `internal/sshserver/`, `internal/tuiserver/` | WARNING | diagram-drift |
| S5-C10 | Architecture prose docs in `docs/architecture/` reference correct package names and component relationships | Actual package structure | INFO | source-drift |
| S5-C11 | Website architecture pages in `website/docs/architecture/` are consistent with `docs/architecture/` prose | `docs/architecture/` source files | INFO | source-drift |

**Total items**: 11

---

## §S6: Container Image Policy

**File scope** (cross-cutting — checks all documentation surfaces):
- `README.md`
- `website/docs/` (all MDX files)
- `website/src/components/Snippet/data/*.ts` (all snippet data)
- `website/i18n/` (all i18n files)
- `docs/architecture/` (architecture prose)

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S6-C01 | No `ubuntu:*` image references in any documentation file | Step 1 grep results + manual scan | ERROR | policy-violation |
| S6-C02 | No `alpine:*` image references in any documentation file | Step 1 grep results + manual scan | ERROR | policy-violation |
| S6-C03 | No `mcr.microsoft.com/windows/*` image references in any documentation file | Step 1 grep results + manual scan | ERROR | policy-violation |
| S6-C04 | All generic container examples use `debian:stable-slim` | Manual scan of CUE snippets with container runtime | ERROR | policy-violation |
| S6-C05 | Language-specific images (e.g., `golang:1.26`, `python:3-slim`, `node:22-slim`) appear only in language-specific runtime demos, not in general container examples | Manual scan | WARNING | policy-violation |
| S6-C06 | CUE snippets with `container` runtime have `image:` field referencing `debian:stable-slim` (unless language-specific demo) | Snippet data files with container runtime config | ERROR | policy-violation |

**Total items**: 6

---

## §S7: DefaultConfig() vs Docs

**File scope**:
- Source of truth: `internal/config/types.go` (`DefaultConfig()` function)
- Schema: `internal/config/config_schema.cue`
- Doc targets:
  - `website/docs/reference/config-schema.mdx`
  - `website/docs/configuration/options.mdx`
  - `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/reference/config-schema.mdx`
  - `website/i18n/pt-BR/docusaurus-plugin-content-docs/current/configuration/options.mdx`

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S7-C01 | All fields in `DefaultConfig()` are documented in `config-schema.mdx` with matching default values | `internal/config/types.go` → `config-schema.mdx` | ERROR | source-drift |
| S7-C02 | All fields in `DefaultConfig()` are documented in `options.mdx` with matching default values | `internal/config/types.go` → `options.mdx` | ERROR | source-drift |
| S7-C03 | Config field names in docs match CUE schema field names (JSON tags in Go struct match CUE field names) | `internal/config/config_schema.cue` | ERROR | schema-drift |
| S7-C04 | pt-BR mirror of `config-schema.mdx` has matching default values and field names | English `config-schema.mdx` | ERROR | i18n-prose-staleness |
| S7-C05 | pt-BR mirror of `options.mdx` has matching default values and field names | English `options.mdx` | ERROR | i18n-prose-staleness |
| S7-C06 | Dual-prefix config snippets in `config.ts`: every `config/X` snippet has a corresponding `reference/config/X` with equivalent content | `website/src/components/Snippet/data/config.ts` | WARNING | snippet-drift |
| S7-C07 | Config snippet content matches current `DefaultConfig()` values | `internal/config/types.go` | ERROR | snippet-drift |

**Total items**: 7

---

## §S8: Homepage and Terminal Demo

**File scope**: `website/src/pages/index.tsx`

**Reference**: `intentional-simplifications.md` registry entries for homepage and terminal demo.

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S8-C01 | Terminal demo CUE syntax is valid (even if simplified, it should not show broken syntax) | CUE language syntax rules | ERROR | cli-contract-drift |
| S8-C02 | Terminal demo CLI commands shown are real invowk commands (not renamed/removed ones) | `cmd/invowk/*.go` | ERROR | cli-contract-drift |
| S8-C03 | Terminal demo does not show features that have been removed or significantly changed | Current codebase behavior | WARNING | source-drift |
| S8-C04 | Homepage feature claims match current capabilities (no removed features, no future features) | `pkg/`, `internal/`, `cmd/` code | WARNING | source-drift |
| S8-C05 | Intentional simplifications match registry entries in `intentional-simplifications.md` (no new unregistered simplifications) | `intentional-simplifications.md` | INFO | source-drift |

**Total items**: 5

---

## §S9: Security Audit Docs

**File scope**:
- `website/docs/security/audit.mdx`
- `website/src/components/Snippet/data/security.ts`
- `website/docs/reference/cli.mdx` audit entries
- `README.md` Security Auditing section

**Source of truth**:
- `cmd/invowk/audit.go`
- `cmd/invowk/llm_flags.go`
- `internal/audit/`
- `internal/auditllm/`
- `internal/app/llmconfig/`

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S9-C01 | Audit command syntax, path argument, and flags match Cobra command definitions | `cmd/invowk/audit.go`, `cmd/invowk/llm_flags.go` | ERROR | security-contract-drift |
| S9-C02 | Audit exit codes in docs match `auditExitClean`, `auditExitFindings`, and `auditExitError` | `cmd/invowk/audit.go` | ERROR | security-contract-drift |
| S9-C03 | JSON output fields, summary fields, and suppressed-finding fields match CLI DTOs | `cmd/invowk/audit.go` | ERROR | security-contract-drift |
| S9-C04 | Severity levels and filtering docs match `internal/audit/severity.go` | `internal/audit/severity.go` | ERROR | security-contract-drift |
| S9-C05 | Category names and checker descriptions match current built-in checkers | `internal/audit/types.go`, `internal/audit/checks_*.go`, `internal/audit/checker.go` | WARNING | security-contract-drift |
| S9-C06 | Finding-code and compound-threat descriptions match current correlator rules | `internal/audit/finding_codes.go`, `internal/audit/correlator.go` | WARNING | security-contract-drift |
| S9-C07 | LLM audit docs state explicit opt-in behavior and provider/config rules accurately | `cmd/invowk/audit.go`, `cmd/invowk/llm_flags.go`, `internal/app/llmconfig/` | ERROR | security-contract-drift |
| S9-C08 | Security audit snippets and reference CLI snippets match current commands and flags | `website/src/components/Snippet/data/security.ts`, `cmd/invowk/audit.go` | ERROR | snippet-drift |
| S9-C09 | README security audit section matches website audit docs and source of truth | `README.md`, `website/docs/security/audit.mdx`, audit code | WARNING | source-drift |
| S9-C10 | Audit CI examples avoid unsafe shell patterns and accurately handle exit code 1 findings | `cmd/invowk/audit.go`, documented CI snippets | WARNING | security-contract-drift |

**Total items**: 10

---

## §S10: LLM-Assisted Command Authoring Docs

**File scope**:
- `website/docs/advanced/llm-assisted-authoring.mdx`
- `website/src/components/Snippet/data/advanced.ts`
- `website/docs/reference/cli.mdx` agent command entries
- `README.md` LLM-Assisted Command Authoring section

**Source of truth**:
- `cmd/invowk/agent.go`
- `cmd/invowk/llm_flags.go`
- `internal/agentcmd/`
- `internal/app/llmconfig/`
- `internal/llm/`

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S10-C01 | `invowk agent cmd prompt` syntax, flags, and output-format values match implementation | `cmd/invowk/agent.go`, `internal/agentcmd/prompt.go` | ERROR | cli-contract-drift |
| S10-C02 | `invowk agent cmd create` syntax, flags, and mutually exclusive mode rules match implementation | `cmd/invowk/agent.go` | ERROR | cli-contract-drift |
| S10-C03 | Shared LLM flags and defaults match `bindLLMFlags` and resolver defaults | `cmd/invowk/llm_flags.go`, `internal/app/llmconfig/`, `internal/auditllm/` | ERROR | cli-contract-drift |
| S10-C04 | Docs correctly distinguish configured-default behavior for `agent cmd create` from audit opt-in behavior | `cmd/invowk/agent.go`, `cmd/invowk/audit.go`, `internal/app/llmconfig/` | ERROR | source-drift |
| S10-C05 | Validation behavior for generated commands matches `internal/agentcmd` implementation | `internal/agentcmd/create.go`, `internal/agentcmd/patch.go` | WARNING | source-drift |
| S10-C06 | Snippets referenced by LLM authoring docs exist and match current CLI behavior | `website/src/components/Snippet/data/advanced.ts`, `cmd/invowk/agent.go` | ERROR | snippet-drift |
| S10-C07 | README LLM authoring section matches website docs and source of truth | `README.md`, `website/docs/advanced/llm-assisted-authoring.mdx`, agent code | WARNING | source-drift |
| S10-C08 | Privacy/caution wording accurately states what content is sent to the configured provider | `internal/agentcmd/create.go`, `cmd/invowk/agent.go` | WARNING | source-drift |

**Total items**: 8

---

## §S11: Agent Workflow Docs

**File scope**:
- `.agents/skills/review-docs/SKILL.md`
- `.agents/skills/review-docs/references/*.md`
- `.agents/commands/review-docs.md`
- `.agents/skills/docs/SKILL.md`
- `AGENTS.md`

**Source of truth**: Live repository documentation surfaces and this checklist.

| ID | Check | Source of Truth | Severity | Finding Type |
|---|---|---|---|---|
| S11-C01 | Live website docs inventory is fully assigned to checklist surfaces | `website/docs/`, `surface-checklists.md`, `SKILL.md` | ERROR | coverage-gap |
| S11-C02 | Live snippet data inventory is fully assigned to checklist surfaces | `website/src/components/Snippet/data/`, `surface-checklists.md` | ERROR | coverage-gap |
| S11-C03 | Slash-command wrapper references the current number of surfaces and subagents | `.agents/commands/review-docs.md`, `SKILL.md` | ERROR | agent-docs-drift |
| S11-C04 | Related docs skill references current snippet file count and sync-map guidance | `.agents/skills/docs/SKILL.md`, live snippet data files | WARNING | agent-docs-drift |
| S11-C05 | Structured output format lists every active surface and required finding fields | `structured-output-format.md`, `surface-checklists.md` | ERROR | agent-docs-drift |
| S11-C06 | Verification command reference includes all current mechanical gates | `verification-commands.md`, `website/package.json`, `scripts/` | WARNING | agent-docs-drift |
| S11-C07 | `AGENTS.md` indexes still match `.agents/skills/`, `.agents/commands/`, and `.agents/rules/` | `make check-agent-docs` | ERROR | agent-docs-drift |
| S11-C08 | Review-docs skill body and reference totals match checklist totals | `SKILL.md`, `surface-checklists.md` | ERROR | agent-docs-drift |
| S11-C09 | Generated-version asset checks are represented in the workflow without reviewing frozen versioned docs manually | `scripts/validate-version-assets.mjs`, `verification-commands.md`, `SKILL.md` | INFO | generated-asset-drift |

**Total items**: 9

---

## Totals

| Surface | Items |
|---------|-------|
| S1: README | 22 |
| S2: Website Docs | 14 |
| S3: Snippet Data & CUE Drift | 19 |
| S4: i18n Parity | 6 |
| S5: Architecture Diagrams | 11 |
| S6: Container Image Policy | 6 |
| S7: DefaultConfig() vs Docs | 7 |
| S8: Homepage & Terminal Demo | 5 |
| S9: Security Audit Docs | 10 |
| S10: LLM-Assisted Command Authoring Docs | 8 |
| S11: Agent Workflow Docs | 9 |
| **Grand total** | **117** |
