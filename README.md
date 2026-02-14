# invowk™

A dynamically extensible, CLI-based command runner similar to [just](https://github.com/casey/just), written in Go.

## Features

- **Three Runtime Modes**:
  - **native**: Execute commands using the system's default shell (bash, sh, powershell, etc.)
  - **virtual**: Execute commands using the built-in [mvdan/sh](https://github.com/mvdan/sh) interpreter with 28 [u-root](https://github.com/u-root/u-root) utilities (cat, cp, ls, grep, sort, seq, tar, etc.)
  - **container**: Execute commands inside a disposable Docker/Podman container

- **CUE Configuration**: Define commands in `invowkfile.cue` files using [CUE](https://cuelang.org/) - a powerful configuration language with validation

- **Cross-Platform**: Works on Linux, Windows, and macOS

- **Hierarchical Commands**: Use spaces in command names to create subcommand-like hierarchies (e.g., `invowk cmd test unit`)

- **Command Dependencies**: Commands can require other commands to be discoverable

- **Multiple Command Sources**: Discover commands from:
  1. Current directory (`invowkfile.cue` + sibling `*.invowkmod` modules, highest priority)
  2. Configured includes (module paths from config)
  3. `~/.invowk/cmds/` (modules only, non-recursive)

- **Transparent Namespace**: Commands from different sources use simple names when unique. When command names conflict across sources, use `@<source>` prefix or `--ivk-from` flag to disambiguate

- **Shell Completion**: Full tab completion support for bash, zsh, fish, and PowerShell

- **Beautiful CLI**: Styled output using [Cobra](https://github.com/spf13/cobra) with [Lip Gloss](https://github.com/charmbracelet/lipgloss) styling

- **Interactive TUI Components**: Built-in gum-like terminal UI components for creating interactive shell scripts (input, choose, confirm, filter, file picker, table, spinner, pager, format, style)

- **Module Dependencies**: Modules can import dependencies from remote Git repositories (GitHub, GitLab) with semantic versioning support and lock files for reproducibility

## Installation

### Shell Script (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh
```

This downloads the latest release, verifies its SHA256 checksum, and installs to `~/.local/bin`. Customize with environment variables:

```bash
INSTALL_DIR=/usr/local/bin INVOWK_VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/invowk/invowk/main/scripts/install.sh | sh
```

### Homebrew (macOS/Linux)

```bash
brew install invowk/tap/invowk
```

### Go Install

```bash
go install github.com/invowk/invowk@latest
```

Requires Go 1.26+. The binary is installed to `$GOBIN` (or `$GOPATH/bin`).

### From Source

```bash
git clone https://github.com/invowk/invowk
cd invowk
make build
make install  # Installs to $GOPATH/bin
```

> **Note:** On x86-64 systems, the default build targets the x86-64-v3 microarchitecture (Haswell+ CPUs from 2013+) for optimal performance. For maximum compatibility with older CPUs, use `make build GOAMD64=v1`.

### Verify Installation

```bash
invowk --version
```

### Upgrading

```bash
# Check for updates
invowk upgrade --check

# Upgrade to latest
invowk upgrade

# Upgrade to a specific version
invowk upgrade v1.2.0
```

If installed via Homebrew, use `brew upgrade invowk` instead. If installed via `go install`, use `go install github.com/invowk/invowk@latest`.

### Platform Support

| Method | Linux | macOS | Windows |
|--------|-------|-------|---------|
| Shell script | amd64, arm64 | amd64 (Intel), arm64 (Apple Silicon) | — |
| Homebrew | amd64, arm64 | amd64, arm64 | — |
| Go install | all | all | all |
| From source | all | all | all |

## Quick Start

1. **Create an invowkfile** in your project directory:

```bash
invowk init
```

2. **List available commands**:

```bash
invowk cmd
```

The list shows all commands grouped by source (invowkfile or module) with allowed runtimes (default marked with `*`). Commands use their **simple names** - no module prefix is required when names are unique:

```
Available Commands
  (* = default runtime)

From invowkfile:
  build - Build the project [native*, container] (linux, macos, windows)
  test unit - Run unit tests [native*, virtual] (linux, macos, windows)
  deploy - Deploy the application (@invowkfile) [native*] (linux, macos)

From tools.invowkmod:
  lint - Run linter [native*] (linux, macos, windows)
  deploy - Deploy to staging (@tools) [native*] (linux, macos)
```

When a command name exists in multiple sources (like `deploy` above), the listing shows a source annotation (`@invowkfile`, `@tools`) to indicate disambiguation is required.

3. **Run a command**:

```bash
invowk cmd build
```

4. **Run a command with a specific runtime**:

```bash
# Use a non-default runtime (must be allowed by the command)
invowk cmd build --ivk-runtime container
```

## Invowkfile Format

Invowkfiles are written in [CUE](https://cuelang.org/) format. CUE provides powerful validation, templating, and a clean syntax. Invowkfiles contain **commands only**; module metadata lives in `invowkmod.cue` when you build a module. Here's an example:

```cue
// Define commands
cmds: [
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
				runtimes: [
					{name: "native"},
					{name: "container", image: "golang:1.26"},
				]
				platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
				// Environment variables for this implementation
				env: {vars: {PROJECT_NAME: "myproject"}}
			}
		]
	},
	// Use spaces in names for subcommand-like behavior
	// This creates: invowk cmd test unit
	{
		name: "test unit"
		description: "Run unit tests"
		implementations: [
			{
				script: "go test ./..."
				runtimes: [{name: "native"}, {name: "virtual"}]  // Can run in native or virtual runtime
				platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
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
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
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
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			tools: [
				{alternatives: ["git"]},
			]
			cmds: [
				{alternatives: ["build"]},
				{alternatives: ["test unit"]},
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
				runtimes: [{name: "container", image: "debian:stable-slim"}]
				platforms: [{name: "linux"}]
			}
		]
	},
]
```

### Root-Level Settings

Invowkfiles support several root-level settings that apply to all commands:

```cue
// Override the default shell for native runtime (optional)
default_shell: "/bin/bash"

// Set a default working directory for all commands (optional)
// Can be absolute or relative to the invowkfile location
workdir: "./src"

// Global environment configuration (optional)
// Applied to all commands; command-level and implementation-level env override these
env: {
	files: [".env", ".env.local?"]  // Load from .env files ('?' suffix = optional)
	vars: {
		PROJECT_NAME: "myapp"
	}
}

// Global dependencies that apply to all commands (optional)
depends_on: {
	tools: [{alternatives: ["git"]}]
}

cmds: [...]
```

**Root-level fields:**

| Field | Description |
|-------|-------------|
| `default_shell` | Override the default shell for native runtime (e.g., `/bin/bash`, `pwsh`) |
| `workdir` | Default working directory for all commands (overridable per command/implementation) |
| `env` | Global environment config with `files` (dotenv loading) and `vars` (key-value pairs) |
| `depends_on` | Global dependencies validated for every command in this invowkfile |

### Env Files

Load environment variables from `.env` files at any scope level:

```cue
env: {
	// Files are loaded in order; later files override earlier ones
	// Suffix with '?' to make a file optional (no error if missing)
	files: [".env", ".env.local?", ".env.${ENV}?"]
	vars: {
		// Direct variables override values from files
		APP_NAME: "myapp"
	}
}
```

### Working Directory

Control where commands execute using `workdir` at root, command, or implementation level. Implementation-level overrides command-level, which overrides root-level:

```cue
cmds: [
	{
		name: "test"
		workdir: "./packages/api"  // Command-level workdir
		implementations: [
			{
				script: "go test ./..."
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
				workdir: "./packages/api/v2"  // Implementation-level override
			}
		]
	}
]
```

You can also override the working directory at runtime with the `--ivk-workdir` / `-w` flag:

```bash
invowk cmd test --ivk-workdir=./packages/frontend
```

## Module Metadata (invowkmod.cue)

Modules (directories ending in `.invowkmod`) use a separate metadata file named `invowkmod.cue`. It defines the module identifier, optional description, and dependencies.

```cue
module: "mymodule"
version: "1.0.0"
description: "Reusable build tools"
```

### Module Field Format

```cue
module: "mymodule"           // Simple module name
module: "my.nested.module"   // Nested module using dot notation (RDNS style)
```

**Validation rules:**
- Must start with a letter (a-z, A-Z)
- Can contain letters and numbers
- Nested modules use dots (`.`) as separators
- Each segment must start with a letter

**Valid examples:** `mymodule`, `my.module`, `my.nested.module`, `Module1`, `a.b.c`

**Invalid examples:** `.module`, `module.`, `my..module`, `my-module`, `my_module`, `1module`

### How Multi-Source Discovery Works

When you run `invowk cmd` in a directory, invowk discovers commands from **multiple sources**:

1. **Root invowkfile**: `invowkfile.cue` in the current directory
2. **Sibling modules**: All `*.invowkmod` directories at the same level (not their dependencies)

Commands from all sources are aggregated and displayed with their **simple names**. The transparent namespace system means you don't need module prefixes unless there's a naming conflict.

```bash
# Directory structure:
# .
# ├── invowkfile.cue          (contains: build, deploy)
# ├── tools.invowkmod/        (contains: lint, deploy)
# └── testing.invowkmod/      (contains: test)

# Run commands with simple names (when unique)
invowk cmd build      # Runs build from invowkfile
invowk cmd lint       # Runs lint from tools.invowkmod
invowk cmd test       # Runs test from testing.invowkmod

# Ambiguous commands require disambiguation (deploy exists in both sources)
invowk cmd @invowkfile deploy      # Using @source prefix
invowk cmd @tools deploy         # Using @source prefix
invowk cmd --ivk-from invowkfile deploy  # Using --ivk-from flag
```

### Command Disambiguation

When a command name exists in multiple sources, invowk requires explicit disambiguation:

**Using `@source` prefix** (appears before the command name):
```bash
invowk cmd @invowkfile deploy           # Run deploy from invowkfile
invowk cmd @tools deploy              # Run deploy from tools.invowkmod
invowk cmd @tools.invowkmod deploy      # Full name also works
```

**Using `--ivk-from` flag** (must appear after `invowk cmd`):
```bash
invowk cmd --ivk-from invowkfile deploy
invowk cmd --ivk-from tools deploy
```

If you try to run an ambiguous command without disambiguation, invowk shows an error with available sources:
```
Error: 'deploy' is ambiguous. Found in:
  - @invowkfile: Deploy the application
  - @tools: Deploy to staging
Use 'invowk cmd @<source> deploy' or 'invowk cmd --ivk-from <source> deploy'
```

### Explicit Source for Non-Ambiguous Commands

You can always specify a source explicitly, even for unique command names:
```bash
invowk cmd @tools lint    # Works even though lint is not ambiguous
```

### Benefits of Transparent Namespaces

1. **Simple by default**: Use short command names when possible
2. **Explicit when needed**: Disambiguation syntax is clear and consistent
3. **Clear provenance**: Listing shows source for each command
4. **Tab completion**: Sources provide natural completion boundaries

### Command Dependencies and Namespaces

Command dependencies refer to other invowk commands by name. Invowk validates that the referenced commands are discoverable (it does not execute them automatically).

- Same invowkfile: use unqualified names like `build` or `test unit`
- Module commands: use the module prefix (or alias) like `tools deploy`

```cue
cmds: [
    {
        name: "release"
        depends_on: {
            cmds: [
                {alternatives: ["build"]},          // Same-file command
                {alternatives: ["test unit"]},      // Same-file nested command
                {alternatives: ["tools deploy"]},   // Command from a module
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

Require other invowk commands to be discoverable. Use the full command name as listed by `invowk cmd` (module prefix when applicable):

```cue
depends_on: {
	cmds: [
		{alternatives: ["clean"]},
		{alternatives: ["build"]},
		// Multiple alternatives - any one satisfies the dependency
		{alternatives: ["test unit", "test integration"]},
	]
}
```

### Filepath Dependencies

Check that required files or directories exist with proper permissions. You can specify multiple alternative paths where if any one exists, the dependency is satisfied:

```cue
depends_on: {
	filepaths: [
		// Simple existence check - any of these files satisfies the dependency
		{alternatives: ["go.mod", "go.sum", "Gopkg.lock"]},
		
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

### Environment Variable Dependencies

Validate that required environment variables exist in the user's environment before the command runs. This check happens **first**, before all other dependency checks, ensuring validation against the user's actual environment (not variables that invowk might set via the `env` construct).

```cue
depends_on: {
	env_vars: [
		// Simple check - just verify the env var exists
		{alternatives: [{name: "HOME"}]},
		
		// Multiple alternatives - any one satisfies the dependency
		{alternatives: [{name: "AWS_ACCESS_KEY_ID"}, {name: "AWS_PROFILE"}]},
		
		// With regex validation - env var must exist AND match the pattern
		{alternatives: [{name: "GO_VERSION", validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"}]},
	]
}
```

**Environment variable validation options:**
- `alternatives` (required): List of environment variable checks (OR semantics)
  - `name` (required): Environment variable name to check
  - `validation` (optional): Regex pattern the value must match

**Key behavior:**
- **OR semantics**: If any alternative exists (and passes validation if specified), the dependency is satisfied
- **Early validation**: Runs before tools, commands, filepaths, capabilities, and custom checks
- **User environment**: Validates against the user's actual environment, not variables set by invowk's `env` construct

**Example - AWS credentials check:**

```cue
cmds: [
	{
		name: "deploy"
		description: "Deploy to AWS"
		implementations: [
			{
				script: """
					echo "Deploying with AWS credentials..."
					aws s3 sync ./dist s3://my-bucket
					"""
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			}
		]
		depends_on: {
			env_vars: [
				// Require either AWS_ACCESS_KEY_ID or AWS_PROFILE for authentication
				{alternatives: [{name: "AWS_ACCESS_KEY_ID"}, {name: "AWS_PROFILE"}]},
			]
			tools: [
				{alternatives: ["aws"]},
			]
		}
	}
]
```

**Example - Version format validation:**

```cue
depends_on: {
	env_vars: [
		// GO_VERSION must be set and match semver format (e.g., "1.25.0")
		{alternatives: [{name: "GO_VERSION", validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"}]},
	]
}
```

When environment variable dependencies are not satisfied, invowk displays a styled error message:

```
✗ Dependencies not satisfied

Command 'deploy' has unmet dependencies:

Missing or Invalid Environment Variables:
  • AWS_ACCESS_KEY_ID - not set in environment

Install the missing tools and try again, or update your invowkfile to remove unnecessary dependencies.
```

### Capability Dependencies

Verify that required system capabilities are available. Invowk supports checking for network connectivity, container engine availability, and interactive TTY:

```cue
depends_on: {
	capabilities: [
		// Check for internet connectivity
		{alternatives: ["internet"]},

		// Check that Docker or Podman is installed and responding
		{alternatives: ["containers"]},

		// Check for interactive TTY
		{alternatives: ["tty"]},

		// OR semantics: either internet or LAN connectivity
		{alternatives: ["internet", "local-area-network"]},
	]
}
```

**Available capabilities:**

| Capability | Description |
|------------|-------------|
| `local-area-network` | Checks for LAN connectivity |
| `internet` | Checks for internet connectivity |
| `containers` | Checks that Docker or Podman is installed and responding |
| `tty` | Checks that invowk is running in an interactive TTY |

### Custom Check Dependencies

Write custom validation scripts for requirements that don't fit built-in dependency types. Check tool versions, configuration validity, or any other custom requirement:

```cue
depends_on: {
	custom_checks: [
		// Simple exit code check (passes if script exits with 0)
		{
			name: "docker-running"
			check_script: "docker info > /dev/null 2>&1"
		},

		// Exit code + output validation
		{
			name: "go-version"
			check_script: "go version"
			expected_code: 0
			expected_output: "go1\\.2[1-9]"  // Must be Go 1.21+
		},

		// Alternatives (OR semantics)
		{
			alternatives: [
				{
					name: "python-3.11"
					check_script: "python3 --version"
					expected_output: "^Python 3\\.11"
				},
				{
					name: "python-3.12"
					check_script: "python3 --version"
					expected_output: "^Python 3\\.12"
				},
			]
		},
	]
}
```

**Custom check properties:**

| Property | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Identifier for error messages |
| `check_script` | Yes | Script to execute for validation |
| `expected_code` | No | Expected exit code (default: 0) |
| `expected_output` | No | Regex pattern to match against script output |

## Command Flags

Commands can define flags that are passed at runtime. Flags are made available to scripts as environment variables with the `INVOWK_FLAG_` prefix.

### Defining Flags

```cue
cmds: [
    {
        name: "deploy"
        description: "Deploy the application"
        implementations: [
            {
                script: """
                    echo "Deploying to ${INVOWK_FLAG_ENV}..."
                    if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                        echo "DRY RUN - no changes made"
                    else
                        ./scripts/deploy.sh "$INVOWK_FLAG_ENV"
                    fi
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
        // Define flags for this command
        flags: [
            {name: "env", description: "Target environment"},
            {name: "dry-run", description: "Perform a dry run", default_value: "false"},
            {name: "retry-count", description: "Number of retries", default_value: "3"},
        ]
    }
]
```

### Flag Properties

| Property | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Flag name (POSIX-compliant: starts with letter, alphanumeric/hyphen/underscore) |
| `description` | Yes | Description shown in help text |
| `default_value` | No | Default value if flag is not provided (cannot be used with `required`) |
| `type` | No | Data type: `string` (default), `bool`, `int`, or `float` |
| `required` | No | If `true`, the flag must be provided (cannot have `default_value`) |
| `short` | No | Single-letter alias (e.g., `v` for `-v` shorthand) |
| `validation` | No | Regex pattern to validate flag values |

> **Reserved prefixes:** The `ivk-`, `invowk-`, and `i-` prefixes are reserved for system flags. Additionally, `help` and `version` are reserved built-in flag names.

### Typed Flags

Flags can specify a type for better validation:

```cue
flags: [
    {name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"},
    {name: "count", description: "Number of iterations", type: "int", default_value: "5"},
    {name: "threshold", description: "Confidence threshold", type: "float", default_value: "0.95"},
    {name: "message", description: "Custom message", type: "string"},
]
```

- **string** (default): Any value is accepted
- **bool**: Only `true` or `false` are accepted
- **int**: Only valid integers are accepted (including negative numbers)
- **float**: Only valid floating-point numbers are accepted (e.g., `3.14`, `-2.5`, `1.5e-3`)

### Required Flags

Mark a flag as required to ensure it must be provided:

```cue
flags: [
    {name: "target", description: "Deployment target", required: true},
    {name: "confirm", description: "Skip confirmation", type: "bool", default_value: "false"},
]
```

Required flags cannot have a `default_value`. If a required flag is not provided, the command will fail with an error.

### Short Aliases

Add single-letter shortcuts for frequently used flags:

```cue
flags: [
    {name: "verbose", description: "Enable verbose output", type: "bool", short: "v"},
    {name: "output", description: "Output file path", short: "o"},
    {name: "force", description: "Force overwrite", type: "bool", short: "f"},
]
```

Usage:
```bash
invowk cmd build -v -o=./dist/output.txt -f
# Equivalent to:
invowk cmd build --verbose --output=./dist/output.txt --force
```

### Validation Patterns

Use regex patterns to validate flag values:

```cue
flags: [
    {name: "env", description: "Environment", validation: "^(dev|staging|prod)$", default_value: "dev"},
    {name: "version", description: "Version (semver)", validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"},
]
```

If a value doesn't match the pattern, the command fails before execution:
```
Error: flag 'env' value 'invalid' does not match required pattern '^(dev|staging|prod)$'
```

### Complete Flag Example

Here's a command using all flag features:

```cue
cmds: [
    {
        name: "deploy"
        description: "Deploy the application"
        implementations: [...]
        flags: [
            {
                name:        "env"
                description: "Target environment"
                type:        "string"
                required:    true
                short:       "e"
                validation:  "^(dev|staging|prod)$"
            },
            {
                name:          "replicas"
                description:   "Number of replicas"
                type:          "int"
                short:         "n"
                default_value: "1"
            },
            {
                name:          "dry-run"
                description:   "Perform a dry run"
                type:          "bool"
                short:         "d"
                default_value: "false"
            },
        ]
    }
]
```

Usage:
```bash
invowk cmd deploy -e=prod -n=3 -d
```

### Using Flags

Flags are passed using standard `--flag=value` or `--flag value` syntax:

```bash
# Pass flags to a command
invowk cmd deploy --env=production --dry-run=true

# Use default values
invowk cmd deploy --env=staging  # dry-run defaults to "false"

# View flag help
invowk cmd deploy --help
```

### Environment Variable Naming

Flag names are converted to environment variables:
- Prefix: `INVOWK_FLAG_`
- Hyphens (`-`) become underscores (`_`)
- Converted to uppercase

| Flag Name | Environment Variable |
|-----------|---------------------|
| `env` | `INVOWK_FLAG_ENV` |
| `dry-run` | `INVOWK_FLAG_DRY_RUN` |
| `output-file` | `INVOWK_FLAG_OUTPUT_FILE` |
| `retry-count` | `INVOWK_FLAG_RETRY_COUNT` |

### Flags in Scripts

Access flag values in your scripts using the environment variables:

```bash
#!/bin/bash
# Access flags as environment variables
echo "Environment: $INVOWK_FLAG_ENV"
echo "Dry run: $INVOWK_FLAG_DRY_RUN"

# Conditional logic based on flags
if [ "$INVOWK_FLAG_VERBOSE" = "true" ]; then
    set -x  # Enable debug output
fi

# Use default value pattern if needed
RETRIES="${INVOWK_FLAG_RETRY_COUNT:-3}"
```

### Script-Level Dependencies

Dependencies can also be specified at the script level, which is especially useful for container-based implementations:

```cue
cmds: [
	{
		name: "docker-build"
		implementations: [
			{
				script: "go build -o /workspace/bin/app ./..."
				runtimes: [{name: "container", image: "golang:1.26"}]
				platforms: [{name: "linux"}]
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

## Command Arguments (Positional Arguments)

Commands can define typed positional arguments that are validated at runtime. Arguments are passed to scripts via `INVOWK_ARG_` environment variables.

### Defining Arguments

```cue
cmds: [
    {
        name: "deploy"
        description: "Deploy the application"
        implementations: [
            {
                script: """
                    echo "Deploying to ${INVOWK_ARG_ENV}..."
                    echo "Replicas: ${INVOWK_ARG_REPLICAS}"
                    echo "Services: ${INVOWK_ARG_SERVICES}"
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
        // Define positional arguments
        args: [
            {name: "env", description: "Target environment", required: true},
            {name: "replicas", description: "Number of replicas", type: "int", default_value: "1"},
            {name: "services", description: "Services to deploy", variadic: true},
        ]
    }
]
```

### Argument Properties

| Property | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Argument name (lowercase letters, numbers, hyphens only) |
| `description` | Yes | Description shown in help text |
| `required` | No | If `true`, the argument must be provided (cannot have `default_value`) |
| `default_value` | No | Default value if argument is not provided |
| `type` | No | Data type: `string` (default), `int`, or `float` |
| `validation` | No | Regex pattern to validate argument values |
| `variadic` | No | If `true`, accepts multiple values (must be last argument) |

### Typed Arguments

Arguments can specify a type for validation:

```cue
args: [
    {name: "env", description: "Target environment", type: "string"},
    {name: "replicas", description: "Number of replicas", type: "int", default_value: "3"},
    {name: "threshold", description: "Threshold value", type: "float", default_value: "0.5"},
]
```

- **string** (default): Any value is accepted
- **int**: Only valid integers are accepted
- **float**: Only valid floating-point numbers are accepted

### Required vs Optional Arguments

Arguments can be required or optional:

```cue
args: [
    // Required: must be provided
    {name: "env", description: "Target environment", required: true},
    
    // Optional with default: uses default if not provided
    {name: "replicas", description: "Number of replicas", default_value: "1"},
    
    // Optional without default: empty if not provided
    {name: "tag", description: "Image tag"},
]
```

**Rules:**
- Required arguments cannot have `default_value`
- Required arguments must come before optional arguments
- Only one variadic argument is allowed (must be last)

### Validation Patterns

Use regex patterns to validate argument values:

```cue
args: [
    {name: "env", description: "Environment", required: true, validation: "^(dev|staging|prod)$"},
    {name: "version", description: "Semantic version", validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"},
]
```

If a value doesn't match the pattern, the command fails with a styled error:
```
✗ Invalid argument value!

Command 'deploy' received an invalid value for argument 'env'.

Value:  "invalid"
Error:  value does not match pattern '^(dev|staging|prod)$'
```

### Variadic Arguments

The last argument can be variadic, accepting multiple values:

```cue
args: [
    {name: "env", description: "Target environment", required: true},
    {name: "services", description: "Services to deploy", variadic: true},
]
```

Usage:
```bash
invowk cmd deploy prod api web worker
```

This provides multiple environment variables:
- `INVOWK_ARG_SERVICES`: Space-joined values (`api web worker`)
- `INVOWK_ARG_SERVICES_COUNT`: Number of values (`3`)
- `INVOWK_ARG_SERVICES_1`, `INVOWK_ARG_SERVICES_2`, etc.: Individual values

### Using Arguments

Arguments are passed after the command name:

```bash
# Required argument only
invowk cmd deploy prod

# With optional argument
invowk cmd deploy prod 3

# With variadic arguments
invowk cmd deploy prod 3 api web worker

# View argument help
invowk cmd deploy --help
```

The help output shows argument documentation:
```
Usage:
  invowk cmd deploy <env> [replicas] [services]...

Arguments:
  env                  (required) - Target environment
  replicas             (default: "1") [int] - Number of replicas
  services             (optional) (variadic) - Services to deploy
```

### Environment Variable Naming

Argument names are converted to environment variables:
- Prefix: `INVOWK_ARG_`
- Hyphens (`-`) become underscores (`_`)
- Converted to uppercase

| Argument Name | Environment Variable |
|---------------|---------------------|
| `env` | `INVOWK_ARG_ENV` |
| `replica-count` | `INVOWK_ARG_REPLICA_COUNT` |
| `output-file` | `INVOWK_ARG_OUTPUT_FILE` |

### Arguments in Scripts

Invowk provides two ways to access command arguments in your scripts:

#### 1. Shell Positional Parameters (Recommended)

Arguments are passed as shell positional parameters (`$1`, `$2`, `$@`, etc.), allowing you to use native shell syntax:

```bash
#!/bin/bash
# Access arguments using standard shell syntax
echo "Environment: $1"
echo "Replicas: $2"
echo "All args: $@"
echo "Number of args: $#"

# Loop over all arguments
for arg in "$@"; do
    echo "Argument: $arg"
done
```

**Shell Compatibility:**

| Shell | Positional Access | Notes |
|-------|-------------------|-------|
| bash, sh, zsh | `$1`, `$2`, `$@`, `$#` | Standard POSIX syntax |
| PowerShell | `$args[0]`, `$args[1]` | Zero-indexed array |
| cmd.exe | N/A | Use environment variables instead |
| virtual (mvdan/sh) | `$1`, `$2`, `$@`, `$#` | Standard POSIX syntax |
| container | `$1`, `$2`, `$@`, `$#` | Standard POSIX syntax (uses /bin/sh) |

#### 2. Environment Variables

Arguments are also available as `INVOWK_ARG_*` environment variables, which work across all shells and runtimes:

```bash
#!/bin/bash
# Access arguments via environment variables
echo "Environment: $INVOWK_ARG_ENV"
echo "Replicas: $INVOWK_ARG_REPLICAS"

# Check if variadic args were provided
if [ -n "$INVOWK_ARG_SERVICES" ]; then
    echo "Services: $INVOWK_ARG_SERVICES"
    echo "Count: $INVOWK_ARG_SERVICES_COUNT"
    
    # Iterate over individual values
    for i in $(seq 1 $INVOWK_ARG_SERVICES_COUNT); do
        eval "SERVICE=\$INVOWK_ARG_SERVICES_$i"
        echo "Processing service: $SERVICE"
    done
fi
```

#### Choosing Between Methods

| Use Case | Recommended Method |
|----------|-------------------|
| Simple scripts | Positional parameters (`$1`, `$2`) |
| Complex arg handling | Environment variables (`INVOWK_ARG_*`) |
| Cross-platform scripts | Environment variables |
| Variadic arguments | Both work (env vars provide `_COUNT` and indexed access) |
| PowerShell scripts | Either `$args[0]` or environment variables |
| cmd.exe scripts | Environment variables only |

### Arguments with Flags

Commands can have both arguments and flags. Flags use `--name=value` syntax and can appear anywhere:

```bash
# Flags before arguments
invowk cmd deploy --dry-run prod 3

# Flags after arguments  
invowk cmd deploy prod 3 --verbose

# Flags mixed with arguments
invowk cmd deploy prod --dry-run 3 api web --verbose
```

### Complete Argument Example

```cue
cmds: [
    {
        name: "deploy"
        description: "Deploy services to an environment"
        implementations: [
            {
                script: """
                    echo "=== Deployment ==="
                    echo "Environment: $INVOWK_ARG_ENV"
                    echo "Replicas: $INVOWK_ARG_REPLICAS"
                    echo "Services: $INVOWK_ARG_SERVICES"
                    
                    if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                        echo "[DRY RUN] Would deploy..."
                    else
                        for i in $(seq 1 $INVOWK_ARG_SERVICES_COUNT); do
                            eval "SERVICE=\$INVOWK_ARG_SERVICES_$i"
                            echo "Deploying $SERVICE with $INVOWK_ARG_REPLICAS replicas..."
                        done
                    fi
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
        args: [
            {
                name:        "env"
                description: "Target environment"
                required:    true
                validation:  "^(dev|staging|prod)$"
            },
            {
                name:          "replicas"
                description:   "Number of replicas"
                type:          "int"
                default_value: "1"
            },
            {
                name:        "services"
                description: "Services to deploy"
                variadic:    true
            },
        ]
        flags: [
            {name: "dry-run", description: "Perform a dry run", type: "bool", default_value: "false"},
        ]
    }
]
```

Usage:
```bash
invowk cmd deploy prod 3 api web worker --dry-run
```

### Environment Variables in Nested Commands

When a command's script invokes another invowk command (e.g., `invowk cmd other-command`), the following environment variable behavior applies:

**Isolated Variables (NOT inherited by child commands):**
- `INVOWK_ARG_*` - Argument values
- `INVOWK_FLAG_*` - Flag values
- `ARGC`, `ARG1`, `ARG2`, etc. - Legacy positional argument variables

This isolation prevents the parent command's arguments and flags from accidentally leaking into child commands, which could cause unexpected behavior.

**Inherited Variables (standard UNIX behavior):**
- Variables defined in the `env` construct of a command or implementation
- Any other environment variables in the process environment

This follows standard UNIX semantics where child processes inherit their parent's environment. If you define `env: { MY_VAR: "value" }` in a command and that command calls another invowk command, the child will see `MY_VAR` in its environment. This is intentional and allows commands to set up environment context for nested invocations.

**Example:**

```cue
cmds: [
    {
        name: "parent"
        env: {vars: {SHARED_CONFIG: "/etc/app/config.yaml"}}  // Inherited by children
        implementations: [
            {
                script: """
                    echo "Parent's INVOWK_ARG_NAME: $INVOWK_ARG_NAME"  # "parent-value"
                    invowk cmd examples child  # Child will NOT see INVOWK_ARG_NAME
                    # But child WILL see SHARED_CONFIG
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            }
        ]
        args: [{name: "name", default_value: "parent-value"}]
    },
    {
        name: "child"
        implementations: [
            {
                script: """
                    echo "Child's INVOWK_ARG_NAME: ${INVOWK_ARG_NAME:-<not set>}"  # "<not set>"
                    echo "Child's SHARED_CONFIG: $SHARED_CONFIG"  # "/etc/app/config.yaml"
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            }
        ]
        args: [{name: "name", default_value: "child-value"}]
    },
]
```

### Arguments vs Subcommands

A command cannot have both positional arguments and subcommands. If a command defines `args` but also has subcommands (commands with the same prefix), invowk will fail with an error:

```
✖ Invalid command structure

Command 'deploy' has both args and subcommands in invowkfile.cue
  defined args: env
  subcommands: deploy status, deploy logs

Remove either the 'args' field or the subcommands to resolve this conflict.
```

This is enforced because CLI parsers interpret positional arguments after a command as potential subcommand names, making the combination ambiguous.

## Platform Compatibility

Every implementation must specify which operating systems it supports using the `platforms` field. At least one platform is required.

### Basic Platform Configuration

```cue
cmds: [
    {
        name: "build"
        description: "Build the project"
        implementations: [
            {
                script: "make build"
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
            }
        ]
    },
    {
        name: "clean"
        description: "Clean build artifacts"
        implementations: [
            {
                script: "rm -rf bin/"
                runtimes: [{name: "native"}]
                // Unix-only command
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
    },
]
```

### Supported Platform Values

| Value | Description |
|-------|-------------|
| `linux` | Linux operating systems |
| `macos` | macOS (Darwin) |
| `windows` | Windows |

### Platform-Specific Implementations

You can provide different implementations for different platforms:

```cue
cmds: [
    {
        name: "system info"
        description: "Display system information"
        implementations: [
            // Unix implementation (Linux and macOS)
            {
                script: """
                    echo "Hostname: $(hostname)"
                    echo "Kernel: $(uname -r)"
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            },
            // Windows implementation
            {
                script: """
                    echo Hostname: %COMPUTERNAME%
                    echo User: %USERNAME%
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        ]
    }
]
```

### Platform-Specific Environment Variables

Each platform can define its own environment variables by creating separate implementations for each platform:

```cue
cmds: [
    {
        name: "deploy"
        description: "Deploy with platform-specific config"
        implementations: [
            {
                script: "echo \"Platform: $PLATFORM_NAME, Config: $CONFIG_PATH\""
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}]
                env: {vars: {PLATFORM_NAME: "Linux", CONFIG_PATH: "/etc/app/config.yaml"}}
            },
            {
                script: "echo \"Platform: $PLATFORM_NAME, Config: $CONFIG_PATH\""
                runtimes: [{name: "native"}]
                platforms: [{name: "macos"}]
                env: {vars: {PLATFORM_NAME: "macOS", CONFIG_PATH: "/usr/local/etc/app/config.yaml"}}
            },
        ]
    }
]
```

### Command Listing

When you run `invowk cmd`, the supported platforms are displayed for each command:

```
Available Commands
  (* = default runtime)

From current directory:
  build - Build the project [native*] (linux, macos, windows)
  clean - Clean build artifacts [native*] (linux, macos)
  system info - Display system information [native*] (linux, macos, windows)
```

### Unsupported Platform Error

If you try to run a command on an unsupported platform, invowk displays a styled error message:

```
✗ Host not supported

Command 'clean' cannot run on this host.

Current host:     windows
Supported hosts:  linux, macos

This command is only available on the platforms listed above.
```

## Script Sources

Commands can use either **inline scripts** or **script files**:

### Inline Scripts

Use single-line or multi-line (with triple quotes `"""`) CUE strings:

```cue
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: """
					#!/bin/bash
					set -e
					echo "Building..."
					go build ./...
					"""
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},
]
```

### Script Files

Reference external script files using paths:

```cue
cmds: [
	// Relative to invowkfile location
	{
		name: "deploy"
		implementations: [
			{
				script: "./scripts/deploy.sh"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},
	// Just the filename (if it has a recognized extension)
	{
		name: "test"
		implementations: [
			{
				script: "run-tests.sh"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
	},
]
```

Recognized script extensions: `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

## Interpreter Support

By default, invowk executes scripts using a shell (`/bin/sh` for native, the container's default shell for containers). However, you can run scripts with other interpreters like Python, Ruby, Node.js, Perl, etc.

### Auto-Detection from Shebang (Default)

When a script starts with a shebang line (`#!/...`), invowk automatically detects and uses that interpreter:

```cue
cmds: [
    {
        name: "python-script"
        description: "Python script with auto-detected interpreter"
        implementations: [
            {
                script: """
                    #!/usr/bin/env python3
                    import sys
                    print(f"Hello from Python {sys.version_info.major}.{sys.version_info.minor}!")
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
    }
]
```

The shebang is parsed to extract both the interpreter and any arguments:
- `#!/usr/bin/env python3` → runs with `python3`
- `#!/usr/bin/env -S python3 -u` → runs with `python3 -u` (unbuffered)
- `#!/usr/bin/perl -w` → runs with `perl -w` (warnings enabled)

### Explicit Interpreter

You can explicitly specify an interpreter using the `interpreter` field in the runtime configuration:

```cue
cmds: [
    {
        name: "ruby-script"
        description: "Ruby script with explicit interpreter"
        implementations: [
            {
                // No shebang needed when interpreter is explicit
                script: """
                    puts "Hello from Ruby!"
                    puts "Ruby version: #{RUBY_VERSION}"
                    """
                runtimes: [{name: "native", interpreter: "ruby"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
    }
]
```

The explicit interpreter takes precedence over shebang detection.

### Interpreter in Containers

The interpreter feature works with container runtimes as well:

```cue
cmds: [
    {
        name: "container-python"
        description: "Python script in container"
        implementations: [
            {
                script: """
                    #!/usr/bin/env python3
                    import os
                    print("Hello from Python in a container!")
                    print(f"Working directory: {os.getcwd()}")
                    """
                runtimes: [{
                    name: "container"
                    image: "python:3-slim"
                    // interpreter auto-detected from shebang
                }]
                platforms: [{name: "linux"}]
            }
        ]
    },
    {
        name: "container-python-explicit"
        description: "Python script with explicit interpreter in container"
        implementations: [
            {
                // No shebang needed
                script: """
                    import sys
                    print(f"Python {sys.version}")
                    """
                runtimes: [{
                    name: "container"
                    image: "python:3-slim"
                    interpreter: "python3"
                }]
                platforms: [{name: "linux"}]
            }
        ]
    }
]
```

### Interpreter with Arguments

Positional arguments are passed to interpreter scripts just like shell scripts:

```cue
cmds: [
    {
        name: "greet-python"
        description: "Python script with arguments"
        implementations: [
            {
                script: """
                    #!/usr/bin/env python3
                    import sys
                    name = sys.argv[1] if len(sys.argv) > 1 else "World"
                    print(f"Hello, {name}!")
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
        args: [
            {name: "name", description: "Name to greet", default_value: "World"}
        ]
    }
]
```

### Fallback Behavior

When no shebang is found and no explicit interpreter is specified, invowk falls back to shell execution:
- **Native runtime**: Uses the system's default shell
- **Container runtime**: Uses `/bin/sh -c`

### Virtual Runtime Restriction

The `interpreter` field is **not supported** with the virtual runtime. The virtual runtime uses the built-in mvdan/sh interpreter and cannot execute Python, Ruby, or other interpreters. Attempting to use `interpreter` with virtual runtime will result in a validation error.

### Supported Interpreters

Any executable available in PATH (or in the container) can be used as an interpreter. Common examples:
- `python3`, `python`
- `ruby`
- `node`
- `perl`
- `php`
- `lua`
- `Rscript`
- Custom interpreters

## Modules

Modules are self-contained folders that bundle module metadata with command definitions and scripts for easy distribution and portability.

### What is a Module?

A module is a directory with the `.invowkmod` suffix that contains:
- Required `invowkmod.cue` at the root (module metadata and dependencies)
- Optional `invowkfile.cue` at the root (command definitions)
- Optional script files referenced by command implementations
- No nested modules (modules cannot contain other modules)

`invowkfile.cue` is optional for library-only modules that exist to declare dependencies.

### Module Naming

Module folder names follow these rules:
- Must end with `.invowkmod`
- The prefix (before `.invowkmod`) must:
  - Start with a letter (a-z, A-Z)
  - Contain only alphanumeric characters
  - Support dot-separated segments for namespacing
- Compatible with RDNS (Reverse Domain Name System) naming conventions

**Valid module names:**
- `mycommands.invowkmod`
- `com.example.mytools.invowkmod`
- `org.company.project.invowkmod`
- `Utils.invowkmod`

**Invalid module names:**
- `.hidden.invowkmod` (starts with dot)
- `my-commands.invowkmod` (contains hyphen)
- `my_commands.invowkmod` (contains underscore)
- `123commands.invowkmod` (starts with number)
- `com..example.invowkmod` (empty segment)

### Module Structure

```
com.example.mytools.invowkmod/
├── invowkmod.cue          # Required: module metadata + dependencies
├── invowkfile.cue         # Optional: command definitions
├── scripts/               # Optional: script files
│   ├── build.sh
│   ├── deploy.sh
│   └── utils/
│       └── helper.sh
└── templates/             # Optional: other resources
    └── config.yaml
```

### Script Paths in Modules

When referencing script files in a module's invowkfile, use paths relative to the module root with **forward slashes** for cross-platform compatibility:

```cue
// invowkfile.cue
cmds: [
    {
        name: "build"
        description: "Build the project"
        implementations: [
            {
                // Path relative to module root, using forward slashes
                script: "scripts/build.sh"
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
    },
    {
        name: "deploy"
        description: "Deploy the application"
        implementations: [
            {
                // Nested script path
                script: "scripts/utils/helper.sh"
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
    }
]
```

**Important:**
- Always use forward slashes (`/`) in script paths, even on Windows
- Paths are automatically converted to the native format at runtime
- Absolute paths are not allowed in modules
- Paths cannot escape the module directory (e.g., `../outside.sh` is invalid)

### Validating Modules

Use the `module validate` command to check a module's structure:

```bash
# Basic validation
invowk module validate ./com.example.mytools.invowkmod

# Deep validation (also parses the invowkfile)
invowk module validate ./com.example.mytools.invowkmod --deep
```

Example output for a valid module:
```
Module Validation
• Path: /home/user/com.example.mytools.invowkmod
• Name: com.example.mytools

✓ Module is valid

✓ Structure check passed
✓ Naming convention check passed
✓ Required files present
✓ Invowkfile parses successfully
```

Example output for an invalid module:
```
Module Validation
• Path: /home/user/invalid.invowkmod

✗ Module validation failed with 2 issue(s)

  1. [structure] missing required invowkmod.cue
  2. [structure] nested.invowkmod: nested modules are not allowed
```

### Creating Modules

Use the `module create` command to scaffold a new module:

```bash
# Create a simple module in the current directory
invowk module create mycommands

# Create a module with RDNS naming
invowk module create com.example.mytools

# Create a module in a specific directory
invowk module create mytools --path /path/to/modules

# Create with a scripts directory
invowk module create mytools --scripts

# Create with custom module identifier and description
invowk module create mytools --module-id "com.example.mytools" --description "A collection of useful commands"
```

The created module will contain `invowkmod.cue` metadata and a template `invowkfile.cue` with a sample "hello" command.

### Listing Modules

Use the `module list` command to see all discovered modules:

```bash
invowk module list
```

Example output:
```
Discovered Modules

• Found 3 module(s)

• current directory:
   ✓ mytools
      /home/user/project/mytools.invowkmod

• user commands (~/.invowk/cmds):
   ✓ com.example.utilities
      /home/user/.invowk/cmds/com.example.utilities.invowkmod
   ✓ org.company.tools
      /home/user/.invowk/cmds/org.company.tools.invowkmod
```

### Archiving Modules

Use the `module archive` command to create a ZIP archive for distribution:

```bash
# Archive a module (creates <module-name>.invowkmod.zip in current directory)
invowk module archive ./mytools.invowkmod

# Archive with custom output path
invowk module archive ./mytools.invowkmod --output ./dist/mytools.zip
```

Example output:
```
Archive Module

✓ Module archived successfully

• Output: /home/user/dist/mytools.zip
• Size: 2.45 KB
```

### Importing Modules

Use the `module import` command to install a module from a ZIP file or URL:

```bash
# Import from a local ZIP file (installs to ~/.invowk/cmds/)
invowk module import ./mytools.invowkmod.zip

# Import from a URL
invowk module import https://example.com/modules/mytools.zip

# Import to a custom directory
invowk module import ./module.zip --path ./local-modules

# Overwrite existing module
invowk module import ./module.zip --overwrite
```

Example output:
```
Import Module

✓ Module imported successfully

• Name: mytools
• Path: /home/user/.invowk/cmds/mytools.invowkmod

• The module commands are now available via invowk
```

### Benefits of Modules

1. **Portability**: Share a complete command set as a single folder
2. **Self-contained**: Module metadata and scripts travel together
3. **Cross-platform**: Forward slash paths work on all operating systems
4. **Namespace isolation**: RDNS naming prevents conflicts between modules
5. **Validation**: Built-in validation ensures module integrity

### Using Modules

Modules are automatically discovered and loaded from all configured sources:
1. Current directory (invowkfile and sibling modules, highest priority)
2. Configured includes (module paths from config)
3. `~/.invowk/cmds/` (modules only, non-recursive)

When invowk discovers a module, it:
- Validates the module structure and naming
- Loads `invowkfile.cue` from within the module (if present)
- Resolves script paths relative to the module root
- Makes all commands available with their module prefix

Commands from modules appear in `invowk cmd` with the source indicated as "module":

## Module Dependencies

Modules can declare dependencies on other modules hosted in remote Git repositories (GitHub, GitLab, etc.). This enables code reuse and sharing of common command definitions across projects.

### Declaring Dependencies

Add a `requires` field to `invowkmod.cue` to declare module dependencies:

```cue
// invowkmod.cue
module: "myproject"
version: "1.0.0"

// Declare module dependencies
requires: [
	{
		git_url: "https://github.com/user/common-tools.git"
		version: "^1.0.0"  // Compatible with 1.x.x
	},
	{
		git_url: "https://github.com/user/deploy-utils.git"
		version: "~2.1.0"  // Approximately 2.1.x
		alias:   "deploy"  // Custom namespace (for collision disambiguation)
	},
	{
		git_url: "https://github.com/user/monorepo.git"
		version: ">=1.0.0"
		path:    "packages/cli-tools"  // Subdirectory within repo
	},
]
```

The repository must contain a module (either a `.invowkmod` directory or an `invowkmod.cue` file at the root).

Commands in a module can only call commands from direct dependencies or globally installed modules (transitive dependencies are not available).

### Version Constraints

| Format | Description | Example |
|--------|-------------|---------|
| `^1.2.0` | Compatible with 1.x.x (>=1.2.0 <2.0.0) | Major version locked |
| `~1.2.0` | Approximately 1.2.x (>=1.2.0 <1.3.0) | Minor version locked |
| `>=1.0.0` | Greater than or equal | Minimum version |
| `<2.0.0` | Less than | Maximum version |
| `1.2.3` | Exact version | Pinned version |

### Module Dependency CLI Commands

```bash
# Add a new module dependency
invowk module add https://github.com/user/module.git ^1.0.0

# Add with custom alias (for collision disambiguation)
invowk module add https://github.com/user/module.git ^1.0.0 --alias myalias

# Add from monorepo subdirectory
invowk module add https://github.com/user/monorepo.git ^1.0.0 --path packages/tools

# List all resolved dependencies
invowk module deps

# Sync dependencies from invowkmod.cue (resolve and download)
invowk module sync

# Update all dependencies to latest matching versions
invowk module update

# Update a specific dependency
invowk module update https://github.com/user/module.git

# Remove a dependency
invowk module remove https://github.com/user/module.git
```

### Lock File

Module resolution creates an `invowkmod.lock.cue` file that records the exact versions resolved. This ensures reproducible builds across environments:

```cue
// invowkmod.lock.cue - Auto-generated lock file
// DO NOT EDIT MANUALLY

version: "1.0"
generated: "2025-01-12T10:30:00Z"

modules: {
	"github.com/user/common-tools": {
		git_url:          "https://github.com/user/common-tools.git"
		version:          "^1.0.0"
		resolved_version: "1.2.3"
		git_commit:       "abc123def456..."
		namespace:        "common-tools@1.2.3"
	}
}
```

### Command Namespacing

When dependency modules are installed or vendored, their commands are namespaced to prevent conflicts:

- **Default**: `<module-name>@<version>` (e.g., `common-tools@1.2.3`)
- **With alias**: Uses the specified alias (e.g., `deploy`)

Access dependency commands using the namespace:

```bash
# Run a command from a dependency
invowk cmd common-tools@1.2.3 build

# With alias
invowk cmd deploy production
```

### Authentication

Module dependencies support both HTTPS and SSH authentication:

- **SSH**: Uses keys from `~/.ssh/` (id_ed25519, id_rsa, id_ecdsa)
- **HTTPS**: Uses environment variables:
  - `GITHUB_TOKEN` for GitHub
  - `GITLAB_TOKEN` for GitLab
  - `GIT_TOKEN` for generic Git servers

### Module Cache

Module dependencies are cached in `~/.invowk/modules/` by default. Override with:

```bash
export INVOWK_MODULES_PATH=/custom/path/to/modules
```

Each module version is cached in a separate directory, allowing multiple versions to coexist.

### Vendoring

Modules can include their dependencies in an `invowk_modules/` subfolder for self-contained distribution:

```
mymodule.invowkmod/
├── invowkmod.cue
├── invowkfile.cue
├── scripts/
└── invowk_modules/              # Vendored dependencies
    ├── io.example.utils.invowkmod/
    │   ├── invowkmod.cue
    │   └── invowkfile.cue
    └── com.other.tools.invowkmod/
        ├── invowkmod.cue
        └── invowkfile.cue
```

Vendored modules are resolved first, before checking the global cache. Use the vendor commands:

```bash
# Fetch dependencies into invowk_modules/
invowk module vendor

# Update vendored modules
invowk module vendor --update

# Remove unused vendored modules
invowk module vendor --prune
```

> **Note:** `invowk module vendor` currently prints the dependencies that would be vendored. Fetching and pruning vendored modules is still being finalized.

### Collision Handling

When two modules have the same name, invowk reports a collision error with guidance on how to disambiguate using aliases:

```
module name collision: 'io.example.tools' defined in both
  '/path/to/module1.invowkmod' and '/path/to/module2.invowkmod'
  Use an alias to disambiguate:
    - For requires: add 'alias' field to the requirement in invowkmod.cue
    - For config includes: add 'alias' field to the include entry in config.cue
```

## Runtime Modes

### Native Runtime

Uses the system's default shell:
- **Linux/macOS**: Uses `$SHELL`, or falls back to `bash` → `sh`
- **Windows**: Uses `pwsh` → `powershell` → `cmd`

```cue
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: "go build ./..."
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
			},
		]
	},
]
```

### Virtual Runtime

Uses the built-in [mvdan/sh](https://github.com/mvdan/sh) shell interpreter. This provides a consistent POSIX-like shell experience across platforms.

```cue
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: """
					echo "Building..."
					go build ./...
					"""
				runtimes: [{name: "virtual"}]
				platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
			},
		]
	},
]
```

### Container Runtime

> ⚠️ **CRITICAL: Linux Containers Only**
>
> The container runtime **requires Linux-based container images** (e.g., `debian:stable-slim`).
>
> **NOT supported:**
> - **Alpine-based images** (`alpine:*`) - BusyBox's `ash` shell has incompatibilities with standard scripts
> - **Windows container images** (`mcr.microsoft.com/windows/*`) - No POSIX shell available
>
> **Platform compatibility:**
> - **Linux with Docker/Podman**: Works natively
> - **macOS with Docker Desktop**: Works (Docker Desktop uses Linux VMs)
> - **Windows with Docker Desktop**: Requires WSL2 backend with Linux containers mode
>
> Scripts are executed using `/bin/sh` inside the container. Windows containers and Alpine images lack the required shell compatibility.

Runs commands inside a Docker or Podman container. Requires an image or containerfile specification in the runtime config.

```cue
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				// Container config is specified in the runtime
				runtimes: [{
					name: "container",
					image: "golang:1.26",
					volumes: ["./data:/data"],
					ports: ["8080:8080"],
				}]
				platforms: [{name: "linux"}]
			}
		]
	},
]
```

Control host environment inheritance per runtime:

```cue
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				runtimes: [{
					name:              "container",
					image:             "debian:stable-slim",
					env_inherit_mode:  "allow",
					env_inherit_allow: ["TERM", "LANG"],
					env_inherit_deny:  ["AWS_SECRET_ACCESS_KEY"],
				}]
				platforms: [{name: "linux"}]
			}
		]
	},
]
```

### Host SSH Access from Containers

Container commands can optionally SSH back into the host system. When `enable_host_ssh: true` is set inside the container runtime configuration, invowk starts a secure SSH server using the [Wish](https://github.com/charmbracelet/wish) library and provides connection credentials to the container via environment variables.

**Security**: The SSH server only accepts token-based authentication. Each command execution gets a unique, time-limited token that is automatically revoked after the command completes.

```cue
cmds: [
	{
		name: "deploy from container"
		implementations: [
			{
				script: """
					# Connection info is available via environment variables:
					# - INVOWK_SSH_ENABLED: Set to "true" when host SSH is active
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
				runtimes: [{name: "container", image: "debian:stable-slim", enable_host_ssh: true}]
				platforms: [{name: "linux"}]
			}
		]
	},
]
```

**Note**: The container needs `sshpass` or similar tools to use password-based SSH authentication. You may need to install it in your container image.

## Configuration

invowk uses a global configuration file (CUE format):

- **Linux**: `~/.config/invowk/config.cue`
- **macOS**: `~/Library/Application Support/invowk/config.cue`
- **Windows**: `%APPDATA%\invowk\config.cue`

### Create Default Configuration

```bash
invowk config init
```

### View Current Configuration

```bash
invowk config show
```

### Configuration Options

```cue
// Container engine preference: "podman" or "docker"
container_engine: "podman"

// Default runtime mode: "native", "virtual", or "container"
default_runtime: "native"

// Include additional modules in command discovery
includes: [
    {path: "/home/user/my-tools.invowkmod"},
    {path: "/path/to/module.invowkmod", alias: "myalias"},
]

// Virtual shell options
virtual_shell: {
  enable_uroot_utils: true
}

// Container runtime options
container: {
  auto_provision: {
    enabled: true                         // Enable/disable auto-provisioning of invowk into containers (default: true)
    binary_path: "/usr/local/bin/invowk"  // Override path to invowk binary to provision (optional)
    includes: [{path: "/extra/modules.invowkmod"}] // Modules to provision into containers (optional)
    inherit_includes: true                // Inherit root-level includes for provisioning (default: true)
    cache_dir: "~/.cache/invowk/provision" // Cache directory for provisioned image metadata (optional)
  }
}

// UI options
ui: {
  color_scheme: "auto"    // "auto", "dark", "light"
  verbose: false
  interactive: false      // Enable alternate screen buffer mode for command execution
}
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
invowk cmd
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
invowk cmd build --ivk-runtime virtual
```

### Force Container Image Rebuild
```bash
invowk cmd build --ivk-force-rebuild
```

### Verbose Mode
```bash
invowk --ivk-verbose cmd build
```

### Interactive Mode (Alternate Screen Buffer)
```bash
invowk --ivk-interactive cmd build
# or
invowk -i cmd build
```

### Override Config File
```bash
invowk --ivk-config /path/to/custom/config.cue cmd build
```

### Environment Overrides
```bash
# Load additional env file at runtime
invowk cmd deploy --ivk-env-file .env.production

# Set an environment variable
invowk cmd deploy --ivk-env-var API_KEY=secret123

# Control host environment inheritance
invowk cmd build --ivk-env-inherit-mode allow --ivk-env-inherit-allow TERM --ivk-env-inherit-allow LANG
```

### Override Working Directory
```bash
invowk cmd test --ivk-workdir ./packages/api
```

## Interactive TUI Components

invowk includes a set of interactive terminal UI components inspired by [gum](https://github.com/charmbracelet/gum). These can be used in shell scripts to create interactive prompts, selections, and styled output.

### Input

Prompt for single-line text input:

```bash
# Basic input
invowk tui input --title "What is your name?"

# With placeholder
invowk tui input --title "Email" --placeholder "user@example.com"

# Password input (hidden)
invowk tui input --title "Password" --password

# With character limit
invowk tui input --title "Username" --char-limit 20

# Use in shell script
NAME=$(invowk tui input --title "Enter your name:")
echo "Hello, $NAME!"
```

### Write

Multi-line text editor for longer input:

```bash
# Basic editor
invowk tui write --title "Enter description"

# With line numbers
invowk tui write --title "Code" --show-line-numbers

# Use for commit messages
MESSAGE=$(invowk tui write --title "Commit message")
git commit -m "$MESSAGE"
```

### Choose

Select one or more options from a list:

```bash
# Single selection
invowk tui choose "Option 1" "Option 2" "Option 3"

# With title
invowk tui choose --title "Pick a color" red green blue

# Multi-select (up to 3)
invowk tui choose --limit 3 "One" "Two" "Three" "Four"

# Unlimited multi-select
invowk tui choose --no-limit "One" "Two" "Three"

# Use in shell script
COLOR=$(invowk tui choose --title "Pick a color" red green blue)
echo "You picked: $COLOR"
```

### Confirm

Yes/no confirmation prompt (exits with code 0 for yes, 1 for no):

```bash
# Basic confirmation
invowk tui confirm "Are you sure?"

# With custom labels
invowk tui confirm --affirmative "Delete" --negative "Cancel" "Delete this file?"

# Default to yes
invowk tui confirm --default "Proceed?"

# Use in shell conditionals
if invowk tui confirm "Continue?"; then
    echo "Continuing..."
else
    echo "Cancelled."
fi
```

### Filter

Fuzzy filter a list of options:

```bash
# Filter from arguments
invowk tui filter "apple" "banana" "cherry" "date"

# Filter from stdin
ls | invowk tui filter --title "Select a file"

# Multi-select filter
cat files.txt | invowk tui filter --no-limit

# With placeholder
invowk tui filter --placeholder "Type to search..." opt1 opt2 opt3
```

### File

File picker for browsing and selecting files:

```bash
# Pick any file from current directory
invowk tui file

# Start in specific directory
invowk tui file /home/user/documents

# Only show directories
invowk tui file --directory

# Show hidden files
invowk tui file --hidden

# Filter by extension
invowk tui file --allowed ".go,.md,.txt"
```

### Table

Display and select from tabular data:

```bash
# Display a CSV file
invowk tui table --file data.csv

# Pipe data with custom separator
echo -e "name|age|city\nAlice|30|NYC\nBob|25|LA" | invowk tui table --separator "|"

# Selectable rows (prints selected row)
cat data.csv | invowk tui table --selectable
```

### Spin

Show a spinner while running a command:

```bash
# Run a command with spinner
invowk tui spin --title "Installing..." -- npm install

# Different spinner types
invowk tui spin --type globe --title "Downloading..." -- curl -O https://example.com/file

# Available types: line, dot, minidot, jump, pulse, points, globe, moon, monkey, meter, hamburger, ellipsis
```

### Pager

Scroll through long content:

```bash
# View a file
invowk tui pager README.md

# Pipe content
cat long-file.txt | invowk tui pager

# With line numbers
invowk tui pager --line-numbers myfile.go

# With title
git log | invowk tui pager --title "Git History"
```

### Format

Format and render text:

```bash
# Format markdown
echo "# Hello World" | invowk tui format --type markdown

# Syntax highlight code
cat main.go | invowk tui format --type code --language go

# Convert emoji shortcodes
echo "Hello :wave: World :smile:" | invowk tui format --type emoji
```

### Style

Apply terminal styling to text:

```bash
# Colored text
invowk tui style --foreground "#FF0000" "Red text"

# Bold and italic
echo "Styled" | invowk tui style --bold --italic

# With background and padding
invowk tui style --background "#333" --foreground "#FFF" --padding-left 1 --padding-right 1 "Box"

# Centered with border
invowk tui style --border rounded --align center --width 40 "Centered Title"

# Multiple styles
invowk tui style --bold --foreground "#00FF00" --background "#000" "Matrix"
```

### Using TUI in Invowkfiles

The TUI components can be used within invowkfile scripts to create interactive commands:

```cue
cmds: [
    {
        name: "interactive setup"
        description: "Interactive project setup wizard"
        implementations: [
            {
                script: """
                    #!/bin/bash
                    NAME=$(invowk tui input --title "Project name:")
                    TYPE=$(invowk tui choose --title "Project type" cli library api)

                    if invowk tui confirm "Create project '$NAME' of type '$TYPE'?"; then
                        invowk tui spin --title "Creating project..." -- mkdir -p "$NAME"
                        echo "Project created!" | invowk tui style --foreground "#00FF00" --bold
                    fi
                    """
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        ]
    }
]
```

## Project Structure

```
invowk/
├── main.go                     # Entry point
├── cmd/invowk/                 # CLI commands (Cobra command tree)
│   ├── root.go                 # Root command and global flags
│   ├── cmd.go                  # cmd subcommand (command execution)
│   ├── cmd_discovery.go        # Dynamic command registration and discovery
│   ├── cmd_execute.go          # Command execution pipeline
│   ├── module.go               # module subcommand tree
│   ├── init.go                 # init command
│   ├── config.go               # config commands
│   ├── completion.go           # Shell completion generation
│   ├── tui.go                  # tui parent command
│   ├── tui_*.go                # TUI subcommands (input, write, choose, confirm, filter, file, table, spin, pager, format, style)
│   └── internal.go             # Hidden internal commands
├── internal/
│   ├── benchmark/              # Benchmarks for PGO profile generation
│   ├── config/                 # Configuration management with CUE schema
│   ├── container/              # Container engine abstraction (Docker, Podman, sandbox)
│   ├── core/serverbase/        # Shared server state machine base
│   ├── discovery/              # Invowkfile and module discovery
│   ├── issue/                  # Error types and ActionableError
│   ├── provision/              # Container provisioning (ephemeral layer attachment)
│   ├── runtime/                # Runtime implementations (native, virtual, container)
│   ├── sshserver/              # SSH server for host access from containers
│   ├── testutil/               # Test utilities
│   ├── tui/                    # TUI component library and interactive execution
│   ├── tuiserver/              # TUI server for interactive sessions
│   └── uroot/                  # u-root utilities for virtual shell built-ins
├── pkg/
│   ├── cueutil/                # Shared CUE parsing utilities
│   ├── invowkmod/                # Module validation and structure
│   ├── invowkfile/               # Invowkfile parsing and validation
│   └── platform/               # Cross-platform utilities
```

## Dependencies

**Core:**
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management
- [CUE](https://cuelang.org/) - Configuration language
- [mvdan/sh](https://github.com/mvdan/sh) - Virtual shell interpreter

**TUI & Styling:**
- [Lip Gloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Glamour](https://github.com/charmbracelet/glamour) - Markdown rendering
- [Huh](https://github.com/charmbracelet/huh) - Terminal forms and prompts
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Bubbletea](https://github.com/charmbracelet/bubbletea) - TUI framework

**SSH & PTY:**
- [Wish](https://github.com/charmbracelet/wish) - SSH server framework (for host SSH access from containers)
- [Charmbracelet SSH](https://github.com/charmbracelet/ssh) - SSH transport layer
- [creack/pty](https://github.com/creack/pty) - PTY handling

**Module Dependencies:**
- [go-git](https://github.com/go-git/go-git) - Git operations for remote module resolution

**Virtual Shell:**
- [u-root](https://github.com/u-root/u-root) - Core utilities for virtual shell built-ins (28 utilities: cat, cp, ls, grep, sort, tar, seq, etc.)

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for how to participate.

## License

This project is licensed under the Mozilla Public License 2.0 (MPL-2.0) - see the [LICENSE](LICENSE) file for details.

SPDX-License-Identifier: MPL-2.0

## Trademark

invowk™ is a trademark of Danilo Cominotti Marques. See [TRADEMARK.md](TRADEMARK.md) for usage guidelines.
