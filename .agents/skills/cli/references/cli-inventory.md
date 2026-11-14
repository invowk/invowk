# CLI Inventory

Lookup tables for `cmd/invowk/` and the execution packages it drives. They are
derivable from source and drift over time: re-derive with the commands in
`SKILL.md` (or the one under each table) before relying on a row.

## Flags

Derive: `go run . --help`, `go run . cmd --help`, and
`rg -n 'Flags\(\)\.' cmd/invowk/root.go cmd/invowk/cmd.go cmd/invowk/cmd_discovery.go`.

### Root persistent flags (`root.go`, stored in `rootFlagValues`)

```text
--ivk-verbose, -v      enable verbose output
--ivk-config, -c       custom config file path (overrides INVOWK_CONFIG)
--ivk-interactive, -i  run in the alternate screen buffer (interactive mode)
--ivk-no-cwd           skip the cwd invowkfile and sibling modules (env: INVOWK_NO_CWD)
```

### `cmd` persistent flags (`cmd.go`, stored in `cmdFlagValues`)

```text
--ivk-runtime, -r      override the runtime (must be allowed by the command)
--ivk-from, -f         source for disambiguation (wins over an @source prefix)
--ivk-force-rebuild    force container image rebuild (container runtime only)
--ivk-container-name   override the persistent container target name (container runtime only)
--ivk-dry-run          print the resolved execution plan without executing
--ivk-watch, -W        watch files for changes and re-execute
```

### Per-leaf reserved flags (`buildLeafCommand` in `cmd_discovery.go`)

Injected on every discovered leaf command, not stored in `cmdFlagValues`;
`buildCommandExecuteRequest()` reads them with `stringFlagValue` /
`stringArrayFlagValue`.

```text
--ivk-env-file, -e         load env vars from file(s) (repeatable)
--ivk-env-var, -E          set env var KEY=VALUE (repeatable)
--ivk-env-inherit-mode     host env inheritance: none, allow, all (overrides runtime config)
--ivk-env-inherit-allow    allowlist for host env inheritance (repeatable)
--ivk-env-inherit-deny     denylist for host env inheritance (repeatable)
--ivk-workdir, -w          override the working directory for this command
```

User-defined command flags from the invowkfile are registered on the same leaf
after these.

## TUI Commands

Derive: `rg -n 'func newTUI.*Command|Use:|Flags\(\)' cmd/invowk/tui*.go`.
Parent: `tui.go`; one `tui_<name>.go` per command.

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
| `tui style` | Apply terminal text styling |
| `tui write` | Multi-line text editor |

## Module Commands

Derive: `go run . module --help` and `rg -n 'Use:' cmd/invowk/module*.go`.
Subcommands are spread across `module.go` (parent, `list`),
`module_create.go`, `module_package.go`, `module_deps.go`, and
`module_tidy.go`.

| Command | File | Purpose |
|---------|------|---------|
| `module create <name>` | `module_create.go` | Create a new module |
| `module list` | `module.go` | List discovered modules |
| `module archive <path>` | `module_package.go` | Create a ZIP archive from a module |
| `module import <source>` | `module_package.go` | Import a module from a ZIP file or URL |
| `module vendor [module-path]` | `module_package.go` | Vendor module dependencies |
| `module add <git-url> <version>` | `module_deps.go` | Add a dependency |
| `module remove <identifier>` | `module_deps.go` | Remove a dependency |
| `module sync` | `module_deps.go` | Sync dependencies from `invowkmod.cue` (accepts 0 args) |
| `module update [identifier]` | `module_deps.go` | Update all deps or one matching dependency |
| `module deps` | `module_deps.go` | List dependencies from the lock file (accepts 0 args) |
| `module tidy` | `module_tidy.go` | Add missing transitive requirements to `invowkmod.cue` |

## Styling Palette (`styles.go`)

| Color | Hex | Use |
|-------|-----|-----|
| `ColorPrimary` | `#7C3AED` | Titles (purple) |
| `ColorMuted` | `#6B7280` | Subtitles (gray) |
| `ColorSuccess` | `#10B981` | Success (green) |
| `ColorError` | `#EF4444` | Errors (red) |
| `ColorWarning` | `#F59E0B` | Warnings (amber) |
| `ColorHighlight` | `#3B82F6` | Commands/links (blue) |
| `ColorVerbose` | `#9CA3AF` | Verbose output (light gray) |

