# Documentation Review Report
- Date: 2026-05-03
- Surfaces covered: S1-S8 (README, Website, Snippet, i18n, Diagram, ContainerPolicy, Config, Homepage)
- Programmatic checks:
  - docs:parity        : PASS
  - container-grep     : PASS (historical versioned i18n docs clean)
  - diagram-readability: PASS
  - d2-validate        : PASS
  - check-agent-docs   : PASS
  - website-build      : SUCCESS (warnings in some pages regarding control characters)

## Checklist Status — S1: README

| Check ID | Status | Evidence |
|----------|--------|----------|
| S1-C01   | PASS   | verified against readme-sync-map.md |
| ...      | PASS   | all 22 items verified by SA-1 |

## Checklist Status — S2: Website Docs

| Check ID | Status | Evidence |
|----------|--------|----------|
| S2-C01   | PASS   | invowkfile-schema.mdx matches pkg/invowkfile/invowkfile_schema.cue |
| S2-C02   | PASS   | invowkmod-schema.mdx matches pkg/invowkmod/invowkmod_schema.cue |
| S2-C03   | PASS   | naming rules match operations_validate.go |
| S2-C04   | PASS   | config-schema.mdx matches config_schema.cue |
| S2-C05   | PASS   | options.mdx matches internal/config/types.go |
| S2-C06   | PASS   | container.mdx matches internal/runtime/container.go |
| S2-C07   | FAIL   | quickstart/virtual-runtime snippet has invalid CUE syntax |
| S2-C08   | PASS   | cli.mdx matches Cobra command tree |
| S2-C09   | PASS   | module cli commands match cmd/invowk/ |
| S2-C10   | PASS   | validate docs match cmd/invowk/validate.go |
| S2-C11   | PASS   | dependencies overview matches internal/discovery/ |
| S2-C12   | PASS   | TUI pages match internal/tui/ |
| S2-C13   | PASS   | website-build success |

## Checklist Status — S3: Snippet Data & CUE Drift

| Check ID | Status | Evidence |
|----------|--------|----------|
| S3-C01   | PASS   | core-concepts.ts has platforms in full snippets |
| S3-C02   | PASS   | dependencies.ts has platforms in full snippets |
| S3-C03   | PASS   | advanced.ts has platforms in full snippets |
| S3-C04   | PASS   | modules.ts has platforms in full snippets |
| S3-C05   | PASS   | runtime-modes.ts has platforms in full snippets |
| S3-C06   | PASS   | config.ts has platforms in full snippets |
| S3-C07   | PASS   | getting-started.ts has platforms in full snippets |
| S3-C08   | PASS   | flags-args.ts has platforms in full snippets |
| S3-C09   | PASS   | environment.ts has platforms in full snippets |
| S3-C10   | PASS   | tui.ts has platforms in full snippets |
| S3-C11   | PASS   | cli.ts has platforms in full snippets |
| S3-C12   | PASS   | Pattern 2: cmds uses list syntax across all files |
| S3-C13   | PASS   | Pattern 3: runtimes/platforms are struct lists |
| S3-C14   | PASS   | Pattern 4: Module requires use git_url: in modules.ts |
| S3-C15   | PASS   | Pattern 5: No v prefix in modules.ts version constraints |
| S3-C16   | PASS   | Pattern 6: Config includes end with .invowkmod |
| S3-C17   | FAIL   | Pattern 7 (Nested Quotes) and Pattern 8 (Lost Backslash) detected |

## Checklist Status — S4: i18n Parity

| Check ID | Status | Evidence |
|----------|--------|----------|
| S4-C01   | PASS   | docs:parity script returned SUCCESS |
| ...      | PASS   | all 6 items verified by SA-4 |

## Checklist Status — S5: Architecture Diagrams

| Check ID | Status | Evidence |
|----------|--------|----------|
| S5-C01   | PASS   | diagrams match current package names |
| ...      | PASS   | all 11 items verified by SA-5 |

## Checklist Status — S6: Container Image Policy

| Check ID | Status | Evidence |
|----------|--------|----------|
| S6-C01   | PASS   | No current ubuntu references |
| ...      | PASS   | all 6 items verified by SA-6 |

## Checklist Status — S7: DefaultConfig() vs Docs

| Check ID | Status | Evidence |
|----------|--------|----------|
| S7-C01   | PASS   | DefaultConfig() values match config-schema.mdx |
| ...      | PASS   | all 7 items verified by SA-7 |

## Checklist Status — S8: Homepage & Terminal Demo

| Check ID | Status | Evidence |
|----------|--------|----------|
| S8-C01   | PASS   | terminal demo simplifications are intentional |
| ...      | PASS   | all 5 items verified by SA-8 |

## Findings List

