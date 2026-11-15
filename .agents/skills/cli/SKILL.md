---
name: cli
description: >-
  CLI command structure, Cobra patterns, execution flow, hidden internal
  commands, command service adapters, and CLI-facing dependency/runtime
  validation. Use when working on cmd/invowk/, internal/app/commandsvc/,
  internal/app/deps/, internal/app/execute/, or internal/app/commandadapters/,
  adding commands or flags, modifying Cobra trees, or changing CLI error/output
  behavior.
---

# CLI Architecture Skill

This skill covers the CLI implementation in Invowk: Cobra command structure,
dynamic command registration, TUI component wrappers, and the execution flow.
Also use it for TUI component integration and for error handling and exit codes.

Lookup tables (full flag inventory, TUI and module command lists, styling
palette, file organization) live in `references/cli-inventory.md`.

---

## Normative Quick Rules

- `.agents/rules/commands.md` is the source of truth for hidden internal command policy.
- `.agents/rules/testing.md` is the source of truth for CLI test coverage and mirror expectations.
- `.agents/skills/go/SKILL.md` is the source of truth for Go comments, ordering, and error-handling conventions.
- On conflict with a rule, the rule wins.
- `internal/app/deps/deps.go` (including `CheckCommandDependenciesExist()`) is
  named by formal correspondence tables: renames or behavior changes there
  also follow `.agents/rules/formal-bindings.md`.

---

## Command Hierarchy Structure

The CLI is organized under `root.go`. Derive the current command tree before
editing or documenting it:

```bash
go run . --help
go run . module --help
go run . tui --help
rg -n 'func new.*Command|Use:|Hidden:' cmd/invowk
```

Current major groups are expected to include:

| Command | Description |
|---------|-------------|
| `invowk cmd` | Dynamic command execution (discovered from invowkfiles/modules) |
| `invowk module` | Module management; subcommands live in `cmd/invowk/module*.go` |
| `invowk validate` | Unified validation (workspace, invowkfile, or module) |
| `invowk audit` | Module supply-chain/security audit |
| `invowk agent` | LLM-assisted invowk authoring helpers |
| `invowk config` | Configuration management |
| `invowk init` | Initialize new invowkfiles |
| `invowk tui` | Interactive terminal UI components (gum-like) |
| `invowk internal` | **Hidden** internal commands |
| `invowk completion` | Shell completion |

---

## Hidden Internal Commands Policy

**CRITICAL: All `invowk internal *` commands MUST remain hidden.**

```go
&cobra.Command{
    Use:    "internal",
    Hidden: true,  // ALWAYS true for internal commands
}
```

- Internal commands are for inter-process communication and subprocess
  execution, used by container runtime, SSH server, and TUI server internals.
- Never document them in website docs; mention them only in `README.md` and
  `.agents/` (canonical: `.agents/rules/commands.md`).
- Current internal commands: `check-cmd`, `exec-virtual-sh`, `exec-virtual-lua`.
  Re-derive from `cmd/invowk/internal*.go` before making changes:

```bash
rg -n 'newInternal|Use:|Hidden: true' cmd/invowk/internal*.go
```

Keep all virtual runtime subprocess entrypoints and container/runtime
dependency probes under hidden `invowk internal` command paths.

---

## Dynamic Command Registration

The discovery → registration flow (`cmd_discovery.go`):

```
Discovery → Validation → Command Registration (registerDiscoveredCommands → buildLeafCommand)
```

Registration consumes the `DiscoveredCommandSet` (all commands plus the
by-name, by-simple-name, by-source, ambiguity, and source-order indexes). The
discovery skill owns the full field table:
`.agents/skills/discovery/SKILL.md` § "Phase 2: Conflict Analysis".

### Startup Gate (Important)

Dynamic registration must stay **lazy** for startup-sensitive paths.

- `shouldRegisterDiscoveredCommands()` (`cmd.go`) registers discovered commands
  only when argv targets the `cmd` subtree, or a `__complete` /
  `__completeNoDesc` request for it.
- Do **not** trigger discovery registration for unrelated root paths like
  `--version` or `--help`; unconditional registration in root init regresses
  their startup toward discovery cost.