Reusable exported styles: `TitleStyle`, `SubtitleStyle`, `SuccessStyle`,
`ErrorStyle`, `WarningStyle`, `CmdStyle`, `VerboseStyle`,
`VerboseHighlightStyle`. Unexported `render*Style` values serve
`cmd_render.go`.

## File Organization

Paths are under `cmd/invowk/` unless stated.

| File | Purpose |
|------|---------|
| `root.go` | Root command, persistent flags, `Execute()` (maps `ExitError` to `os.Exit`) |
| `app.go` | `App` wiring, `cliCommandAdapter` (renders dry-run results), `renderAndWrapServiceError()`, `issueIDForServiceErrorKind()` |
| `discovery_scope.go` | `--ivk-no-cwd` / `INVOWK_NO_CWD`: `resolveNoCwd()`, `explicitNoCwdFromArgs()`, `noCwdForRegistration()`, `scopeDiscovery()` / `contextWithDiscoveryScope()` (switch + config path + request cache), `renderExplicitConfigError()`, agent `--verify` rejection |
| `exit_error.go` | `ExitError{Code types.ExitCode, Err error}` |
| `cmd.go` | `invowk cmd` parent and its persistent flags, `ParseSourceFilter()`, `normalizeSourceName()`, `shouldRegisterDiscoveredCommands()`, `explicitConfigPathFromArgs()`, `executeRequest()`, `resolveUIFlags()` |
| `cmd_discovery.go` | Dynamic command registration (`registerDiscoveredCommands`, `buildLeafCommand` with per-leaf reserved flags), completion, `listCommands` |
| `cmd_execute_helpers.go` | `buildCommandExecuteRequest()`, `runDisambiguatedCommand()` (delegates to `executeRequest()`), `renderServiceErrorIfPresent()`, `checkAmbiguousCommand()`, `parseEnvVarFlags()`, `toEnvVarNames()`, `toDotenvFilePaths()` |
| `cmd_dryrun.go` | `renderDryRun()`: prints the service's `DryRunPlan` (script, env, runtime, platform, timeout) |
| `cmd_watch.go` | `runWatchMode()`: resolves the command's watch plan, executes once, starts the `internal/watch/` loop with a re-execution callback |
| `service_error.go` | `ServiceError` type and `renderServiceError()` |
| `cmd_render.go` | Styled error rendering (argument validation, deps, runtime, host support, ambiguity) |
| `validate.go` | Unified `invowk validate` (workspace, invowkfile, module auto-detection) |
| `module*.go` | Module commands (see Module Commands) |
| `tui*.go` | TUI component wrappers |
| `styles.go` | Unified styling system |
| `internal*.go` | Hidden internal commands (`check-cmd`, `exec-virtual-sh`, `exec-virtual-lua`) |
| `internal/app/commandsvc/service.go` | `(*Service).Execute` pipeline, dry-run short-circuit, host-access lifecycle |
| `internal/app/commandsvc/dispatch.go` | `dispatchExecution()`, `failFastContainerInit()`, `applyExecutionTimeout()`, `executeWithRequestedMode()`, `newClassifiedExecutionError()` |
| `internal/app/commandsvc/errors.go` | `classifyExecutionError()`: maps runtime errors to `commandsvc.ErrorKind` (the CLI maps kinds to issue IDs) |
| `internal/app/execute/orchestrator.go` | `RuntimeSelection`, `RuntimeNotAllowedError`, `ResolveRuntime()` (3-tier precedence), `BuildExecutionContext()` (env projection incl. `INVOWK_CMD_NAME`, `INVOWK_RUNTIME`, `INVOWK_SOURCE`, `INVOWK_PLATFORM`) |
| `internal/app/commandadapters/runtime_registry.go` | `NewRuntimeRegistryFactory()`: builds the runtime session/registry used by `dispatchExecution()` |
| `internal/app/commandadapters/discovery.go` | `DiscoveryService` (CLI-facing discovery with per-request cache) |
| `internal/runtime/registry_factory.go` | `HostCallback*` types and `InitDiagnostic` (runtime init diagnostics) |
| `internal/discovery/validation.go` | `ValidateCommandTree()` for args/subcommand conflicts |
| `internal/watch/watcher.go` | File watcher with fsnotify + doublestar glob matching, debouncing, default ignores |
