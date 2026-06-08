# README Section → Source of Truth Map

Maps each major section of `README.md` to the authoritative source(s) in the codebase.
During review, read the source of truth first, then compare with the README section.

| README Section | Line | Source of Truth | Review Focus |
|---|---|---|---|
| Features | L5 | Code capabilities across `pkg/`, `internal/`, `cmd/` | Feature list completeness and accuracy |
| Installation | L41 | `scripts/install.sh`, `scripts/install.ps1`, `.goreleaser.yaml` | URLs, platform table, Cosign verify command, install paths |
| Quick Start | L156 | `cmd/invowk/init.go`, actual `invowk init` output | Generated invowkfile content, CLI output format, flag names |
| LLM-Assisted Command Authoring | L204 | `cmd/invowk/agent.go`, `internal/agentcmd/`, LLM flags/config | Provider behavior, generated command validation, examples |
| Invowkfile Format | L237 | `pkg/invowkfile/invowkfile_schema.cue` | CUE field names, types, constraints, required fields |
| Module Metadata | L410 | `pkg/invowkmod/invowkmod_schema.cue` | CUE field names, `requires` structure, module ID format |
| Dependencies | L559 | `pkg/invowkfile/invowkfile_schema.cue` `#DependsOn` | Dependency fields (`tools`, `cmds`, `filepaths`, `env_vars`, `capabilities`, `custom_checks`), syntax |
| Command Flags | L796 | `pkg/invowkfile/invowkfile_schema.cue` `#Flag` | Flag types, syntax, validation options |
| Command Arguments | L1060 | `pkg/invowkfile/invowkfile_schema.cue` `#Argument` | Argument types, positional syntax, validation |
| Platform Compatibility | L1436 | `pkg/invowkfile/invowkfile_schema.cue` `#PlatformConfig` | Platform names ("macos" not "darwin"), struct format |
| Script Sources | L1567 | `pkg/invowkfile/invowkfile_schema.cue` `#Implementation` | `script.content` / `script.file` syntax, path resolution |
| Interpreter Support | L1631 | `pkg/invowkfile/invowkfile_schema.cue` `#InterpreterSpec` | Interpreter config format |
| Modules | L1798 | `pkg/invowkmod/`, module directory conventions | Module structure, RDNS naming, file layout |
| Module Dependencies | L2064 | `pkg/invowkmod/invowkmod_schema.cue` `#ModuleRequirement` | `requires` syntax, lock file format, `git_url` (not `git`) |
| Runtime Modes | L2261 | `internal/runtime/` | Runtime descriptions, capabilities, selection logic |
| Configuration | L2464 | `internal/config/config_schema.cue`, CUE-derived `DefaultConfig()` | Config fields, default values, file location |
| Shell Completion | L2564 | `cmd/invowk/completion.go` | Supported shells, generation commands |
| Command Examples | L2601 | Actual CLI behavior | Example command accuracy, output format |
| Interactive TUI Components | L2706 | `cmd/invowk/tui_*.go` | Component list, flags, behavior descriptions |
| Security Auditing | L2952 | `cmd/invowk/audit.go`, `internal/audit/`, `internal/auditllm/` | Scan scope, report formats, LLM opt-in behavior |
| Project Structure | L3164 | Actual directory layout | Directory descriptions match reality |
| Dependencies (Go) | L3241 | `go.mod` | Dependency list and version accuracy |
| Performance and PGO | L3274 | `Makefile`, `default.pgo`, `internal/benchmark/` | PGO description accuracy |
| Local SonarCloud | L3295 | `scripts/sonar-local.sh` | Command accuracy |
| Contributing | L3320 | Actual contribution process | Accuracy |

## README-Specific Review Notes

- The README is ~3330 lines. Focus review time on the most drift-prone sections: Invowkfile Format, Dependencies, Command Flags/Arguments, Module Dependencies, Configuration, LLM-Assisted Command Authoring, and Security Auditing.
- README examples are intentionally self-contained and may not show every optional field. This is progressive disclosure, not an error.
- The Quick Start section should match `invowk init` output exactly — run `invowk init --help` and compare.
- Platform names in README must use "macos" (not "darwin") in CUE examples, matching the schema.