- `explicitConfigPathFromArgs()` pulls `--ivk-config` / `-c` from raw argv
  before Cobra parses flags, so registration uses the same config as execution.
  `explicitNoCwdFromArgs()` / `noCwdForRegistration()` do the same for
  `--ivk-no-cwd` (stop at `--`, env fallback); any error skips registration.
- The raw-argv scan and pflag can disagree: a value-taking flag consumes a
  `--` (`--msg -- --ivk-no-cwd`), so pflag parses a flag the scan skipped.
  Never trust registration-time state captured in a leaf closure for a
  security decision; re-check it in `RunE` (see `buildLeafCommand`'s
  `registeredNoCwd` and `rescopeRegisteredCommand`).

### Transparent Namespace

Unambiguous commands are registered under their `SimpleName`:

```bash
# Only one source defines "build" → user can run directly
invowk cmd build

# Multiple sources define "deploy" → requires disambiguation
invowk cmd @foo deploy      # Using @source prefix
invowk cmd --ivk-from foo deploy  # Using --ivk-from flag
```

### Hierarchical Tree Building

Multi-word commands (e.g., "deploy staging") are built into a command tree with automatically created parent commands.

---

## TUI Component Wrapper Pattern

All TUI components follow a **dual-layer delegation pattern** (`tui_*.go`,
e.g. `tui_input.go`):

```go
// Layer 1: running under a parent TUI server? Delegate over the wire.
if client := tuiclient.NewClientFromEnv(); client != nil {
    result, err = client.InputContext(cmd.Context(), tuiwire.InputRequest{...})
} else {
    // Layer 2: direct TUI rendering.
    result, err = tui.Input(tui.InputOptions{...})
}
```

Commands therefore work both standalone and nested in a TUI server, and write
results to stdout for piping/variable assignment. Rebuild the TUI inventory
(table in `references/cli-inventory.md`) before changing TUI commands:

```bash
rg -n 'func newTUI.*Command|Use:|Flags\(\)' cmd/invowk/tui*.go
```

---

## Discovery → Runtime → Execution Flow

The execution path spans the Cobra adapter, command service, command adapters,
and runtime packages. Re-read these files before editing flow-sensitive code:

- `cmd/invowk/cmd*.go` for Cobra flag parsing, dynamic command registration,
  disambiguation, watch mode, and service-error rendering.
- `internal/app/commandsvc/service.go`, `dispatch.go`, `inputs.go`, `errors.go`,
  and `watch.go` for service orchestration and error classification.
- `internal/app/commandadapters/` for dependency probes, runtime registry
  construction, discovery adapters, and interactive subprocess/session bridges.
- `internal/app/execute/` for runtime selection and execution-context
  construction.

`(*commandsvc.Service).Execute(ctx, req)` currently runs in this order:

```
Execute(ctx, req)
    │
    ├── req.Validate()          ← Typed-field validation before any work
    │
    ├── discoverCommand()       ← Uses req.ResolvedCommand when provided by CLI/disambiguation path;
    │                              otherwise loads config and calls s.discovery.GetCommand(ctx, name)
    │
    ├── resolveDefinitions()    ← Resolves flag/arg defs with fallbacks
    │
    ├── validateInputs()        ← Validates flags, args, platform compatibility
    │
    ├── resolveRuntime()        ← Delegates to appexec.ResolveRuntime() (3-tier precedence),
    │                              wraps errors as ServiceError
    │
    ├── buildExecContext()      ← Delegates to appexec.BuildExecutionContext() for env var projection
    │                              (includes INVOWK_CMD_NAME, INVOWK_RUNTIME, INVOWK_SOURCE, INVOWK_PLATFORM)
    │
    ├── [DRY-RUN SHORT-CIRCUIT] ← If req.DryRun: return Result.DryRunData (the CLI adapter calls
    │                              renderDryRun()). Runs before dependency validation and before host
    │                              or runtime setup, so planning never starts infrastructure.
    │
    ├── ensureSSHIfNeeded()     ← Conditional host-access (SSH) start for container callbacks; only the
    │                              Execute call that started it stops it (deferred)
    │
    └── dispatchExecution()     ← registryFactory.Create(...) runtime session, then:
        ├── failFastContainerInit(session.ContainerInitErr())
        ├── applyExecutionTimeout() ← Parse-only validation (fail-fast on invalid strings),
        │                              then context.WithTimeout
        ├── validateDeps()      ← Returns raw typed errors (e.g. *DependencyError); the CLI renders them
        ├── executeWithRequestedMode() ← Interactive mode (alternate screen + TUI server) OR standard
        │                              execution
        └── newClassifiedExecutionError() → classifyExecutionError() on runtime failure
```

