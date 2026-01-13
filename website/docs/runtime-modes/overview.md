---
sidebar_position: 1
---

# Runtime Modes Overview

Invowk gives you three different ways to execute commands, each with its own strengths. Choose the right runtime for your use case.

## The Three Runtimes

| Runtime | Description | Best For |
|---------|-------------|----------|
| **native** | System's default shell | Daily development, performance |
| **virtual** | Built-in POSIX shell | Cross-platform scripts, portability |
| **container** | Docker/Podman container | Reproducibility, isolation |

## Quick Comparison

```cue
commands: [
    // Native: uses your system shell (bash, zsh, PowerShell, etc.)
    {
        name: "build native"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Virtual: uses built-in POSIX-compatible shell
    {
        name: "build virtual"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "virtual"}]}
        }]
    },
    
    // Container: runs inside a container
    {
        name: "build container"
        implementations: [{
            script: "go build -o /workspace/bin/app ./..."
            target: {runtimes: [{name: "container", image: "golang:1.21"}]}
        }]
    }
]
```

## When to Use Each Runtime

### Native Runtime

Use **native** when you want:
- Maximum performance
- Access to all system tools
- Shell-specific features (bash completions, zsh plugins)
- Integration with your development environment

```cue
target: {runtimes: [{name: "native"}]}
```

### Virtual Runtime

Use **virtual** when you want:
- Consistent behavior across platforms
- POSIX-compatible scripts that work everywhere
- No external shell dependency
- Simpler debugging of shell scripts

```cue
target: {runtimes: [{name: "virtual"}]}
```

### Container Runtime

Use **container** when you want:
- Reproducible builds
- Isolated environments
- Specific tool versions
- Clean-room execution

```cue
target: {runtimes: [{name: "container", image: "golang:1.21"}]}
```

## Multiple Runtimes Per Command

Commands can support multiple runtimes. The first one is the default:

```cue
{
    name: "build"
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [
                {name: "native"},  // Default
                {name: "virtual"}, // Alternative
                {name: "container", image: "golang:1.21"}  // Reproducible
            ]
        }
    }]
}
```

### Overriding at Runtime

```bash
# Use default (native)
invowk cmd myproject build

# Override to virtual
invowk cmd myproject build --runtime virtual

# Override to container
invowk cmd myproject build --runtime container
```

## Command Listing

The command list shows available runtimes with an asterisk marking the default:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*, virtual, container] (linux, macos)
```

## Runtime Selection Flow

```
                    ┌──────────────────┐
                    │  invowk cmd run  │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │ --runtime flag?  │
                    └────────┬─────────┘
                             │
              ┌──────────────┴──────────────┐
              │ Yes                         │ No
              ▼                             ▼
    ┌─────────────────────┐    ┌─────────────────────┐
    │ Use specified       │    │ Use first runtime   │
    │ runtime             │    │ (default)           │
    └─────────────────────┘    └─────────────────────┘
```

## Dependency Validation

Dependencies are validated according to the runtime:

| Runtime | Dependencies Validated Against |
|---------|-------------------------------|
| native | Host system's shell and tools |
| virtual | Built-in shell with core utilities |
| container | Container's shell and environment |

This means a `tools` dependency like `go` is checked:
- **native**: Is `go` in the host's PATH?
- **virtual**: Is `go` available in the virtual shell's built-ins?
- **container**: Is `go` installed in the container image?

## Next Steps

Dive deeper into each runtime:

- [Native Runtime](./native) - System shell execution
- [Virtual Runtime](./virtual) - Built-in POSIX shell
- [Container Runtime](./container) - Docker/Podman execution
