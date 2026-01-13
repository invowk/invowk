---
sidebar_position: 1
---

# Configuration Overview

Invowk uses a CUE-based configuration file to customize its behavior. This is where you set your preferences for container engines, search paths, runtime defaults, and more.

## Configuration File Location

The configuration file lives in your OS-specific config directory:

| Platform | Location |
|----------|----------|
| Linux    | `~/.config/invowk/config.cue` |
| macOS    | `~/Library/Application Support/invowk/config.cue` |
| Windows  | `%APPDATA%\invowk\config.cue` |

You can also specify a custom config file path using the `--config` flag:

```bash
invowk --config /path/to/my/config.cue cmd list
```

## Creating a Configuration File

The easiest way to create a configuration file is to use the `config init` command:

```bash
invowk config init
```

This creates a default configuration file with sensible defaults. If a config file already exists, it won't be overwritten (safety first!).

## Viewing Your Configuration

There are several ways to inspect your current configuration:

### Show Human-Readable Config

```bash
invowk config show
```

This displays your configuration in a friendly, readable format.

### Show Raw CUE

```bash
invowk config dump
```

This outputs the raw CUE configuration, useful for debugging or copying to another machine.

### Find the Config File

```bash
invowk config path
```

This prints the path to your configuration file. Handy when you want to edit it directly.

## Setting Configuration Values

You can modify configuration values from the command line:

```bash
# Set the container engine
invowk config set container_engine podman

# Set the default runtime
invowk config set default_runtime virtual

# Set the color scheme
invowk config set ui.color_scheme dark
```

Or just open the config file in your favorite editor:

```bash
# Linux/macOS
$EDITOR $(invowk config path)

# Windows PowerShell
notepad (invowk config path)
```

## Example Configuration

Here's what a typical configuration file looks like:

```cue
// ~/.config/invowk/config.cue

// Container engine: "podman" or "docker"
container_engine: "podman"

// Additional directories to search for invkfiles
search_paths: [
    "~/.invowk/cmds",
    "~/projects/shared-commands",
]

// Default runtime for commands that don't specify one
default_runtime: "native"

// Virtual shell configuration
virtual_shell: {
    enable_uroot_utils: true
}

// UI preferences
ui: {
    color_scheme: "auto"  // "auto", "dark", or "light"
    verbose: false
}
```

## Configuration Hierarchy

When running a command, Invowk merges configuration from multiple sources (later sources override earlier ones):

1. **Built-in defaults** - Sensible defaults for all options
2. **Configuration file** - Your `config.cue` settings
3. **Environment variables** - `INVOWK_*` environment variables
4. **Command-line flags** - Flags like `--verbose`, `--runtime`

For example, if your config file sets `verbose: false`, but you run with `--verbose`, verbose mode will be enabled.

## What's Next?

Head over to [Configuration Options](./options) for a complete reference of all available settings.