### Error Classification Pipeline

Execution error handling is two-stage: `classifyExecutionError()`
(`internal/app/commandsvc/errors.go`) maps runtime errors to service-owned
`ErrorKind` values, then the CLI layer maps those kinds to issue catalog IDs in
`cmd/invowk/app.go`.

```go
classifyExecutionError(err) → (commandsvc.ErrorKind, hint)
issueIDForServiceErrorKind(kind commandsvc.ErrorKind) → issue.Id
```

### ServiceError & renderServiceError()

`ServiceError` (`service_error.go`) carries optional rendering info for the CLI
layer; `renderAndWrapServiceError()` (`app.go`) builds it from raw service
errors. `renderServiceError()` is the single rendering path, called from
`executeRequest()` (`cmd.go`), `renderServiceErrorIfPresent()` (used by
`runDisambiguatedCommand()` in `cmd_execute_helpers.go`), and watch mode
(`cmd_watch.go`):

```go
renderServiceError(stderr, svcErr)
  → prints styled message (if present)
  → renders issue catalog help section (if IssueID set)
```

---

## Disambiguation Pattern

Two methods for specifying command source, parsed by `ParseSourceFilter()`
(`cmd.go`):

- `--ivk-from` / `-f` takes precedence over an `@source` prefix.
- `@source` is recognized only as the first positional token, so command
  arguments that start with `@` are not mistaken for sources.

```bash
invowk cmd @foo deploy        # Run deploy from foo.invowkmod
invowk cmd @invowkfile build  # Run build from invowkfile.cue
invowk cmd --ivk-from foo deploy
```

### Source Name Normalization

`normalizeSourceName()` strips a leading `@`, then maps:
- `"foo"` → `"foo"`
- `"foo.invowkmod"` → `"foo"`
- `"invowkfile"` → `"invowkfile"`
- `"invowkfile.cue"` → `"invowkfile"`

---

## Error Handling & Exit Codes

### ExitError Type

```go
type ExitError struct {
    Code types.ExitCode
    Err  error
}
```

**Pattern:**
- Command `RunE` returns `ExitError` for an explicit exit code (a forwarded user-command status, or a command such as `audit`/`agent` that already rendered its message)
- Root `Execute()` passes the error from `fang.Execute` to `exitCodeForError` and calls `os.Exit` with the result
- Prevents cascading error messages while maintaining proper exit codes

### Exit-Code Contract and `UsageError`

The contract (documented in `website/docs/reference/cli.mdx` "Exit Codes" and the README) is `0` success, `N` a user command's own status, `2` a rejected command line, `1` any other invowk failure. Per-command exceptions: `audit` (0/1/2), `tui confirm`, `tui spin`, `validate`.

`cmd/invowk/usage_error.go` owns the mechanism:
- `UsageError{Err}` marks a command-line rejection. `Error()` returns the wrapped text unchanged, so messages and fang's "Try --help" hint stay the same; `Unwrap()` keeps `errors.Is/As` working (for example `errors.AsType[*ServiceError]`).
- `newUsageError(err)` returns nil for nil and returns `err` unchanged when it already carries an `*ExitError`, so an explicit code always wins. Never build `&UsageError{}` directly.
- `exitCodeForError(err)` is the pure mapping: nil → 0, `*ExitError` → its `Code`, `*UsageError` → `ExitCodeUsage` (2), anything else → 1.
- `installUsageGuards(root)` runs once at the end of `NewRootCommand`: it sets a `FlagErrorFunc` (pflag errors → usage), a root `Args` validator with Cobra's unknown-command message and suggestions, a group guard (`rejectUnmatchedGroupToken`) on every parent with `Args == nil` (plus `RunE: help` when it is not runnable), and wraps every explicit `Args` validator.
- `rejectUnmatchedGroupToken` writes the group's visible subcommands to stderr (nothing to stdout) and returns the unknown-command `UsageError`. A bare group still prints help and exits 0.

