# Repository Map

On-demand orientation. Rules load automatically through their `paths:` frontmatter, so the routing table below is advisory: it names the skills worth loading for each area. Each package's doc comment (usually `doc.go`, where present) is authoritative for its own scope.

## Layout

- `cmd/invowk/`: Cobra CLI adapters. Thin wrappers that parse flags, build requests, call application services and render output.
- `internal/app/`: application services (hexagonal domain layer).
  - `commandsvc/`: command execution pipeline (discovery, validation, SSH lifecycle, runtime dispatch). Returns raw typed errors; the CLI adapter renders them.
  - `commandadapters/`: infrastructure adapters for the command service (including interactive virtual-shell and Lua plumbing).
  - `deps/`: dependency validation. Host deps always, container deps for the container runtime; command-scope check `CheckCommandDependenciesExist()`. Exported `DependencyError`, `ArgumentValidationError`.
  - `execute/`: runtime resolution and execution-context construction.
  - `modulecache/`, `moduleops/`, `modulesync/`: module cache operations, module create/vendor/packaging, and lock/sync resolution.
  - `llmconfig/`: LLM defaults, environment overrides and CLI flag resolution.
- `internal/` (other):
  - `config/`: configuration with CUE schema (`config_schema.cue`).
  - `modulelock/`: OS advisory edit locks for module commands (flock on Unix, LockFileEx on Windows) and physical project paths; the adapter of `modulesync.EditLocker`.
  - `basedir/`: the one resolver for per-user base directories (home, XDG/AppData, runtime dir); every result is absolute. `TestBaseDirGuard` keeps other packages off direct lookups.
  - `container/`: Docker/Podman engine abstraction. `containerplan/`: pure container execution policies. `provision/`: ephemeral image-layer provisioning. `provisionenv/`: environment contract between container provisioning and runtime.
  - `discovery/`: module and command discovery, precedence, collisions, source tracking.
  - `runtime/`: runtimes `native`, `virtual-sh`, `virtual-lua`, `container`, all implementing the interface in `runtime.go`.
  - `uroot/`: u-root built-in utilities for the virtual runtimes.
  - `core/serverbase/`: shared server state machine (used by `sshserver/`, `tuiserver/`).
  - `sshserver/`: SSH server for container callbacks. `tuiserver/`, `tuiclient/`, `tuiwire/`: delegated TUI rendering server, child-process client and wire vocabulary. `tui/`: Charm-based TUI components (bubbletea, bubbles, lipgloss).
  - `audit/`: security scanning of invowkfiles and modules. `auditllm/`: LLM client for audit.
  - `llm/`: neutral LLM contracts. `agentcmd/`: LLM-assisted command authoring. `acpclient/`: dormant Agent Client Protocol client.
  - `issue/`: `ActionableError` and user-facing error guidance.
  - `watch/`: file watching with debounced re-execution for `--ivk-watch`.
  - `benchmark/`: benchmarks for PGO profile generation.
  - `testutil/`: test helpers (`pathmatrix`, `invowkfiletest`, `fstree`, `procgate`, `schematest`, `alloygolden`, `tlatrace`, `mpctree`, container semaphore/context).
- `pkg/`: public packages `containerargs`, `cueutil`, `fspath`, `invowkfile`, `invowkmod`, `platform`, `types`.
- `tests/cli/`: testscript CLI tests (`.txtar` in `testdata/`); mirror exemptions in `runtime_mirror_exemptions.json`.
- `samples/invowkmods/`: sample modules and audit fixtures.
- `scripts/`: build, install (`install.sh`, `install.ps1`), release, WinGet (`enhance-winget-manifest.sh`), file-length (`check-file-length.sh`), formal (`formal.py`, `formal_mutation.py`) and mutation (`mutation.sh`) scripts.
- `tools/mutation/`: mutation target manifests, accepted-survivor baselines and triage ledgers.
- `.goplint/`: goplint consumer configuration (guide: `docs/goplint/README.md`).
- `formal/`: Alloy and TLA+ models, `manifest.toml`, results in `README.md`.
- `website/`, `docs/architecture/`, `docs/diagrams/`: user docs and architecture diagrams.
- `openspec/`, `specs/`, `tasks/next/`: specifications, feature specs and items awaiting decision.

## Code area to skills

| Area | Skills |
|------|--------|
| `cmd/invowk/` | go, cli, go-hexagonal-ddd, d2-diagrams |
| `internal/app/commandsvc/`, `internal/app/execute/`, `internal/app/commandadapters/` | go, cli, go-hexagonal-ddd |
| `internal/app/deps/` | go, cli, go-hexagonal-ddd, formal-verification |
| `internal/app/module{cache,ops,sync}/` | go, go-hexagonal-ddd, module-security, formal-verification |
| `internal/modulelock/` | go, go-hexagonal-ddd, windows-testing, formal-verification |
| `internal/container/`, `internal/containerplan/`, `internal/provision/` | go, container, go-hexagonal-ddd, linux-testing |
| `internal/discovery/` | go, discovery, go-hexagonal-ddd, d2-diagrams |
| `internal/runtime/` | go, shell (virtual-sh), virtual-lua, go-hexagonal-ddd, d2-diagrams, go-testing |
| `internal/config/`, `pkg/cueutil/` | go, cue |
| `internal/basedir/` | go, windows-testing, go-testing |
| `internal/sshserver/`, `internal/tuiserver/`, `internal/core/serverbase/` | go, server |
| `internal/tui/` | go, testing, tui-testing, tmux-testing, windows-testing |
| `internal/uroot/` | go, uroot |
| `internal/watch/` | go, macos-testing, linux-testing |
| `internal/audit/` | go, module-security, go-hexagonal-ddd |
| `internal/issue/` | go |
| `internal/benchmark/` | go, bencher |
| `pkg/invowkfile/`, `pkg/invowkmod/` | go, cue, invowk-schema, go-hexagonal-ddd (+ formal-verification for bound code) |
| `pkg/platform/` | go, windows-testing |
| `pkg/types/` | go, invowk-typesystem, go-hexagonal-ddd |
| `tests/cli/` | go, testing, cli, invowk-schema, go-testing, native-mirror |
| `website/`, `docs/architecture/` | docs, review-docs, d2-diagrams |
| `formal/` | formal-verification |
| `openspec/` | openspec-* (user-invoked only) |

Code bound by formal correspondence tables is listed in `.agents/rules/formal-bindings.md`.
