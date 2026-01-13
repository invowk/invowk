---
sidebar_position: 2
---

# Configuration Options

This page documents all available configuration options for Invowk.

## Configuration Schema

The configuration file uses CUE format and follows this schema:

```cue
#Config: {
    container_engine?: "podman" | "docker"
    search_paths?: [...string]
    default_runtime?: "native" | "virtual" | "container"
    virtual_shell?: #VirtualShellConfig
    ui?: #UIConfig
}
```

## Options Reference

### container_engine

**Type:** `"podman" | "docker"`  
**Default:** Auto-detected (prefers Podman if available)

Specifies which container runtime to use for container-based command execution.

```cue
container_engine: "podman"
```

Invowk will auto-detect available container engines if not specified:
1. First checks for Podman
2. Falls back to Docker if Podman isn't available
3. Returns an error if neither is found (only when container runtime is needed)

### search_paths

**Type:** `[...string]`  
**Default:** `["~/.invowk/cmds"]`

Additional directories to search for invkfiles. Paths are searched in order after the current directory.

```cue
search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
    "/opt/company/invowk-commands",
]
```

**Search Order:**
1. Current directory (always searched first, highest priority)
2. Each path in `search_paths` in order
3. `~/.invowk/cmds` (default, always included)

Commands from earlier paths override commands with the same name from later paths.

### default_runtime

**Type:** `"native" | "virtual" | "container"`  
**Default:** `"native"`

Sets the global default runtime mode for commands that don't specify a runtime.

```cue
default_runtime: "virtual"
```

**Runtime Options:**
- `"native"` - Execute using the system's native shell (bash, zsh, PowerShell, etc.)
- `"virtual"` - Execute using Invowk's built-in shell interpreter (mvdan/sh)
- `"container"` - Execute inside a container (requires Docker or Podman)

:::note
Commands can override this default by specifying their own runtime in the `implementations` field.
:::

### virtual_shell

**Type:** `#VirtualShellConfig`  
**Default:** `{}`

Configures the virtual shell runtime behavior.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

#### virtual_shell.enable_uroot_utils

**Type:** `bool`  
**Default:** `false`

Enables u-root utilities in the virtual shell. When enabled, additional commands like `ls`, `cat`, `grep`, and others become available in the virtual shell environment.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

This is useful when you want the virtual shell to have more capabilities beyond basic shell builtins, while still avoiding native shell execution.

### ui

**Type:** `#UIConfig`  
**Default:** `{}`

Configures the user interface settings.

```cue
ui: {
    color_scheme: "dark"
    verbose: false
}
```

#### ui.color_scheme

**Type:** `"auto" | "dark" | "light"`  
**Default:** `"auto"`

Sets the color scheme for Invowk's output.

```cue
ui: {
    color_scheme: "auto"
}
```

**Options:**
- `"auto"` - Detect from terminal settings (respects `COLORTERM`, `TERM`, etc.)
- `"dark"` - Use colors optimized for dark terminals
- `"light"` - Use colors optimized for light terminals

#### ui.verbose

**Type:** `bool`  
**Default:** `false`

Enables verbose output by default. When enabled, Invowk prints additional information about command discovery, dependency validation, and execution.

```cue
ui: {
    verbose: true
}
```

This is equivalent to always passing `--verbose` on the command line.

## Complete Example

Here's a complete configuration file with all options:

```cue
// Invowk Configuration File
// Located at: ~/.config/invowk/config.cue

// Use Podman as the container engine
container_engine: "podman"

// Search for invkfiles in these directories
search_paths: [
    "~/.invowk/cmds",          // Personal commands
    "~/work/shared-commands",   // Team shared commands
]

// Default to virtual shell for portability
default_runtime: "virtual"

// Virtual shell settings
virtual_shell: {
    // Enable u-root utilities for more shell commands
    enable_uroot_utils: true
}

// UI preferences
ui: {
    // Auto-detect color scheme from terminal
    color_scheme: "auto"
    
    // Don't be verbose by default
    verbose: false
}
```

## Environment Variable Overrides

Some configuration options can be overridden with environment variables:

| Environment Variable | Overrides |
|---------------------|-----------|
| `INVOWK_CONFIG` | Config file path |
| `INVOWK_VERBOSE` | `ui.verbose` (set to `1` or `true`) |
| `INVOWK_CONTAINER_ENGINE` | `container_engine` |

```bash
# Example: Use Docker instead of configured Podman
INVOWK_CONTAINER_ENGINE=docker invowk cmd build

# Example: Enable verbose output for this run
INVOWK_VERBOSE=1 invowk cmd test
```

## Command-Line Overrides

All configuration options can be overridden via command-line flags:

```bash
# Override config file
invowk --config /path/to/config.cue cmd list

# Override verbose setting
invowk --verbose cmd build

# Override runtime for a command
invowk cmd build --runtime container
```

See [CLI Reference](/docs/reference/cli) for all available flags.
