---
sidebar_position: 3
---

# Implementations

Every command needs at least one **implementation** - the actual code that runs when you invoke the command. Implementations define *what* runs, *where* it runs (platform), and *how* it runs (runtime).

## Basic Structure

An implementation has three main parts:

```cue
{
    name: "build"
    implementations: [
        {
            // 1. The script to run
            script: "go build ./..."
            
            // 2. Target constraints (runtime + platform)
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        }
    ]
}
```

## Scripts

The `script` field contains the commands to execute. It can be inline or reference an external file.

### Inline Scripts

Single-line scripts are simple:

```cue
script: "echo 'Hello, World!'"
```

Multi-line scripts use triple quotes:

```cue
script: """
    #!/bin/bash
    set -e
    echo "Building..."
    go build -o bin/app ./...
    echo "Done!"
    """
```

### External Script Files

Reference a script file instead of inline code:

```cue
// Relative to invkfile location
script: "./scripts/build.sh"

// Just the filename (if it has a recognized extension)
script: "deploy.sh"
```

Recognized extensions: `.sh`, `.bash`, `.ps1`, `.bat`, `.cmd`, `.py`, `.rb`, `.pl`, `.zsh`, `.fish`

### When to Use External Scripts

- **Inline**: Quick, simple commands; keeps everything in one file
- **External**: Complex scripts; reusable across commands; easier to edit with syntax highlighting

## Target Constraints

The `target` field defines where and how the implementation runs.

### Runtimes

Every implementation must specify at least one runtime:

```cue
target: {
    runtimes: [
        {name: "native"},      // System shell
        {name: "virtual"},     // Built-in POSIX shell
        {name: "container", image: "alpine:latest"}  // Container
    ]
}
```

The **first runtime** is the default. Users can override with `--runtime`:

```bash
# Uses default runtime (first in list)
invowk cmd myproject build

# Override to use container runtime
invowk cmd myproject build --runtime container
```

See [Runtime Modes](../runtime-modes/overview) for details on each runtime.

### Platforms

Optionally restrict which operating systems the implementation supports:

```cue
target: {
    runtimes: [{name: "native"}]
    platforms: [
        {name: "linux"},
        {name: "macos"},
        {name: "windows"}
    ]
}
```

If `platforms` is omitted, the implementation works on all platforms.

Available platforms: `linux`, `macos`, `windows`

## Multiple Implementations

Commands can have multiple implementations for different scenarios:

### Platform-Specific Implementations

```cue
{
    name: "clean"
    implementations: [
        // Unix implementation
        {
            script: "rm -rf build/"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Windows implementation
        {
            script: "rmdir /s /q build"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

Invowk automatically selects the right implementation for the current platform.

### Runtime-Specific Implementations

```cue
{
    name: "build"
    implementations: [
        // Fast native build
        {
            script: "go build ./..."
            target: {
                runtimes: [{name: "native"}]
            }
        },
        // Reproducible container build
        {
            script: "go build -o /workspace/bin/app ./..."
            target: {
                runtimes: [{name: "container", image: "golang:1.21"}]
            }
        }
    ]
}
```

### Combined Platform + Runtime

```cue
{
    name: "build"
    implementations: [
        // Linux/macOS with multiple runtime options
        {
            script: "make build"
            target: {
                runtimes: [
                    {name: "native"},
                    {name: "container", image: "ubuntu:22.04"}
                ]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        // Windows native only
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

## Platform-Specific Environment

Platforms can define their own environment variables:

```cue
{
    name: "deploy"
    implementations: [
        {
            script: "echo \"Deploying to $PLATFORM with config at $CONFIG_PATH\""
            target: {
                runtimes: [{name: "native"}]
                platforms: [
                    {
                        name: "linux"
                        env: {
                            PLATFORM: "Linux"
                            CONFIG_PATH: "/etc/app/config.yaml"
                        }
                    },
                    {
                        name: "macos"
                        env: {
                            PLATFORM: "macOS"
                            CONFIG_PATH: "/usr/local/etc/app/config.yaml"
                        }
                    }
                ]
            }
        }
    ]
}
```

## Implementation-Level Settings

Implementations can have their own environment, working directory, and dependencies:

```cue
{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {
                runtimes: [{name: "native"}]
            }
            
            // Implementation-specific env
            env: {
                vars: {
                    NODE_ENV: "production"
                }
            }
            
            // Implementation-specific workdir
            workdir: "./frontend"
            
            // Implementation-specific dependencies
            depends_on: {
                tools: [{alternatives: ["node", "npm"]}]
                filepaths: [{alternatives: ["package.json"]}]
            }
        }
    ]
}
```

These override command-level settings when this implementation is selected.

## Implementation Selection

When you run a command, Invowk selects an implementation based on:

1. **Current platform** - Filters to implementations supporting your OS
2. **Requested runtime** - If `--runtime` specified, uses that; otherwise uses the default
3. **First match wins** - Uses the first implementation matching both criteria

### Selection Examples

Given this command:

```cue
{
    name: "build"
    implementations: [
        {
            script: "make build"
            target: {
                runtimes: [{name: "native"}, {name: "virtual"}]
                platforms: [{name: "linux"}, {name: "macos"}]
            }
        },
        {
            script: "msbuild"
            target: {
                runtimes: [{name: "native"}]
                platforms: [{name: "windows"}]
            }
        }
    ]
}
```

| Platform | Command | Selected |
|----------|---------|----------|
| Linux | `invowk cmd myproject build` | First impl, native runtime |
| Linux | `invowk cmd myproject build --runtime virtual` | First impl, virtual runtime |
| Windows | `invowk cmd myproject build` | Second impl, native runtime |
| Windows | `invowk cmd myproject build --runtime virtual` | Error: no matching impl |

## Command Listing

The `invowk cmd list` output shows available runtimes and platforms:

```
Available Commands
  (* = default runtime)

From current directory:
  myproject build - Build the project [native*, virtual] (linux, macos)
  myproject clean - Clean artifacts [native*] (linux, macos, windows)
  myproject docker-build - Container build [container*] (linux, macos, windows)
```

- `[native*, virtual]` - Supports native (default) and virtual runtimes
- `(linux, macos)` - Only available on Linux and macOS

## Using CUE Templates

Reduce repetition with CUE templates:

```cue
// Define reusable templates
_unixNative: {
    target: {
        runtimes: [{name: "native"}]
        platforms: [{name: "linux"}, {name: "macos"}]
    }
}

_allPlatforms: {
    target: {
        runtimes: [{name: "native"}]
    }
}

commands: [
    {
        name: "build"
        implementations: [
            _unixNative & {script: "make build"}
        ]
    },
    {
        name: "test"
        implementations: [
            _unixNative & {script: "make test"}
        ]
    },
    {
        name: "version"
        implementations: [
            _allPlatforms & {script: "cat VERSION"}
        ]
    }
]
```

## Best Practices

1. **Start simple** - One implementation is often enough
2. **Add platforms as needed** - Don't specify platforms unless you need platform-specific behavior
3. **Default runtime first** - Put the most common runtime first in the list
4. **Use templates** - Reduce repetition with CUE's templating
5. **Keep scripts focused** - One task per command; chain with dependencies

## Next Steps

- [Runtime Modes](../runtime-modes/overview) - Deep dive into native, virtual, and container runtimes
- [Dependencies](../dependencies/overview) - Declare what your implementations need