Rules:
- **A parent command that accepts positionals MUST declare `Args` explicitly** (for example `cobra.ArbitraryArgs` on `invowk cmd` and on discovered group nodes); otherwise the group guard rejects its tokens before `RunE` runs.
- Classify at the CLI adapter, not in domain packages: `renderAndWrapServiceError` wraps argument/flag validation, ambiguous or unknown command, unknown source and runtime-not-allowed errors; `buildCommandExecuteRequest` wraps invalid reserved `--ivk-*` values.
- A refusal decided from argv and the environment alone, before any file is read, is also usage: an invalid `INVOWK_NO_CWD` (`noCwdFromEnv`), `--ivk-watch` with `--ivk-dry-run`, `config set` under an explicit config source, `agent … --verify` under `--ivk-no-cwd`.
- Environment and internal failures (dependencies, platform, engine, timeouts, `InvalidRequestError`, config files, including a failed `--ivk-config`/`INVOWK_CONFIG` source) stay at 1.
- `agent` and `audit` keep explicit `ExitError` codes. The shared LLM helpers in `llm_flags.go` take an `llmExitCodes{usage, failure}` value (`auditLLMExitCodes`, `agentLLMExitCodes`).
- In testscript, assert exact codes with the custom `exitcode <want> <cmd> <args...>` command (`tests/cli/exitcode_test.go`); `! exec` accepts any non-zero status.

### Dual-Channel Error Output

`.agents/rules/testing.md` requires CLI handler error tests to assert both
channels; this is why. CLI handlers (e.g., `runModuleRemove`, `runModuleSync`)
use two output channels simultaneously:

1. **stdout** — Handler prints styled progress with icons via `fmt.Printf` (e.g., `"• Removing dep..."`, `"✗ Failed to remove module: ..."`)
2. **stderr** — Handler returns the error to Cobra, which renders it with styled `ERROR` formatting

This means a handler error produces output on BOTH channels. When writing txtar tests for error paths, assert both:
```
! exec invowk module remove dep
stdout 'Failed to remove'        # Handler's styled output (stdout)
stderr '[Nn]o module found'      # Cobra's error rendering (stderr)
```

Testing only one channel misses bugs in the other (e.g., handler formatting breaks but Cobra rendering works, or vice versa).

### Styled Error Rendering

`cmd_render.go` provides styled error messages for:
- Argument validation failures
- Dependency errors
- Runtime mismatches
- Ambiguous commands
- Host platform compatibility issues

### Styling

Use the palette and styles from `styles.go` for all CLI output; never ad-hoc
colors. Palette and style list: `references/cli-inventory.md`.

---

## Flags

Derive the current set with `go run . --help` and `go run . cmd --help`; the
full inventory is in `references/cli-inventory.md`. Three kinds exist, each
registered and read in a different place:

- **Root persistent** (`--ivk-verbose -v`, `--ivk-config -c`,
  `--ivk-interactive -i`, `--ivk-no-cwd`): `root.go`, stored in `rootFlagValues`.
- **`cmd` persistent** (`--ivk-runtime -r`, `--ivk-from -f`,
  `--ivk-force-rebuild`, `--ivk-container-name`, `--ivk-dry-run`,
  `--ivk-watch -W`): `cmd.go`, stored in `cmdFlagValues`.
- **Per-leaf reserved** (`--ivk-env-file -e`, `--ivk-env-var -E`,
  `--ivk-env-inherit-{mode,allow,deny}`, `--ivk-workdir -w`): registered on
  every discovered leaf by `buildLeafCommand()` (`cmd_discovery.go`) and read in
  `buildCommandExecuteRequest()` via `stringFlagValue` / `stringArrayFlagValue`.

---

## Configuration Loading

Configuration is request-scoped rather than initialized into global accessors:

1. `--ivk-config` and `--ivk-no-cwd` are stored in `rootFlagValues`. Every
   command-source discovery entry calls `scopeDiscovery()` (`resolveNoCwd()`
   plus `contextWithDiscoveryScope()`, `discovery_scope.go`), which attaches the
   switch, the config path and the request cache. Do not attach the config path
   without the switch.
