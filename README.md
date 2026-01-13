# invowk

A dynamically extensible, CLI-based command runner similar to [just](https://github.com/casey/just), written in Go.

## Features

- **Three Runtime Modes**:
  - **native**: Execute commands using the system's default shell (bash, sh, powershell, etc.)
  - **virtual**: Execute commands using the built-in [mvdan/sh](https://github.com/mvdan/sh) interpreter
  - **container**: Execute commands inside a disposable Docker/Podman container

- **CUE Configuration**: Define commands in `invowkfile.cue` files using [CUE](https://cuelang.org/) - a powerful configuration language with validation

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
invowk cmd
# or
invowk cmd --list
```

The list shows all commands with their group prefix, allowed runtimes (default marked with `*`):

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*, container] (linux, macos, windows)
  myproject test unit - Run unit tests [native*, virtual] (linux, macos, windows)
  myproject docker-build - Build in container [container*] (linux, macos, windows)
```

3. **Run a command** (use the group prefix):

```bash
invowk cmd myproject build
```

4. **Run a command with a specific runtime**:

```bash
# Use a non-default runtime (must be allowed by the command)
invowk cmd myproject build --runtime container
```

## Invowkfile Format

Invowkfiles are written in [CUE](https://cuelang.org/) format. CUE provides powerful validation, templating, and a clean syntax. Here's an example:

```cue
group: "myproject"  // Required: namespace for all commands in this file
version: "1.0"
description: "My project commands"

// Define commands
commands: [
	{
		name: "build"
		description: "Build the project"
		implementations: [
			{
				// Multi-line scripts use triple quotes
				script: """
					echo "Building ${PROJECT_NAME}..."
					go build -o bin/app ./...
					"""
				// Allowed runtimes (first is default). Container runtime requires image or containerfile.
				target: {
					runtimes: [
						{name: "native"},
						{name: "container", image: "golang:1.21"},
					]
					// Platform-specific environment variables
					platforms: [
						{name: "linux", env: {PROJECT_NAME: "myproject"}},
						{name: "macos", env: {PROJECT_NAME: "myproject"}},
						{name: "windows", env: {PROJECT_NAME: "myproject"}},
					]
				}
			}
		]
	},
	// Use spaces in names for subcommand-like behavior
	// This creates: invowk cmd myproject test unit
	{
		name: "test unit"
		description: "Run unit tests"
		implementations: [
			{
				script: "go test ./..."
				target: {
					runtimes: [{name: "native"}, {name: "virtual"}]  // Can run in native or virtual runtime
				}
			}
		]
	},
	// Commands can also reference external script files
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "./scripts/deploy.sh"
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
	},
	// Dependencies: tools (binaries in PATH) and commands (other invowk commands)
	{
		name: "release"
		description: "Create a release"
		implementations: [
			{
				script: "echo 'Creating release...'"
				target: {
					runtimes: [{name: "native"}]
					platforms: [{name: "linux"}, {name: "macos"}]
				}
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["git"]},
			]
			commands: [
				{alternatives: ["myproject build"]},
				{alternatives: ["myproject test unit"]},
			]
		}
	},
	// Run commands in a container (requires image or containerfile)
	{
		name: "container hello-invowk"
		description: "Print a greeting from a container"
		implementations: [
			{
				script: "echo \"Hello, Invowk!\""
				target: {
					runtimes: [{name: "container", image: "alpine:latest"}]
				}
			}
		]
	},
]
```

## Command Groups

Every invowkfile must specify a **group** field. The group becomes the first segment of all command names from that file, creating a namespace for the commands.

### Group Field Format

```cue
group: "mygroup"           // Simple group
group: "my.nested.group"   // Nested group using dot notation
```

**Validation rules:**
- Must start with a letter (a-z, A-Z)
- Can contain letters and numbers
- Nested groups use dots (`.`) as separators
- Each segment must start with a letter

**Valid examples:** `mygroup`, `my.group`, `my.nested.group`, `Group1`, `a.b.c`

**Invalid examples:** `.group`, `group.`, `my..group`, `my-group`, `my_group`, `1group`

### How Groups Affect Command Names

When you define a command in an invowkfile with `group: "myproject"`:

```cue
group: "myproject"
commands: [
    {name: "build", ...},
    {name: "test unit", ...},
]
```

The commands are accessed with the group as a prefix:

```bash
invowk cmd myproject build
invowk cmd myproject test unit
```

### Benefits of Command Groups

1. **Namespace isolation**: Multiple invowkfiles can have commands with the same name without conflicts
2. **Clear provenance**: You know which invowkfile a command comes from
3. **Hierarchical organization**: Use dot notation for logical grouping (e.g., `frontend.components`, `backend.api`)
4. **Tab completion**: Groups provide natural completion boundaries

### Command Dependencies with Groups

When referencing command dependencies, use the full group-prefixed name:

```cue
group: "myproject"
commands: [
    {
        name: "release"
        depends_on: {
            commands: [
                {alternatives: ["myproject build"]},      // Same-file command
                {alternatives: ["myproject test unit"]},  // Same-file nested command
                {alternatives: ["other.project deploy"]}, // Command from another invowkfile
            ]
        }
    }
]
```

## Dependencies

Commands can specify dependencies that must be satisfied before running:

### Tool Dependencies

Verify that required binaries are available in PATH. You can specify alternatives with OR semantics (any alternative found satisfies the dependency):

```cue
depends_on: {
	tools: [
		// Simple check - just verify tool is in PATH
		{alternatives: ["git"]},
		
		// Multiple alternatives - any one satisfies the dependency
		{alternatives: ["podman", "docker"]},
	]
}
```

**Tool validation options:**
- `alternatives` (required): List of binary names that can satisfy this dependency (OR semantics)

### Command Dependencies

Run other invowk commands first. Use the full group-prefixed command name:

```cue
depends_on: {
	commands: [
		{alternatives: ["myproject clean"]},
		{alternatives: ["myproject build"]},
		// Multiple alternatives - any one satisfies the dependency
		{alternatives: ["myproject test unit", "myproject test integration"]},
	]
}
```

### Filepath Dependencies

Check that required files or directories exist with proper permissions. You can specify multiple alternative paths where if any one exists, the dependency is satisfied:

```cue
depends_on: {
	filepaths: [
		// Simple existence check - any of these files satisfies the dependency
		{alternatives: ["go.mod", "go.sum", "Gopkg.toml"]},
		
		// Check with read permission - any of these READMEs works
		{alternatives: ["README.md", "README", "readme.md"], readable: true},
		
		// Check with write permission
		{alternatives: ["output", "dist", "build"], writable: true},
		
		// Check with execute permission
		{alternatives: ["scripts/deploy.sh"], executable: true},
		
		// Absolute paths are also supported
		{alternatives: ["/etc/app/config.yaml", "./config.yaml"], readable: true},
	]
}
```

**Filepath validation options:**
- `alternatives` (required): List of file or directory paths (at least one). If any path exists and satisfies the permission requirements, the dependency is considered satisfied.
- `readable` (optional): Check if path is readable
- `writable` (optional): Check if path is writable
- `executable` (optional): Check if path is executable

Permission checks are cross-platform compatible (Linux, macOS, Windows).

When dependencies are not satisfied, invowk displays a styled error message listing all issues at once.

### Script-Level Dependencies

Dependencies can also be specified at the script level, which is especially useful for container-based implementations:

```cue
commands: [
	{
		name: "docker-build"
		implementations: [
			{
				script: "go build -o /workspace/bin/app ./..."
				target: {
					runtimes: [{name: "container", image: "golang:1.21"}]
				}
				// Implementation-level depends_on - validated within the container
				depends_on: {
					tools: [
						{alternatives: ["go"]},
					]
					filepaths: [
						{alternatives: ["/workspace/go.mod"]},
					]
				}
			}
		]
	}
]
```

**Runtime-Aware Validation:**

When either command-level or implementation-level dependencies are used, the validation behavior changes according to the runtime used at execution time:

- **native**: Dependencies are validated against the native standard shell from the host system
- **virtual**: Dependencies are validated against invowk's built-in sh interpreter with core utils  
- **container**: Dependencies are validated against the container's default shell from within the container itself

This allows you to specify dependencies that need to exist inside the container rather than on the host system.

## Platform Compatibility

Every command must specify which operating systems it supports using the `works_on` field:

```cue
commands: [
	{
		name: "build"
		script: "make build"
		works_on: {
			hosts: ["linux", "mac", "windows"]  // Runs on all platforms
		}
	},
	{
		name: "clean"
		script: "rm -rf bin/"
		works_on: {
			hosts: ["linux", "mac"]  // Unix-only command
		}
	},
]
```

**Supported host values:**
- `linux`: Linux operating systems
- `mac`: macOS (Darwin)
- `windows`: Windows

When you run `invowk cmd list`, the supported hosts are displayed for each command:

```
Available Commands

From current directory:
  myproject build - Build the project [native] (linux, macos, windows)
  myproject clean - Clean build artifacts [native] (linux, macos)
```

If you try to run a command on an unsupported platform, invowk displays a styled error message explaining which platforms are supported.

## Script Sources

Commands can use either **inline scripts** or **script files**:

### Inline Scripts

Use single-line or multi-line (with triple quotes `"""`) CUE strings:

```cue
commands: [
	{
		name: "build"
		script: """
			#!/bin/bash
			set -e
			echo "Building..."
			go build ./...
			"""
	},
]
```

### Script Files

Reference external script files using paths:

```cue
commands: [
	// Relative to invowkfile location
	{
		name: "deploy"
		script: "./scripts/deploy.sh"
	},
	// Absolute path
	{
		name: "system-check"
		script: "/usr/local/bin/check.sh"
	},
	// Just the filename (if it has a recognized extension)
	{
		name: "test"
		script: "run-tests.sh"
	},
]
```

Recognized script extensions: `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

## Runtime Modes

### Native Runtime

Uses the system's default shell:
- **Linux/macOS**: Uses `$SHELL`, or falls back to `bash` → `sh`
- **Windows**: Uses `pwsh` → `powershell` → `cmd`

```cue
commands: [
	{
		name: "build"
		runtime: "native"
		script: "go build ./..."
	},
]
```

### Virtual Runtime

Uses the built-in [mvdan/sh](https://github.com/mvdan/sh) shell interpreter. This provides a consistent POSIX-like shell experience across platforms.

```cue
commands: [
	{
		name: "build"
		runtime: "virtual"
		script: """
			echo "Building..."
			go build ./...
			"""
	},
]
```

### Container Runtime

Runs commands inside a Docker or Podman container. Requires an image or containerfile specification in the runtime config.

```cue
commands: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				// Container config is specified in the runtime
				target: {
					runtimes: [{
						name: "container",
						image: "golang:1.21",
						volumes: ["./data:/data"],
						ports: ["8080:8080"],
					}]
				}
			}
		]
	},
]
```

### Host SSH Access from Containers

Container commands can optionally SSH back into the host system. When `enable_host_ssh: true` is set inside the container runtime configuration, invowk starts a secure SSH server using the [Wish](https://github.com/charmbracelet/wish) library and provides connection credentials to the container via environment variables.

**Security**: The SSH server only accepts token-based authentication. Each command execution gets a unique, time-limited token that is automatically revoked after the command completes.

```cue
commands: [
	{
		name: "deploy from container"
		implementations: [
			{
				script: """
					# Connection info is available via environment variables:
					# - INVOWK_SSH_HOST: Host address (host.docker.internal or host.containers.internal)
					# - INVOWK_SSH_PORT: SSH server port
					# - INVOWK_SSH_USER: Username (invowk)
					# - INVOWK_SSH_TOKEN: One-time authentication token
					
					# Example: Run a command on the host
					sshpass -p $INVOWK_SSH_TOKEN ssh -o StrictHostKeyChecking=no \
						$INVOWK_SSH_USER@$INVOWK_SSH_HOST -p $INVOWK_SSH_PORT \
						'echo "Hello from host!"'
					"""
				// enable_host_ssh and image are specified inside the container runtime config
				target: {
					runtimes: [{name: "container", image: "alpine:latest", enable_host_ssh: true}]
				}
			}
		]
	},
]
```

**Note**: The container needs `sshpass` or similar tools to use password-based SSH authentication. You may need to install it in your container image.

## Configuration

invowk uses a global configuration file (TOML format):

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
invowk cmd myproject build
```

### Run a Command with Spaces in Name
```bash
invowk cmd myproject test unit
```

### Override Runtime
```bash
invowk cmd myproject build --runtime virtual
```

### Verbose Mode
```bash
invowk --verbose cmd myproject build
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

