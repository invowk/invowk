---
sidebar_position: 1
---

# Environment Overview

Invowk provides powerful environment variable management for your commands. Set variables, load from files, and control precedence across multiple levels.

## Quick Example

```cue
{
    name: "build"
    env: {
        // Load from .env files
        files: [".env", ".env.local?"]  // ? means optional
        
        // Set variables directly
        vars: {
            NODE_ENV: "production"
            BUILD_DATE: "$(date +%Y-%m-%d)"
        }
    }
    implementations: [{
        script: """
            echo "Building for $NODE_ENV"
            echo "Date: $BUILD_DATE"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Environment Sources

Variables come from multiple sources, in order of precedence (highest first):

1. **CLI flags** - `--env-var KEY=value`
2. **CLI env files** - `--env-file .env.custom`
3. **Implementation vars** - Implementation-level `env.vars`
4. **Implementation files** - Implementation-level `env.files`
5. **Command vars** - Command-level `env.vars`
6. **Command files** - Command-level `env.files`
7. **Root vars** - Root-level `env.vars`
8. **Root files** - Root-level `env.files`
9. **System environment** - Host's environment variables

Later sources don't override earlier ones.

## Scope Levels

### Root Level

Applies to all commands in the invkfile:

```cue
group: "myproject"

env: {
    vars: {
        PROJECT_NAME: "myproject"
    }
}

commands: [...]  // All commands get PROJECT_NAME
```

### Command Level

Applies to a specific command:

```cue
{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [...]
}
```

### Implementation Level

Applies to a specific implementation:

```cue
{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
        }
    ]
}
```

### Platform Level

Set variables per platform:

```cue
implementations: [{
    script: "echo $CONFIG_PATH"
    target: {
        runtimes: [{name: "native"}]
        platforms: [
            {
                name: "linux"
                env: {CONFIG_PATH: "/etc/myapp/config"}
            },
            {
                name: "macos"
                env: {CONFIG_PATH: "/usr/local/etc/myapp/config"}
            }
        ]
    }
}]
```

## Env Files

Load variables from `.env` files:

```cue
env: {
    files: [
        ".env",           // Required - fails if missing
        ".env.local?",    // Optional - suffix with ?
        ".env.${ENV}?",   // Interpolation - uses ENV variable
    ]
}
```

Files are loaded in order; later files override earlier ones.

See [Env Files](./env-files) for details.

## Environment Variables

Set variables directly:

```cue
env: {
    vars: {
        API_URL: "https://api.example.com"
        DEBUG: "true"
        VERSION: "1.0.0"
    }
}
```

See [Env Vars](./env-vars) for details.

## CLI Overrides

Override at runtime:

```bash
# Set a single variable
invowk cmd myproject build --env-var NODE_ENV=development

# Set multiple variables
invowk cmd myproject build -E NODE_ENV=dev -E DEBUG=true

# Load from a file
invowk cmd myproject build --env-file .env.local

# Combine
invowk cmd myproject build --env-file .env.local -E OVERRIDE=value
```

## Built-in Variables

Invowk provides these variables automatically:

| Variable | Description |
|----------|-------------|
| `INVOWK_CMD_NAME` | Full command name (e.g., `myproject build`) |
| `INVOWK_CMD_GROUP` | Command group (e.g., `myproject`) |
| `INVOWK_RUNTIME` | Current runtime (native, virtual, container) |
| `INVOWK_WORKDIR` | Working directory |

Plus flag and argument variables:
- `INVOWK_FLAG_*` - Flag values
- `INVOWK_ARG_*` - Argument values

## Container Environment

For container runtime, environment is passed into the container:

```cue
{
    name: "build"
    env: {
        vars: {
            BUILD_ENV: "container"
        }
    }
    implementations: [{
        script: "echo $BUILD_ENV"  // Available inside container
        target: {
            runtimes: [{name: "container", image: "alpine"}]
        }
    }]
}
```

## Nested Commands

When a command invokes another command, some variables are isolated:

**Isolated (NOT inherited):**
- `INVOWK_ARG_*`
- `INVOWK_FLAG_*`

**Inherited (normal UNIX behavior):**
- Variables from `env.vars`
- Platform-level variables
- System environment

This prevents parent command arguments from leaking into child commands.

## Next Steps

- [Env Files](./env-files) - Load from .env files
- [Env Vars](./env-vars) - Set variables directly
- [Precedence](./precedence) - Understand override order
