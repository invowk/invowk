# README Section → Source of Truth Map

Maps each major section of `README.md` to the authoritative source(s) in the codebase.
During review, read the source of truth first, then compare with the README section.

| README Section | Line | Source of Truth | Review Focus |
|---|---|---|---|
| Features | L5 | Code capabilities across `pkg/`, `internal/`, `cmd/` | Feature list completeness and accuracy |
| Installation | L36 | `scripts/install.sh`, `scripts/install.ps1`, `.goreleaser.yml` | URLs, platform table, Cosign verify command, install paths |
| Quick Start | L151 | `cmd/invowk/init.go`, actual `invowk init` output | Generated invowkfile content, CLI output format, flag names |
| Invowkfile Format | L199 | `pkg/invowkfile/invowkfile_schema.cue` | CUE field names, types, constraints, required fields |
| Module Metadata | L369 | `pkg/invowkmod/invowkmod_schema.cue` | CUE field names, `requires` structure, module ID format |
| Dependencies | L486 | `pkg/invowkfile/invowkfile_schema.cue` `#DependencyConfig` | Dependency types (`tools`, `filepaths`, `capabilities`, `custom`), syntax |
| Command Flags | L722 | `pkg/invowkfile/invowkfile_schema.cue` `#FlagConfig` | Flag types, syntax, validation options |
| Command Arguments | L986 | `pkg/invowkfile/invowkfile_schema.cue` `#ArgumentConfig` | Argument types, positional syntax, validation |
| Platform Compatibility | L1362 | `pkg/invowkfile/invowkfile_schema.cue` `#PlatformConfig` | Platform names ("macos" not "darwin"), struct format |
| Script Sources | L1493 | `pkg/invowkfile/invowkfile_schema.cue` `#Implementation` | `source_file` syntax, path resolution |
| Interpreter Support | L1554 | `pkg/invowkfile/invowkfile_schema.cue` `#Interpreter` | Interpreter config format |
| Modules | L1711 | `pkg/invowkmod/`, module directory conventions | Module structure, RDNS naming, file layout |
| Module Dependencies | L1977 | `pkg/invowkmod/invowkmod_schema.cue` `#RequiredModule` | `requires` syntax, lock file format, `git_url` (not `git`) |
| Runtime Modes | L2157 | `internal/runtime/` | Runtime descriptions, capabilities, selection logic |
| Configuration | L2301 | `internal/config/config_schema.cue`, `internal/config/types.go` `DefaultConfig()` | Config fields, default values, file location |
| Shell Completion | L2379 | `cmd/invowk/completion.go` | Supported shells, generation commands |
| Command Examples | L2416 | Actual CLI behavior | Example command accuracy, output format |
| Interactive TUI Components | L2502 | `cmd/invowk/tui_*.go` | Component list, flags, behavior descriptions |
| Project Structure | L2739 | Actual directory layout | Directory descriptions match reality |
| Dependencies (Go) | L2786 | `go.mod` | Dependency list and version accuracy |
| Performance and PGO | L2812 | `Makefile`, `default.pgo`, `internal/benchmark/` | PGO description accuracy |
| Local SonarCloud | L2833 | `scripts/sonar-local.sh` | Command accuracy |
| Contributing | L2857 | Actual contribution process | Accuracy |

## README-Specific Review Notes

- The README is ~2870 lines. Focus review time on the most drift-prone sections: Invowkfile Format, Dependencies, Command Flags/Arguments, Module Dependencies, and Configuration.
- README examples are intentionally self-contained and may not show every optional field. This is progressive disclosure, not an error.
- The Quick Start section should match `invowk init` output exactly — run `invowk init --help` and compare.
- Platform names in README must use "macos" (not "darwin") in CUE examples, matching the schema.
