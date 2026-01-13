---
sidebar_position: 3
---

# Configuration Schema Reference

Complete reference for the Invowk configuration file schema.

## Overview

The configuration file uses [CUE](https://cuelang.org/) format and is located at:

| Platform | Location |
|----------|----------|
| Linux    | `~/.config/invowk/config.cue` |
| macOS    | `~/Library/Application Support/invowk/config.cue` |
| Windows  | `%APPDATA%\invowk\config.cue` |

## Schema Definition

```cue
// Root configuration structure
#Config: {
    container_engine?: "podman" | "docker"
    search_paths?:     [...string]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
}

// Virtual shell configuration
#VirtualShellConfig: {
    enable_uroot_utils?: bool
}

// UI configuration
#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
}
```

## Config

The root configuration object.

```cue
#Config: {
    container_engine?: "podman" | "docker"
    search_paths?:     [...string]
    default_runtime?:  "native" | "virtual" | "container"
    virtual_shell?:    #VirtualShellConfig
    ui?:               #UIConfig
}
```

### container_engine

**Type:** `"podman" | "docker"`  
**Required:** No  
**Default:** Auto-detected

Specifies which container runtime to use for container-based command execution.

```cue
container_engine: "podman"
```

**Auto-detection order:**
1. Podman (preferred)
2. Docker (fallback)

### search_paths

**Type:** `[...string]`  
**Required:** No  
**Default:** `["~/.invowk/cmds"]`

Additional directories to search for invkfiles and packs.

```cue
search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
    "/opt/company/invowk-commands",
]
```

**Path resolution:**
- Paths starting with `~` are expanded to the user's home directory
- Relative paths are resolved from the current working directory
- Non-existent paths are silently ignored

**Search priority (highest to lowest):**
1. Current directory
2. Paths in `search_paths` (in order)
3. Default `~/.invowk/cmds`

### default_runtime

**Type:** `"native" | "virtual" | "container"`  
**Required:** No  
**Default:** `"native"`

Sets the global default runtime mode for commands that don't specify a preferred runtime.

```cue
default_runtime: "virtual"
```

| Value | Description |
|-------|-------------|
| `"native"` | Execute using the system's native shell |
| `"virtual"` | Execute using Invowk's built-in shell interpreter |
| `"container"` | Execute inside a container |

### virtual_shell

**Type:** `#VirtualShellConfig`  
**Required:** No  
**Default:** `{}`

Configuration for the virtual shell runtime.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

### ui

**Type:** `#UIConfig`  
**Required:** No  
**Default:** `{}`

User interface configuration.

```cue
ui: {
    color_scheme: "dark"
    verbose: false
}
```

---

## VirtualShellConfig

Configuration for the virtual shell runtime (mvdan/sh).

```cue
#VirtualShellConfig: {
    enable_uroot_utils?: bool
}
```

### enable_uroot_utils

**Type:** `bool`  
**Required:** No  
**Default:** `false`

Enables u-root utilities in the virtual shell environment. When enabled, provides additional commands beyond basic shell builtins.

```cue
virtual_shell: {
    enable_uroot_utils: true
}
```

**Available utilities when enabled:**
- File operations: `ls`, `cat`, `cp`, `mv`, `rm`, `mkdir`, `chmod`
- Text processing: `grep`, `sed`, `awk`, `sort`, `uniq`
- And many more core utilities

---

## UIConfig

User interface configuration.

```cue
#UIConfig: {
    color_scheme?: "auto" | "dark" | "light"
    verbose?:      bool
}
```

### color_scheme

**Type:** `"auto" | "dark" | "light"`  
**Required:** No  
**Default:** `"auto"`

Sets the color scheme for terminal output.

```cue
ui: {
    color_scheme: "auto"
}
```

| Value | Description |
|-------|-------------|
| `"auto"` | Detect from terminal (respects `COLORTERM`, `TERM`, etc.) |
| `"dark"` | Colors optimized for dark terminals |
| `"light"` | Colors optimized for light terminals |

### verbose

**Type:** `bool`  
**Required:** No  
**Default:** `false`

Enables verbose output by default for all commands.

```cue
ui: {
    verbose: true
}
```

Equivalent to always passing `--verbose` on the command line.

---

## Complete Example

A fully documented configuration file:

```cue
// Invowk Configuration File
// =========================
// Location: ~/.config/invowk/config.cue

// Container Engine
// ----------------
// Which container runtime to use: "podman" or "docker"
// If not specified, Invowk auto-detects (prefers Podman)
container_engine: "podman"

// Search Paths
// ------------
// Additional directories to search for invkfiles and packs
// Searched in order after the current directory
search_paths: [
    // Personal commands
    "~/.invowk/cmds",
    
    // Team shared commands
    "~/work/shared-commands",
    
    // Organization-wide commands
    "/opt/company/invowk-commands",
]

// Default Runtime
// ---------------
// The runtime to use when a command doesn't specify one
// Options: "native", "virtual", "container"
default_runtime: "native"

// Virtual Shell Configuration
// ---------------------------
// Settings for the virtual shell runtime (mvdan/sh)
virtual_shell: {
    // Enable u-root utilities for more shell commands
    // Provides ls, cat, grep, etc. in the virtual environment
    enable_uroot_utils: true
}

// UI Configuration
// ----------------
// User interface settings
ui: {
    // Color scheme: "auto", "dark", or "light"
    // "auto" detects from terminal settings
    color_scheme: "auto"
    
    // Enable verbose output by default
    // Same as always passing --verbose
    verbose: false
}
```

---

## Minimal Configuration

If you're happy with defaults, a minimal config might be:

```cue
// Just override what you need
container_engine: "docker"
```

Or even an empty file (all defaults):

```cue
// Empty config - use all defaults
```

---

## Validation

You can validate your configuration file using CUE:

```bash
cue vet ~/.config/invowk/config.cue
```

Or check it with Invowk:

```bash
invowk config show
```

If there are any errors, Invowk will report them when loading the configuration.
