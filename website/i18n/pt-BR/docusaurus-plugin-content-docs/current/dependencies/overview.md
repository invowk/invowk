---
sidebar_position: 1
---

# Dependencies Overview

Dependencies let you declare what your command needs before it runs. Invowk validates all dependencies upfront and provides clear error messages when something's missing.

## Why Declare Dependencies?

Without dependency checks:
```bash
$ invowk cmd myproject build
./scripts/build.sh: line 5: go: command not found
```

With dependency checks:
```
$ invowk cmd myproject build

✗ Dependencies not satisfied

Command 'build' has unmet dependencies:

Missing Tools:
  • go - not found in PATH

Install the missing tools and try again.
```

Much better! You know exactly what's wrong before anything runs.

## Dependency Types

Invowk supports six types of dependencies:

| Type | Checks For |
|------|------------|
| [tools](./tools) | Binaries in PATH |
| [filepaths](./filepaths) | Files or directories |
| [commands](./commands) | Other Invowk commands |
| [capabilities](./capabilities) | System capabilities (network) |
| [env_vars](./env-vars) | Environment variables |
| [custom_checks](./custom-checks) | Custom validation scripts |

## Basic Syntax

Dependencies are declared in the `depends_on` block:

```cue
{
    name: "build"
    depends_on: {
        tools: [
            {alternatives: ["go"]}
        ]
        filepaths: [
            {alternatives: ["go.mod"]}
        ]
    }
    implementations: [...]
}
```

## The Alternatives Pattern

Every dependency uses an `alternatives` list with **OR semantics**:

```cue
// ANY of these tools satisfies the dependency
tools: [
    {alternatives: ["podman", "docker"]}
]

// ANY of these files satisfies the dependency
filepaths: [
    {alternatives: ["config.yaml", "config.json", "config.toml"]}
]
```

If **any** alternative is found, the dependency is satisfied. Invowk uses early return - it stops checking as soon as one matches.

## Validation Order

Dependencies are validated in this order:

1. **env_vars** - Environment variables (checked first!)
2. **tools** - Binaries in PATH
3. **filepaths** - Files and directories
4. **capabilities** - System capabilities
5. **commands** - Other Invowk commands
6. **custom_checks** - Custom validation scripts

Environment variables are validated first, before Invowk sets any command-level environment. This ensures you're checking the user's actual environment.

## Scope Levels

Dependencies can be declared at three levels:

### Root Level (Global)

Applies to all commands in the invkfile:

```cue
group: "myproject"

depends_on: {
    tools: [{alternatives: ["git"]}]  // Required by all commands
}

commands: [...]
```

### Command Level

Applies to a specific command:

```cue
{
    name: "build"
    depends_on: {
        tools: [{alternatives: ["go"]}]  // Required by this command
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
            script: "go build ./..."
            target: {runtimes: [{name: "container", image: "golang:1.21"}]}
            depends_on: {
                // Validated INSIDE the container
                tools: [{alternatives: ["go"]}]
            }
        }
    ]
}
```

### Scope Inheritance

Dependencies are **combined** across levels:

```cue
group: "myproject"

// Root level: requires git
depends_on: {
    tools: [{alternatives: ["git"]}]
}

commands: [
    {
        name: "build"
        // Command level: also requires go
        depends_on: {
            tools: [{alternatives: ["go"]}]
        }
        implementations: [
            {
                script: "go build ./..."
                target: {runtimes: [{name: "native"}]}
                // Implementation level: also requires make
                depends_on: {
                    tools: [{alternatives: ["make"]}]
                }
            }
        ]
    }
]

// Effective dependencies for "build": git + go + make
```

## Runtime-Aware Validation

Dependencies are validated according to the runtime:

| Runtime | Dependencies Checked Against |
|---------|------------------------------|
| native | Host system |
| virtual | Virtual shell with built-ins |
| container | Inside the container |

This is powerful for container commands:

```cue
{
    name: "build"
    implementations: [
        {
            script: "go build ./..."
            target: {
                runtimes: [{name: "container", image: "golang:1.21"}]
            }
            depends_on: {
                // Checked INSIDE the container, not on host
                tools: [{alternatives: ["go"]}]
                filepaths: [{alternatives: ["/workspace/go.mod"]}]
            }
        }
    ]
}
```

## Error Messages

When dependencies aren't satisfied, Invowk shows a helpful error:

```
✗ Dependencies not satisfied

Command 'deploy' has unmet dependencies:

Missing Tools:
  • docker - not found in PATH
  • kubectl - not found in PATH

Missing Files:
  • Dockerfile - file not found

Missing Environment Variables:
  • AWS_ACCESS_KEY_ID - not set in environment

Install the missing tools and try again.
```

## Complete Example

Here's a command with multiple dependency types:

```cue
{
    name: "deploy"
    description: "Deploy to production"
    depends_on: {
        // Check environment first
        env_vars: [
            {alternatives: [{name: "AWS_ACCESS_KEY_ID"}, {name: "AWS_PROFILE"}]}
        ]
        // Check required tools
        tools: [
            {alternatives: ["docker", "podman"]},
            {alternatives: ["kubectl"]}
        ]
        // Check required files
        filepaths: [
            {alternatives: ["Dockerfile"]},
            {alternatives: ["k8s/deployment.yaml"]}
        ]
        // Check network connectivity
        capabilities: [
            {alternatives: ["internet"]}
        ]
        // Run other commands first
        commands: [
            {alternatives: ["myproject build"]},
            {alternatives: ["myproject test"]}
        ]
    }
    implementations: [
        {
            script: "./scripts/deploy.sh"
            target: {runtimes: [{name: "native"}]}
        }
    ]
}
```

## Next Steps

Learn about each dependency type in detail:

- [Tools](./tools) - Check for binaries in PATH
- [Filepaths](./filepaths) - Check for files and directories
- [Commands](./commands) - Require other commands to run first
- [Capabilities](./capabilities) - Check system capabilities
- [Environment Variables](./env-vars) - Check environment variables
- [Custom Checks](./custom-checks) - Write custom validation scripts
