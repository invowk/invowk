---
name: cli
description: CLI command structure, Cobra patterns, execution flow, hidden internal commands. Use when working on cmd/invowk/ files, adding CLI commands, or modifying Cobra command trees.
disable-model-invocation: false
---

# CLI Architecture Skill

This skill covers the CLI implementation in Invowk, including Cobra command structure, dynamic command registration, TUI component wrappers, and the execution flow.

Use this skill when working on:
- `cmd/invowk/` - CLI commands and structure
- Adding new CLI commands or subcommands
- Modifying command output format
- TUI component integration
- Error handling and exit codes

---

## Normative Quick Rules

- `.agents/rules/commands.md` is the source of truth for hidden internal command policy.
- `.agents/rules/testing.md` is the source of truth for CLI test coverage and mirror expectations.
- `.agents/rules/go-patterns.md` is the source of truth for Go comments, ordering, and error-handling conventions.
- This skill documents CLI architecture and implementation details; if this file conflicts with a rule, the rule wins.

---

## Command Hierarchy Structure

The CLI is organized under `root.go` with these main command groups:

| Command | Description |
|---------|-------------|
| `invowk cmd` | Dynamic command execution (discovered from invowkfiles/modules) |
| `invowk module` | Module management (validate, create, alias, deps) |
| `invowk validate` | Unified validation (workspace, invowkfile, or module) |
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

**Rules:**
- Internal commands are for inter-process communication and subprocess execution
- Do NOT document in website docs
- Only mention in `.agents/` and `README.md`
- Used by container runtime, SSH server, TUI server internals

**Current internal commands:**
- `invowk internal exec-virtual` — Runs virtual shell in subprocess context
- `invowk internal check-cmd <name>` — Returns exit 0 if command is discoverable, exit 1 otherwise. Used by runtime-level `cmds` dependency validation inside containers to verify auto-provisioning worked.

---

## Dynamic Command Registration

The discovery → registration flow (`cmd_discovery.go`):

```
Discovery → Validation → Command Registration
                ↓
        DiscoveredCommandSet
         ├── Commands: all discovered
         ├── AmbiguousNames: conflicts
         └── SourceOrder: sorted sources
```

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

All TUI components follow a **dual-layer delegation pattern** (`tui_*.go`):

```go
func runTuiInput(cmd *cobra.Command, args []string) error {
    // Layer 1: Check if running under parent TUI server
    if client := tuiserver.NewClientFromEnv(); client != nil {
        // Delegate to parent TUI server via HTTP/IPC
        result, err := client.Input(tuiserver.InputRequest{...})
        return handleResult(result)
    }

    // Layer 2: Direct TUI rendering
    result, err := tui.Input(tui.InputOptions{...})
    return handleResult(result)
}
```

### Available TUI Commands

| Command | Purpose |
|---------|---------|
| `tui input` | Single-line text input |
| `tui choose` | Single/multi-select from list |
| `tui confirm` | Yes/no confirmation |
| `tui spin` | Spinner with command execution |
| `tui filter` | Fuzzy filter |
| `tui file` | File picker |
| `tui table` | Display/select from table |
| `tui pager` | Scrollable content viewer |
| `tui format` | Markdown/code/emoji formatting |
| `tui write` | Multi-line text editor |

**Benefits:**
- Commands work both standalone and nested in TUI server
- Output to stdout for piping/variable assignment
- Consistent behavior across execution contexts

---

## Discovery → Runtime → Execution Flow

The execution is decomposed into a pipeline of focused methods on `commandService` (`cmd_execute.go`), with runtime selection and context construction delegated to `internal/app/execute/`:

```
commandService.Execute(ctx, req)
    │
    ├── discoverCommand()       ← Loads config, routes through DiscoveryService (uses per-request cache)
    │   └── s.discovery.GetCommand(ctx, name)
    │
    ├── resolveDefinitions()    ← Resolves flag/arg defs with fallbacks
    │
    ├── validateInputs()        ← Validates flags, args, platform compatibility
    │
    ├── resolveRuntime()        ← Delegates to appexec.ResolveRuntime() (3-tier precedence),
    │                              wraps errors as ServiceError
    │
    ├── ensureSSHIfNeeded()     ← Conditional SSH server start for container host access
    │
    ├── buildExecContext()      ← Delegates to appexec.BuildExecutionContext() for env var projection
    │                              (includes INVOWK_CMD_NAME, INVOWK_RUNTIME, INVOWK_SOURCE, INVOWK_PLATFORM)
    │
    ├── [DRY-RUN SHORT-CIRCUIT] ← If req.DryRun: renderDryRun() and return (no execution)
    │
    └── dispatchExecution()     ← Calls runtime.BuildRegistry(), then executes pipeline:
        ├── Container init fail-fast (via runtimeRegistryResult.ContainerInitErr)
        ├── Timeout validation (parse-only, fail-fast on invalid strings)
        ├── Timeout wrapping (context.WithTimeout)
        ├── Dependency validation (validateAndRenderDeps)
        ├── Interactive mode (alternate screen + TUI server) OR standard execution
        └── Error classification via classifyExecutionError()
```

