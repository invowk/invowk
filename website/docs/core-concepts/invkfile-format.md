---
sidebar_position: 1
---

# Invkfile Format

Invkfiles are written in [CUE](https://cuelang.org/), a powerful configuration language that's like JSON with superpowers. If you've never used CUE before, don't worry - it's intuitive and you'll pick it up quickly.

## Why CUE?

We chose CUE over YAML or JSON because:

- **Built-in validation** - CUE catches errors before you run anything
- **No indentation nightmares** - Unlike YAML, a misplaced space won't break everything
- **Comments!** - Yes, you can actually write comments
- **Type safety** - Invowk's schema ensures your invkfile is correct
- **Templating** - Reduce repetition with CUE's powerful templating

## Basic Syntax

CUE looks like JSON, but more readable:

```cue
// This is a comment
group: "myproject"
version: "1.0"

// Lists use square brackets
commands: [
    {
        name: "hello"
        description: "A greeting command"
    }
]

// Multi-line strings use triple quotes
script: """
    echo "Line 1"
    echo "Line 2"
    """
```

Key differences from JSON:
- No commas needed after fields (though they're allowed)
- Trailing commas are fine
- Comments with `//`
- Multi-line strings with `"""`

## Schema Overview

Every invkfile follows this structure:

```cue
// Root level
group: string           // Required: namespace prefix
version?: string        // Optional: invkfile version (e.g., "1.0")
description?: string    // Optional: what this file is about
default_shell?: string  // Optional: override default shell
workdir?: string        // Optional: default working directory
env?: #EnvConfig        // Optional: global environment config
depends_on?: #DependsOn // Optional: global dependencies

// Required: at least one command
commands: [...#Command]
```

The `?` suffix means a field is optional.

## The Group Field

The `group` is the most important field - it namespaces all your commands:

```cue
group: "myproject"

commands: [
    {name: "build"},
    {name: "test"},
]
```

These commands become:
- `myproject build`
- `myproject test`

### Group Naming Rules

- Must start with a letter
- Can contain letters, numbers
- Dots (`.`) create nested namespaces
- No hyphens, underscores, or spaces

**Valid:**
- `myproject`
- `my.project`
- `com.company.tools`
- `frontend`

**Invalid:**
- `my-project` (hyphen)
- `my_project` (underscore)
- `.project` (starts with dot)
- `123project` (starts with number)

### RDNS Naming

For packs or shared invkfiles, we recommend Reverse Domain Name System (RDNS) naming:

```cue
group: "com.company.devtools"
group: "io.github.username.project"
```

This prevents conflicts when combining multiple invkfiles.

## Commands Structure

Each command has this structure:

```cue
{
    name: string                 // Required: command name
    description?: string         // Optional: help text
    implementations: [...]       // Required: how to run the command
    flags?: [...]                // Optional: command flags
    args?: [...]                 // Optional: positional arguments
    env?: #EnvConfig             // Optional: environment config
    workdir?: string             // Optional: working directory
    depends_on?: #DependsOn      // Optional: dependencies
}
```

### Implementations

A command can have multiple implementations for different platforms/runtimes:

```cue
{
    name: "build"
    implementations: [
        // Unix implementation
        {
            script: "make build"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Windows implementation
        {
            script: "msbuild /p:Configuration=Release"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

Invowk automatically picks the right implementation for your platform.

## Scripts

Scripts can be inline or reference external files:

### Inline Scripts

```cue
// Single line
script: "echo 'Hello!'"

// Multi-line
script: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build ./...
    """
```

### External Script Files

```cue
// Relative to invkfile location
script: "./scripts/build.sh"

// Just the filename (recognized extensions)
script: "deploy.sh"
```

Recognized extensions: `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

## Complete Example

Here's a full-featured invkfile:

```cue
group: "myapp"
version: "1.0"
description: "Build and deployment commands for MyApp"

// Global environment
env: {
    vars: {
        APP_NAME: "myapp"
        LOG_LEVEL: "info"
    }
}

// Global dependencies
depends_on: {
    tools: [{alternatives: ["sh", "bash"]}]
}

commands: [
    {
        name: "build"
        description: "Build the application"
        implementations: [
            {
                script: """
                    echo "Building $APP_NAME..."
                    go build -o bin/$APP_NAME ./...
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
        depends_on: {
            tools: [{alternatives: ["go"]}]
            filepaths: [{alternatives: ["go.mod"]}]
        }
    },
    {
        name: "deploy"
        description: "Deploy to production"
        implementations: [
            {
                script: "./scripts/deploy.sh"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
        depends_on: {
            tools: [{alternatives: ["docker", "podman"]}]
            commands: [{alternatives: ["myapp build"]}]
        }
        flags: [
            {name: "env", description: "Target environment", required: true},
            {name: "dry-run", description: "Simulate deployment", type: "bool", default_value: "false"}
        ]
    }
]
```

## CUE Tips & Tricks

### Reduce Repetition

Use CUE's templating to avoid repetition:

```cue
// Define a template
_nativeUnix: {
    target: {
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }
}

commands: [
    {
        name: "build"
        implementations: [
            _nativeUnix & {script: "make build"}
        ]
    },
    {
        name: "test"
        implementations: [
            _nativeUnix & {script: "make test"}
        ]
    }
]
```

### Validation

Run your invkfile through the CUE validator:

```bash
cue vet invkfile.cue path/to/invkfile_schema.cue -d '#Invkfile'
```

Or just try to list commands - Invowk validates automatically:

```bash
invowk cmd --list
```

## Next Steps

- [Commands and Groups](./commands-and-groups) - Naming conventions and hierarchies
- [Implementations](./implementations) - Platforms, runtimes, and scripts
