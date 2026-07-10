# README Section → Source of Truth Map

Maps each major section of `README.md` to the authoritative source(s) in the codebase.
During review, read the source of truth first, then compare with the README section.

| README Section | Stable Locator | Source of Truth | Review Focus |
|---|---|---|---|
| Features | `## Features` | Code capabilities across `pkg/`, `internal/`, `cmd/` | Feature list completeness and accuracy |
| Installation | `## Installation` | `scripts/install.sh`, `scripts/install.ps1`, `.goreleaser.yaml` | URLs, platform table, Cosign verify command, install paths |
| Quick Start | `## Quick Start` | `cmd/invowk/init.go`, actual `invowk init` output | Generated invowkfile content, CLI output format, flag names |
| LLM-Assisted Agent Authoring | `## LLM-Assisted Agent Authoring` | `cmd/invowk/agent.go`, `internal/agentcmd/`, LLM flags/config | Provider behavior, command/module generated CUE validation, create/change/remove examples |
| Invowkfile Format | `## Invowkfile Format` | `pkg/invowkfile/invowkfile_schema.cue` | CUE field names, types, constraints, required fields |
| Module Metadata | `## Module Metadata (invowkmod.cue)` | `pkg/invowkmod/invowkmod_schema.cue` | CUE field names, `requires` structure, module ID format |
| Dependencies | `## Dependencies` | `pkg/invowkfile/invowkfile_schema.cue` `#DependsOn` | Dependency fields (`tools`, `cmds`, `filepaths`, `env_vars`, `capabilities`, `custom_checks`), syntax |
| Command Flags | `## Command Flags` | `pkg/invowkfile/invowkfile_schema.cue` `#Flag` | Flag types, syntax, validation options |
| Command Arguments | `## Command Arguments (Positional Arguments)` | `pkg/invowkfile/invowkfile_schema.cue` `#Argument` | Argument types, positional syntax, validation |
| Platform Compatibility | `## Platform Compatibility` | `pkg/invowkfile/invowkfile_schema.cue` `#PlatformConfig` | Platform names ("macos" not "darwin"), struct format |
| Script Sources | `## Script Sources` | `pkg/invowkfile/invowkfile_schema.cue` `#Implementation` | `script.content` / `script.file` syntax, path resolution |
| Interpreter Support | `## Interpreter Support` | `pkg/invowkfile/invowkfile_schema.cue` `#InterpreterSpec` | Interpreter config format |
| Modules | `## Modules` | `pkg/invowkmod/`, module directory conventions | Module structure, RDNS naming, file layout |
| Module Dependencies | `## Module Dependencies` | `pkg/invowkmod/invowkmod_schema.cue` `#ModuleRequirement` | `requires` syntax, lock file format, `git_url` (not `git`) |
| Runtime Modes | `## Runtime Modes` | `internal/runtime/` | Runtime descriptions, capabilities, selection logic |
| Configuration | `## Configuration` | `internal/config/config_schema.cue`, CUE-derived `DefaultConfig()` | Config fields, default values, file location |
| Shell Completion | `## Shell Completion` | `cmd/invowk/completion.go` | Supported shells, generation commands |
| Command Examples | `## Command Examples` | Actual CLI behavior | Example command accuracy, output format |
| Interactive TUI Components | `## Interactive TUI Components` | `cmd/invowk/tui_*.go` | Component list, flags, behavior descriptions |
| Security Auditing | `## Security Auditing` | `cmd/invowk/audit.go`, `internal/audit/`, `internal/auditllm/` | Scan scope, report formats, LLM opt-in behavior |
| Project Structure | `## Project Structure` | Actual directory layout | Directory descriptions match reality |
| Dependencies (Go) | `## Dependencies` | `go.mod` | Dependency list and version accuracy |
| Performance and PGO | `## Performance and PGO` | `Makefile`, `default.pgo`, `internal/benchmark/` | PGO description accuracy |
| Local SonarCloud | `## Local SonarCloud Status Check` | `scripts/sonar-local.sh` | Command accuracy |
| Contributing | `## Contributing` | Actual contribution process | Accuracy |

## README-Specific Review Notes

- Use headings as stable locators. Do not maintain line-number anchors; they drift whenever earlier
  sections change.
- README examples are intentionally self-contained and may not show every optional field. This is progressive disclosure, not an error.
- The Quick Start section should match `invowk init` output exactly — run `invowk init --help` and compare.
- Platform names in README must use "macos" (not "darwin") in CUE examples, matching the schema.