2. Discovery and command execution load through the injected config provider;
   the request context also owns the per-request discovery/config cache. The
   provider applies `--ivk-config` > `INVOWK_CONFIG` > user file > defaults
   (`config.SelectSource`); the cwd is never a config source. Env reads go
   through `App.Getenv` / `config.NewProviderWithEnv`, never `os.Getenv` in
   handlers. A failed explicit source (`config.ErrExplicitConfig`) is fatal and
   cached per request: render it with `renderExplicitConfigError()` and return
   it as a plain error (exit 1); only user-file failures fall back to defaults.
3. `buildCommandExecuteRequest()` carries the explicit config path and whether
   UI flags were explicitly set into the service request.
4. `resolveUIFlags()` applies `ui.verbose` and `ui.interactive` only when the
   corresponding persistent CLI flag was not explicitly set. Config-load
   failures are rendered as warnings and leave the CLI values in place.

**Priority:** explicitly set CLI flags > config values > defaults.

---

## Module Commands

Module management spans `module.go` (parent and `list`), `module_create.go`,
`module_package.go` (archive, import, vendor), `module_deps.go` (add, remove,
sync, update, deps), and `module_tidy.go`. Derive with `go run . module --help`;
table in `references/cli-inventory.md`.

---

## Discovery Integration Points

The CLI never calls `discovery.New` directly. It goes through
`commandadapters.DiscoveryService` (`internal/app/commandadapters/discovery.go`),
which loads config through the injected provider and caches per request:

```go
svc := commandadapters.NewDiscoveryService(provider)
// or NewDiscoveryServiceWithDirs(provider, baseDir, commandsDir) for injected dirs

cfg, diags, err := svc.LoadConfig(ctx)       // also LoadConfigForCommand(ctx); err = failed explicit source
result, err := svc.DiscoverAndValidateCommandSet(ctx) // result.Set + result.Diagnostics
result, err  = svc.DiscoverCommandSet(ctx)   // without tree validation (listing)
modules, err := svc.DiscoverModules(ctx)     // discovery.ModuleListResult
lookup, err  := svc.GetCommand(ctx, cmdName) // lookup.Command (nil if not found) + lookup.Diagnostics
```

`DiscoverAll()` and `LoadAll()` exist only on the raw `discovery.Discovery`
(used e.g. by `internal/audit/scan_context.go`); for that API see
`.agents/skills/discovery/SKILL.md` § "Usage Patterns".

---

## Common Pitfalls

| Pitfall | Symptom | Fix |
|---------|---------|-----|
| New built-in command without txtar test | `TestBuiltinCommandTxtarCoverage` fails | Add a `.txtar` test in `tests/cli/testdata/` with `exec invowk <command>` |
| Forgetting `Hidden: true` on internal cmd | Users see internal commands | Add `Hidden: true` to command |
| Hardcoded exit in RunE | Error message not shown | Return `ExitError` instead |
| Missing TUI server check | TUI components fail in nested context | Use dual-layer pattern |
| Not using styled output | Inconsistent CLI appearance | Use styles from `styles.go` |
| Wrong flag priority | Config overrides CLI flag | Check precedence logic |
| New execution flag omitted from request mapping | Flag silently ignored in some paths | For a `cmd` persistent flag: add it to `cmdFlagValues`, the Cobra registration in `cmd.go`, and `buildCommandExecuteRequest()`. For a per-leaf reserved flag: register it in `buildLeafCommand()` and read it in `buildCommandExecuteRequest()` with `stringFlagValue`/`stringArrayFlagValue`. Either way verify dynamic leaves, disambiguation, dry-run, and watch child requests |
| `context.Background()` in Cobra RunE handler | Ctrl+C / timeout not propagated | Extract `cmd.Context()` at closure boundary, pass as `ctx context.Context` first param to handler function |
| Unconditional discovered-command registration in root init | `invowk --version` / `--help` startup regresses toward discovery cost | Keep the `shouldRegisterDiscoveredCommands()` argv gate (see Startup Gate) |