### Error Classification Pipeline

`classifyExecutionError()` (`cmd_execute_error_classifier.go`) maps runtime errors to issue catalog IDs using type-safe `errors.Is()` chains:

```go
switch {
case errors.Is(err, container.ErrNoEngineAvailable):  → ContainerEngineNotFoundId
case errors.Is(err, runtime.ErrRuntimeNotAvailable):  → RuntimeNotAvailableId
case errors.Is(err, os.ErrPermission):                → PermissionDeniedId
default (ActionableError "find shell"):               → ShellNotFoundId
fallback:                                             → ScriptExecutionFailedId
}
```

### ServiceError & renderServiceError()

`ServiceError` (`service_error.go`) carries optional rendering info for the CLI layer. The extracted `renderServiceError()` DRYs the identical rendering pattern used in both `executeRequest()` and `runDisambiguatedCommand()`:

```go
renderServiceError(stderr, svcErr)
  → prints styled message (if present)
  → renders issue catalog help section (if IssueID set)
```

---

## Disambiguation Pattern

Two methods for specifying command source:

### @source Prefix (First Argument)

```bash
invowk cmd @foo deploy      # Run deploy from foo.invowkmod
invowk cmd @invowkfile build  # Run build from invowkfile.cue
```

### --ivk-from Flag

```bash
invowk cmd --ivk-from foo deploy
```

### Source Name Normalization

`normalizeSourceName()` handles various formats:
- `"foo"` → `"foo"`
- `"foo.invowkmod"` → `"foo"`
- `"invowkfile"` → `"invowkfile"`
- `"invowkfile.cue"` → `"invowkfile"`

---

## Error Handling & Exit Codes

### ExitError Type

```go
type ExitError struct {
    Code int
    Err  error
}
```

**Pattern:**
- Command `RunE` returns `ExitError` for non-zero exit codes
- Root `Execute()` catches `ExitError` and calls `os.Exit(exitErr.Code)`
- Prevents cascading error messages while maintaining proper exit codes

### Dual-Channel Error Output

CLI handlers (e.g., `runModuleRemove`, `runModuleSync`) use two output channels simultaneously:

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

---

## Styling System

Unified color palette (`styles.go`):

| Color | Hex | Use |
|-------|-----|-----|
| ColorPrimary | `#7C3AED` | Titles (purple) |
| ColorMuted | `#6B7280` | Subtitles (gray) |
| ColorSuccess | `#10B981` | Success (green) |
| ColorError | `#EF4444` | Errors (red) |
| ColorWarning | `#F59E0B` | Warnings (amber) |
| ColorHighlight | `#3B82F6` | Commands/links (blue) |
| ColorVerbose | `#9CA3AF` | Verbose output (light gray) |

**Reusable Styles:**
- `TitleStyle`, `SubtitleStyle`, `SuccessStyle`, `ErrorStyle`
- `WarningStyle`, `CmdStyle`, `VerboseStyle`

---

## Global Flags

### Root Command (Persistent)

```go
--ivk-verbose, -v     // Enable verbose output
--ivk-config          // Custom config file path
--ivk-interactive, -i // Run in alternate screen buffer
```

### Cmd Command

```go
--ivk-runtime, -r     // Override runtime (must be allowed)
--ivk-from            // Specify source for disambiguation
--ivk-force-rebuild   // Force container image rebuild
--ivk-dry-run         // Print resolved execution context without executing
--ivk-watch, -W       // Watch files for changes and re-execute
```

---

## Configuration Loading

Flow from `root.go`:

```go
Execute()
    ↓
cobra.OnInitialize(initRootConfig)
    ├── Apply --ivk-config flag override
    ├── Load config via config.Load()
    ├── Surface errors as warnings (non-fatal)
    ├── Apply verbose/interactive from config if not set via flags
    └── Store in GetVerbose(), GetInteractive() accessors
```