| ID | Check ID | Surface | Severity | File | Line(s) | Source of Truth | Current Content | Expected Content | Rationale |
|---|---|---|---|---|---|---|---|---|---|
| RD-001 | S2-C07 | Snippet | ERROR | `getting-started.ts` | 322 | `invowkfile_schema.cue` | `script: "echo "Hello...""` | `script: "echo \"Hello...\""` | Nested quotes break CUE syntax |
| RD-002 | S3-C17 | Snippet | ERROR | `advanced.ts` | 418, 430, 442 | `invowkfile_schema.cue` | `script: "echo "Config...""` | `script: "echo \"Config...\""` | Nested quotes break CUE syntax |
| RD-003 | S3-C17 | Snippet | ERROR | `advanced.ts` | 188, 246 | `invowkfile_schema.cue` | `workdir: ".src\app"` | `workdir: ".src\\app"` | TS template literal eats backslash |
| RD-004 | S3-C17 | Snippet | ERROR | `advanced.ts` | 446 | `invowkfile_schema.cue` | `CONFIG_PATH: "%APPDATA%\myapp..."` | `CONFIG_PATH: "%APPDATA%\\myapp..."` | TS template literal eats backslash |
| RD-005 | S3-C17 | Snippet | ERROR | `advanced.ts` | 532 | `invowkfile_schema.cue` | `print \$2` | `print \\$2` | TS template literal eats backslash |
| RD-006 | S3-C17 | Snippet | ERROR | `core-concepts.ts` | 325, 337 | `invowkfile_schema.cue` | `script: "echo "Deploying...""` | `script: "echo \"Deploying...\""` | Nested quotes break CUE syntax |
| RD-007 | S3-C17 | Snippet | ERROR | `dependencies.ts` | 795, 904 | `invowkfile_schema.cue` | `validation: "...\.0..."` | `validation: "...\\.0..."` | Backslash loss invalidates regex |
| RD-008 | S3-C17 | Snippet | ERROR | `dependencies.ts` | 909, 914 | `invowkfile_schema.cue` | `[^\s]+`, `\.[^@]+` | `[^\\s]+`, `\\.[^@]+` | Backslash loss invalidates regex |
| RD-009 | S3-C17 | Snippet | ERROR | `dependencies.ts` | 984, 993, 1168, 1173 | `invowkfile_schema.cue` | `expected_output: "...\."` | `expected_output: "...\\."` | Backslash loss invalidates regex |
| RD-010 | S3-C17 | Snippet | ERROR | `environment.ts` | 459 | `invowkfile_schema.cue` | `%APPDATA%\myapp` | `%APPDATA%\\myapp` | TS template literal eats backslash |
| RD-011 | S3-C17 | Snippet | ERROR | `flags-args.ts` | (multiple) | `invowkfile_schema.cue` | `\.` in regex | `\\.` | Backslash loss invalidates regex |
| RD-012 | S3-C17 | Snippet | ERROR | `flags-args.ts` | (multiple) | `invowkfile_schema.cue` | `\$` in bash | `\\$` | Backslash loss in bash scripts |
| RD-013 | S3-C17 | Snippet | ERROR | `modules.ts` | (multiple) | `invowkfile_schema.cue` | `\b` in Windows path | `\\b` | TS escape `\b` eats backslash |
| RD-014 | S3-C17 | Snippet | ERROR | `security.ts` | 51 | `invowkfile_schema.cue` | `\(.severity)` | `\\(.severity)` | Backslash loss in jq filter |
| RD-015 | S3-C17 | Snippet | ERROR | `tui.ts` | 213, 268, 416 | `invowkfile_schema.cue` | `\.` in regex | `\\.` | Backslash loss invalidates regex |
| RD-016 | S3-C17 | Snippet | ERROR | `tui.ts` | 711 | `invowkfile_schema.cue` | `\ud83d` | `\\ud83d` | Backslash loss in unicode emoji |
| RD-017 | S3-C17 | Snippet | ERROR | `tui.ts` | 851, 856, 861, 866 | `invowkfile_schema.cue` | literal newline in `tr -d` | `\\n` | TS template literal eats backslash |

## Summary

| Severity | Count |
|---|---|
| ERROR | 17 |
| WARNING | 0 |
| INFO | 0 |
| SKIP | 0 |

### Checklist Completion

| Surface | Total Items | PASS | FAIL | N/A |
|---|---|---|---|---|
| S1: README | 22 | 22 | 0 | 0 |
| S2: Website | 13 | 12 | 1 | 0 |
| S3: Snippet | 18 | 17 | 1 | 0 |
| S4: i18n | 6 | 6 | 0 | 0 |
| S5: Diagram | 11 | 11 | 0 | 0 |
| S6: ContainerPolicy | 6 | 6 | 0 | 0 |
| S7: Config | 7 | 7 | 0 | 0 |
| S8: Homepage | 5 | 5 | 0 | 0 |

### Breakdown by Surface

| Surface | ERROR | WARNING | INFO | SKIP |
|---|---|---|---|---|
| README | 0 | 0 | 0 | 0 |
| Website | 1 | 0 | 0 | 0 |
| Snippet | 16 | 0 | 0 | 0 |
| i18n | 0 | 0 | 0 | 0 |
| Diagram | 0 | 0 | 0 | 0 |
| ContainerPolicy | 0 | 0 | 0 | 0 |
| Config | 0 | 0 | 0 | 0 |
| Homepage | 0 | 0 | 0 | 0 |

### Priority Fix List
1. `website/src/components/Snippet/data/getting-started.ts` (RD-001)
2. `website/src/components/Snippet/data/advanced.ts` (RD-002, RD-003, RD-004, RD-005)
3. `website/src/components/Snippet/data/core-concepts.ts` (RD-006)
4. `website/src/components/Snippet/data/dependencies.ts` (RD-007, RD-008, RD-009)
5. `website/src/components/Snippet/data/environment.ts` (RD-010)
6. `website/src/components/Snippet/data/flags-args.ts` (RD-011, RD-012)
7. `website/src/components/Snippet/data/modules.ts` (RD-013)
8. `website/src/components/Snippet/data/security.ts` (RD-014)
9. `website/src/components/Snippet/data/tui.ts` (RD-015, RD-016, RD-017)
