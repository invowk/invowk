---
sidebar_position: 3
---

# Your First Invkfile

Now that you've run your first command, let's build something more practical. We'll create an invkfile for a typical project with build, test, and deploy commands.

## Understanding the Structure

An invkfile has a simple structure:

```cue
group: "myproject"           // Required: namespace for your commands
version: "1.0"               // Optional: version of this invkfile
description: "My commands"   // Optional: what this file is about

commands: [                  // Required: list of commands
    // ... your commands here
]
```

The `group` is mandatory and becomes the prefix for all your commands. Think of it as a namespace that keeps your commands organized and prevents collisions with commands from other invkfiles.

## A Real-World Example

Let's create an invkfile for a Go project:

```cue
group: "goproject"
version: "1.0"
description: "Commands for my Go project"

commands: [
    // Simple build command
    {
        name: "build"
        description: "Build the project"
        implementations: [
            {
                script: """
                    echo "Building..."
                    go build -o bin/app ./...
                    echo "Done! Binary at bin/app"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },

    // Test command with subcommand-style naming
    {
        name: "test unit"
        description: "Run unit tests"
        implementations: [
            {
                script: "go test -v ./..."
                target: {
                    runtimes: [{name: "native"}, {name: "virtual"}]
                }
            }
        ]
    },

    // Test with coverage
    {
        name: "test coverage"
        description: "Run tests with coverage"
        implementations: [
            {
                script: """
                    go test -coverprofile=coverage.out ./...
                    go tool cover -html=coverage.out -o coverage.html
                    echo "Coverage report: coverage.html"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    },

    // Clean command
    {
        name: "clean"
        description: "Remove build artifacts"
        implementations: [
            {
                script: "rm -rf bin/ coverage.out coverage.html"
                target: {
                    runtimes: [{name: "native"}]
                    platforms: [{name: "linux"}, {name: "macos"}]
                }
            }
        ]
    }
]
```

Save this as `invkfile.cue` and try:

```bash
invowk cmd --list
```

You'll see:

```
Available Commands
  (* = default runtime)

From current directory:
  goproject build - Build the project [native*]
  goproject test unit - Run unit tests [native*, virtual]
  goproject test coverage - Run tests with coverage [native*]
  goproject clean - Remove build artifacts [native*] (linux, macos)
```

## Subcommand-Style Names

Notice how `test unit` and `test coverage` create a hierarchy. You run them like:

```bash
invowk cmd goproject test unit
invowk cmd goproject test coverage
```

This is just naming convention - spaces in the name create a subcommand feel. It's great for organizing related commands!

## Multiple Runtimes

The `test unit` command allows both `native` and `virtual` runtimes:

```cue
runtimes: [{name: "native"}, {name: "virtual"}]
```

The first one is the default. You can override it:

```bash
# Use the default (native)
invowk cmd goproject test unit

# Explicitly use virtual runtime
invowk cmd goproject test unit --runtime virtual
```

## Platform-Specific Commands

The `clean` command only works on Linux and macOS (because it uses `rm -rf`):

```cue
platforms: [{name: "linux"}, {name: "macos"}]
```

If you try to run it on Windows, Invowk will show a helpful error message explaining the command isn't available on your platform.

## Adding Dependencies

Let's make our build command smarter by checking if Go is installed:

```cue
{
    name: "build"
    description: "Build the project"
    implementations: [
        {
            script: """
                echo "Building..."
                go build -o bin/app ./...
                echo "Done!"
                """
            target: {
                runtimes: [{name: "native"}]
            }
        }
    ]
    depends_on: {
        tools: [
            {alternatives: ["go"]}
        ]
        filepaths: [
            {alternatives: ["go.mod"], readable: true}
        ]
    }
}
```

Now if you run `invowk cmd goproject build` without Go installed, you'll get:

```
✗ Dependencies not satisfied

Command 'build' has unmet dependencies:

Missing Tools:
  • go - not found in PATH

Install the missing tools and try again.
```

## Environment Variables

You can set environment variables at different levels:

```cue
group: "goproject"

// Root-level env applies to ALL commands
env: {
    vars: {
        GO111MODULE: "on"
    }
}

commands: [
    {
        name: "build"
        // Command-level env applies to this command
        env: {
            vars: {
                CGO_ENABLED: "0"
            }
        }
        implementations: [
            {
                script: "go build -o bin/app ./..."
                // Implementation-level env is most specific
                env: {
                    vars: {
                        GOOS: "linux"
                        GOARCH: "amd64"
                    }
                }
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]
```

Environment variables merge with later levels overriding earlier ones.

## What's Next?

You now know the basics of creating invkfiles! Continue learning about:

- [Core Concepts](../core-concepts/invkfile-format) - Deep dive into the invkfile format
- [Runtime Modes](../runtime-modes/overview) - Learn about native, virtual, and container runtimes
- [Dependencies](../dependencies/overview) - All the ways to declare dependencies