**Priority:** CLI flags > config file > defaults

---

## Module Commands

Module management (`module.go`):

| Command | Purpose |
|---------|---------|
| `module create` | Create new module |
| `module list` | List discovered modules |
| `module archive` | Package module as archive |
| `module import` | Import external module |
| `module vendor` | Vendor module dependencies |

### Dependency Management

```bash
module add <git-url> <version>  # Add dependency
module remove <identifier>       # Remove dependency
module sync                      # Sync dependencies from invowkmod.cue (accepts 0 args)
module update [identifier]       # Update all deps or one matching dependency
module deps                      # List dependencies from lock file (accepts 0 args)
```

---

## Discovery Integration Points

```go
disc := discovery.New(cfg)

// Discover and validate all commands (with ambiguity detection + diagnostics)
result, err := disc.DiscoverAndValidateCommandSet(ctx)
// result.Set is the DiscoveredCommandSet, result.Diagnostics are non-fatal warnings

// Get specific command info (for execution)
lookup, err := disc.GetCommand(ctx, cmdName)
// lookup.Command is the CommandInfo, lookup.Diagnostics are accumulated warnings

// Discover command set without tree validation (for listing)
result, err := disc.DiscoverCommandSet(ctx)

// List all discovered files (for module operations)
files, err := disc.DiscoverAll()  // or disc.LoadAll() to also parse
```

---

## Design Principles

| Principle | Implementation |
|-----------|----------------|
| Transparent namespace | Users don't specify source for unambiguous commands |
| Explicit disambiguation | Require @source or --ivk-from for ambiguous commands |
| Platform-aware execution | Different runtimes, validation per platform |
| Dual-layer TUI | Support standalone and server-delegated rendering |
| Styled consistency | Unified color palette across all output |
| Hidden internals | Internal commands not documented to users |
| Configuration priority | CLI flags > config file > defaults |

---

## File Organization

| File | Purpose |
|------|---------|
| `root.go` | Root command, global flags, config loading |
| `cmd.go` | `invowk cmd` parent, executeRequest(), disambiguation entry point |
| `cmd_discovery.go` | Dynamic command registration (registerDiscoveredCommands, buildLeafCommand) |
| `cmd_execute.go` | commandService: decomposed pipeline (discoverCommand → resolveDefinitions → validateInputs → resolveRuntime → ensureSSHIfNeeded → buildExecContext → [dry-run] → dispatchExecution). `resolveRuntime` and `buildExecContext` delegate to `internal/app/execute/` |
| `cmd_execute_helpers.go` | runtimeRegistryResult, createRuntimeRegistry() (delegates to `runtime.BuildRegistry()`), runDisambiguatedCommand(), checkAmbiguousCommand() |
| `cmd_dryrun.go` | `renderDryRun()` — prints resolved execution context (script, env, runtime, platform, timeout) without executing |
| `cmd_watch.go` | `runWatchMode()` — discovers command's WatchConfig, executes once, starts `internal/watch/` watcher loop with re-execution callback |
| `internal/app/execute/orchestrator.go` | RuntimeSelection, RuntimeNotAllowedError, ResolveRuntime() (3-tier precedence), BuildExecutionContext() (env var projection including INVOWK_CMD_NAME, INVOWK_RUNTIME, INVOWK_SOURCE, INVOWK_PLATFORM) |
| `internal/discovery/validation.go` | ValidateCommandTree() for args/subcommand conflicts |
| `internal/watch/watcher.go` | File watcher with fsnotify + doublestar glob matching, debouncing, default ignores |
| `internal/runtime/registry_factory.go` | BuildRegistry(), BuildRegistryOptions, RegistryBuildResult, InitDiagnostic |
| `cmd_execute_error_classifier.go` | classifyExecutionError() — maps runtime errors to issue catalog IDs |
| `service_error.go` | ServiceError type and renderServiceError() |
| `cmd_render.go` | Styled error rendering (argument validation, deps, runtime, host support) |
| `validate.go` | Unified `invowk validate` command (workspace, invowkfile, module auto-detection) |
| `module.go` | Module commands |
| `tui_*.go` | TUI component wrappers |
| `styles.go` | Unified styling system |
| `internal_*.go` | Hidden internal commands |

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
| New flag missing from one ExecuteRequest site | Flag silently ignored for some paths | Wire in ALL 3 sites: `runCommand`, `buildLeafCommand`, `runDisambiguatedCommand` |
