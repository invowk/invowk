# invowk

A dynamically extensible, CLI-based command runner similar to [just](https://github.com/casey/just), written in Go.

## Features

- **Three Runtime Modes**:
  - **native**: Execute commands using the system's default shell (bash, sh, powershell, etc.)
  - **virtual**: Execute commands using the built-in [mvdan/sh](https://github.com/mvdan/sh) interpreter
  - **container**: Execute commands inside a disposable Docker/Podman container

- **TOML 1.1 Configuration**: Define commands in easy-to-read `invowkfile` files using TOML syntax

- **Cross-Platform**: Works on Linux, Windows, and macOS

- **Hierarchical Commands**: Use spaces in command names to create subcommand-like hierarchies (e.g., `invowk cmd test unit`)

- **Command Dependencies**: Commands can depend on other commands that run first

- **Multiple Command Sources**: Discover commands from:
  1. Current directory (highest priority)
  2. User commands directory (`~/.invowk/cmds/`)
  3. Configured search paths

- **Shell Completion**: Full tab completion support for bash, zsh, fish, and PowerShell

- **Beautiful CLI**: Styled output using [Cobra](https://github.com/spf13/cobra) with [Lip Gloss](https://github.com/charmbracelet/lipgloss) styling

## Installation

### From Source

```bash
git clone https://github.com/yourusername/invowk
cd invowk
go build -o invowk .
```

### Installing the Binary

Move the built binary to a location in your PATH:

```bash
sudo mv invowk /usr/local/bin/
```

## Quick Start

1. **Create an invowkfile** in your project directory:

```bash
invowk init
```

2. **List available commands**:

```bash
invowk cmd list
```

3. **Run a command**:

```bash
invowk cmd build
```

## Invowkfile Format

Invowkfiles are written in TOML 1.1 format. Here's an example:

```toml
version = "1.0"
description = "My project commands"
default_runtime = "native"

# Global environment variables
[env]
PROJECT_NAME = "myproject"

# Container configuration (optional)
[container]
dockerfile = "Dockerfile"
# Or use a pre-built image:
image = "alpine:latest"

# Define commands with inline scripts (multi-line supported via ''')
[[commands]]
name = "build"
description = "Build the project"
script = '''
echo "Building ${PROJECT_NAME}..."
go build -o bin/app ./...
'''

# Use spaces in names for subcommand-like behavior
# This creates: invowk cmd test unit
[[commands]]
name = "test unit"
description = "Run unit tests"
script = "go test ./..."

# Commands can also reference external script files
[[commands]]
name = "deploy"
description = "Deploy the application"
script = "./scripts/deploy.sh"

[[commands]]
name = "release"
description = "Create a release"
depends_on = ["build", "test unit"]
script = "echo 'Creating release...'"

# Run commands in a container
[[commands]]
name = "container hello-invowk"
description = "Print a greeting from a container"
runtime = "container"
script = 'echo "Hello, Invowk!"'
```

## Script Sources

Commands can use either **inline scripts** or **script files**:

### Inline Scripts

Use single-line or multi-line (with `'''`) TOML strings:

```toml
[[commands]]
name = "build"
script = '''
#!/bin/bash
set -e
echo "Building..."
go build ./...
'''
```

### Script Files

Reference external script files using paths:

```toml
# Relative to invowkfile location
[[commands]]
name = "deploy"
script = "./scripts/deploy.sh"

# Absolute path
[[commands]]
name = "system-check"
script = "/usr/local/bin/check.sh"

# Just the filename (if it has a recognized extension)
[[commands]]
name = "test"
script = "run-tests.sh"
```

Recognized script extensions: `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

## Runtime Modes

### Native Runtime

Uses the system's default shell:
- **Linux/macOS**: Uses `$SHELL`, or falls back to `bash` → `sh`
- **Windows**: Uses `pwsh` → `powershell` → `cmd`

```toml
[[commands]]
name = "build"
runtime = "native"
script = "go build ./..."
```

### Virtual Runtime

Uses the built-in [mvdan/sh](https://github.com/mvdan/sh) shell interpreter. This provides a consistent POSIX-like shell experience across platforms.

```toml
[[commands]]
name = "build"
runtime = "virtual"
script = '''
echo "Building..."
go build ./...
'''
```

### Container Runtime

Runs commands inside a Docker or Podman container. Requires a Dockerfile or image specification.

```toml
[container]
dockerfile = "Dockerfile"
volumes = ["./data:/data"]
ports = ["8080:8080"]

[[commands]]
name = "build"
runtime = "container"
script = "make build"
```

## Configuration

invowk uses a global configuration file:

- **Linux**: `~/.config/invowk/config.toml`
- **macOS**: `~/Library/Application Support/invowk/config.toml`
- **Windows**: `%APPDATA%\invowk\config.toml`

### Create Default Configuration

```bash
invowk config init
```

### View Current Configuration

```bash
invowk config show
```

### Configuration Options

```toml
# Container engine preference: "podman" or "docker"
container_engine = "podman"

# Default runtime mode: "native", "virtual", or "container"
default_runtime = "native"

# Additional directories to search for invowkfiles
search_paths = [
    "/home/user/global-commands"
]

# Virtual shell options
[virtual_shell]
enable_uroot_utils = true

# UI options
[ui]
color_scheme = "auto"  # "auto", "dark", "light"
verbose = false
```

## Shell Completion

### Bash

```bash
# Add to ~/.bashrc:
eval "$(invowk completion bash)"

# Or install system-wide:
invowk completion bash > /etc/bash_completion.d/invowk
```

### Zsh

```bash
# Add to ~/.zshrc:
eval "$(invowk completion zsh)"

# Or install to fpath:
invowk completion zsh > "${fpath[1]}/_invowk"
```

### Fish

```bash
invowk completion fish > ~/.config/fish/completions/invowk.fish
```

### PowerShell

```powershell
invowk completion powershell | Out-String | Invoke-Expression

# Or add to $PROFILE:
invowk completion powershell >> $PROFILE
```

## Command Examples

### List Commands
```bash
invowk cmd list
```

### Run a Command
```bash
invowk cmd build
```

### Run a Command with Spaces in Name
```bash
invowk cmd test unit
```

### Override Runtime
```bash
invowk cmd build --runtime virtual
```

### Verbose Mode
```bash
invowk --verbose cmd build
```

## Project Structure

```
invowk-cli/
├── main.go                     # Entry point
├── cmd/invowk/                 # CLI commands
│   ├── root.go                 # Root command
│   ├── cmd.go                  # cmd subcommand
│   ├── init.go                 # init command
│   ├── config.go               # config commands
│   └── completion.go           # completion command
├── internal/
│   ├── config/                 # Configuration handling
│   ├── container/              # Container engine abstraction
│   │   ├── engine.go           # Engine interface
│   │   ├── docker.go           # Docker implementation
│   │   └── podman.go           # Podman implementation
│   ├── discovery/              # Invowkfile discovery
│   ├── issue/                  # Error types and messages
│   └── runtime/                # Runtime implementations
│       ├── runtime.go          # Runtime interface
│       ├── native.go           # Native shell runtime
│       ├── virtual.go          # Virtual shell runtime
│       └── container.go        # Container runtime
└── pkg/invowkfile/             # Invowkfile parsing
```

## Dependencies

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [go-toml](https://github.com/pelletier/go-toml) - TOML parsing
- [mvdan/sh](https://github.com/mvdan/sh) - Virtual shell interpreter
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering

## License

MIT License - see LICENSE file for details.

