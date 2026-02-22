# Command Execution Sequence Diagram

This diagram shows the temporal flow from CLI invocation through discovery, runtime selection, and execution. Understanding this flow helps with debugging and extending Invowk.

## Main Execution Flow

![Main Execution Sequence](../diagrams/rendered/sequences/execution-main.svg)

## Container Runtime Flow (Detailed)

When the container runtime is selected, additional steps occur:

![Container Runtime Sequence](../diagrams/rendered/sequences/execution-container.svg)

## Virtual Runtime Flow

The virtual runtime uses the embedded mvdan/sh interpreter:

![Virtual Runtime Sequence](../diagrams/rendered/sequences/execution-virtual.svg)

## Phase Descriptions

### 1. Initialization Phase

| Step | Component | Action |
|------|-----------|--------|
| 1 | CLI | Receive user command |
| 2-4 | Config + CUE | Load and parse `~/.config/invowk/config.cue` |

**Key decisions made:**
- Container engine preference (docker vs podman)
- Includes for modules
- Default runtime if not specified

### 2. Discovery Phase

| Step | Component | Action |
|------|-----------|--------|
| 5 | Discovery | Start command discovery |
| 6-7 | CUE Parser | Parse discovered root/module `invowkfile.cue` files |
| 8-9 | Discovery | Scan one-level vendored modules (`invowk_modules/`) |
| 10 | Discovery | Build unified command tree |

**Precedence order (highest to lowest):**
1. Current directory `invowkfile.cue`
2. Current directory `*.invowkmod`
3. Configured includes (module paths from `config.Includes`)
4. User directory `~/.invowk/cmds/` (modules only, non-recursive)

Vendored modules are scanned one level deep per discovered module. Nested vendored modules are ignored with diagnostics.

### 3. Resolution Phase

| Step | Component | Action |
|------|-----------|--------|
| 11 | CLI | Match command name to discovered commands |
| 12 | CLI | Select platform-specific implementation |
| 13-14 | Registry | Get appropriate runtime instance |
| 15-16 | Runtime | Validate execution context |

**Platform matching:**
- Match implementations using declared platform objects (for example `platforms: [{name: "linux"}]`)
- If no implementation matches the current host platform, execution fails with a host-not-supported error

**Runtime resolution precedence:**
1. `--ivk-runtime` CLI override (hard validation)
2. Config `default_runtime` (used only when compatible)
3. Command default runtime for the matched implementation

### 4. Execution Phase

| Step | Component | Action |
|------|-----------|--------|
| 17 | Runtime | Begin execution |
| 18-19 | Runtime | Run the actual script/command |
| 20 | Runtime | Return result |
| 21 | CLI | Output to user |

**Runtime-specific behavior:**
- **Native**: Spawns host shell process
- **Virtual**: Interprets via mvdan/sh
- **Container**: Provisions image, runs container with transient retry handling
- **SSH/TUI lifecycle**: Managed by command orchestration (CommandService), not by the runtime implementation itself

### 5. Dry-Run Intercept

When `--ivk-dry-run` is passed, the pipeline short-circuits after resolution:

| Step | Component | Action |
|------|-----------|--------|
| 1 | CLI | Detect `--ivk-dry-run` flag |
| 2 | CLI | Run discovery + resolution as normal (steps 1-6) |
| 3 | CLI | Print resolved execution plan (Command, Source, Runtime, Platform, WorkDir, Timeout, Script, Environment) |
| 4 | CLI | Exit with code 0 (dependency validation is skipped) |

### 6. Watch Mode Loop

When `--ivk-watch` is passed, execution is wrapped in a watch loop:

| Step | Component | Action |
|------|-----------|--------|
| 1 | CLI | Detect `--ivk-watch` flag (mutually exclusive with `--ivk-dry-run`) |
| 2 | CLI | Run initial command execution normally |
| 3 | Watch Engine | Set up file watchers from `watch.patterns` (or `**/*` fallback) |
| 4 | Watch Engine | Wait for file changes, debounce (default 500ms) |
| 5 | CLI | Re-execute command |
| 6 | Watch Engine | Repeat from step 4 until Ctrl+C |

**Error handling:** Non-zero exit codes from the command continue the watch loop. Infrastructure errors (not `ExitError`) abort the loop after 3 consecutive failures.

## Error Handling Points

![Execution Error Categories](../diagrams/rendered/flowcharts/execution-errors.svg)

## Performance Considerations

| Phase | Typical Duration | Optimization |
|-------|------------------|--------------|
| Initialization | < 10ms | Config cached after first load |
| Discovery | 10-100ms | Depends on number of files/modules |
| Resolution | < 1ms | Simple lookup |
| Execution | Variable | Depends on command |

**Bottlenecks to watch:**
- Many modules in configured includes → slower discovery
- Large invowkfiles → slower parsing
- Container image pulls → can be minutes

## Related Diagrams

- [C4 Container Diagram](./c4-container.md) - Component relationships
- [Runtime Selection Flowchart](./flowchart-runtime-selection.md) - How runtimes are chosen
- [Discovery Precedence Flowchart](./flowchart-discovery.md) - How commands are discovered
