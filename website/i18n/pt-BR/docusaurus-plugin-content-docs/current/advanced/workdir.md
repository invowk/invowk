---
sidebar_position: 2
---

# Working Directory

Control where your commands execute with the `workdir` setting. This is especially useful for monorepos and projects with complex directory structures.

## Default Behavior

By default, commands run in the current directory (where you invoked `invowk`).

## Setting Working Directory

### Command Level

```cue
{
    name: "build frontend"
    workdir: "./frontend"  // Run in frontend subdirectory
    implementations: [{
        script: "npm run build"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Implementation Level

```cue
{
    name: "build"
    implementations: [
        {
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
            workdir: "./web"  // This implementation runs in ./web
        },
        {
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
            workdir: "./api"  // This implementation runs in ./api
        }
    ]
}
```

### Root Level

```cue
group: "myproject"
workdir: "./src"  // All commands default to ./src

commands: [
    {
        name: "build"
        // Inherits workdir: ./src
        implementations: [...]
    },
    {
        name: "test"
        workdir: "./tests"  // Override to ./tests
        implementations: [...]
    }
]
```

## Path Types

### Relative Paths

Relative to the invkfile location:

```cue
workdir: "./frontend"
workdir: "../shared"
workdir: "src/app"
```

### Absolute Paths

Full system paths:

```cue
workdir: "/opt/myapp"
workdir: "/home/user/projects/myapp"
```

### Environment Variables

Expand variables in paths:

```cue
workdir: "${HOME}/projects/myapp"
workdir: "${PROJECT_ROOT}/src"
```

## Precedence

Implementation overrides command, which overrides root:

```cue
group: "myproject"
workdir: "./root"  // Default: ./root

commands: [
    {
        name: "build"
        workdir: "./command"  // Override: ./command
        implementations: [
            {
                script: "make"
                workdir: "./implementation"  // Final: ./implementation
                target: {runtimes: [{name: "native"}]}
            }
        ]
    }
]
```

## Monorepo Pattern

Perfect for monorepos with multiple packages:

```cue
group: "monorepo"

commands: [
    {
        name: "web build"
        workdir: "./packages/web"
        implementations: [{
            script: "npm run build"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "api build"
        workdir: "./packages/api"
        implementations: [{
            script: "go build ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "mobile build"
        workdir: "./packages/mobile"
        implementations: [{
            script: "flutter build"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

## Container Working Directory

For containers, the current directory is mounted to `/workspace`:

```cue
{
    name: "build"
    implementations: [{
        script: """
            pwd  # /workspace
            ls   # Shows your project files
            """
        target: {
            runtimes: [{name: "container", image: "alpine"}]
        }
    }]
}
```

With `workdir`, that subdirectory becomes the container's working directory:

```cue
{
    name: "build frontend"
    workdir: "./frontend"
    implementations: [{
        script: """
            pwd  # /workspace/frontend
            npm run build
            """
        target: {
            runtimes: [{name: "container", image: "node:20"}]
        }
    }]
}
```

## Cross-Platform Paths

Use forward slashes for cross-platform compatibility:

```cue
// Good - works everywhere
workdir: "./src/app"

// Avoid - Windows-specific
workdir: ".\\src\\app"
```

Invowk automatically converts to native path separators at runtime.

## Real-World Examples

### Frontend/Backend Split

```cue
commands: [
    {
        name: "start frontend"
        workdir: "./frontend"
        implementations: [{
            script: "npm run dev"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "start backend"
        workdir: "./backend"
        implementations: [{
            script: "go run ./cmd/server"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Test Organization

```cue
commands: [
    {
        name: "test unit"
        workdir: "./tests/unit"
        implementations: [{
            script: "pytest"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "test integration"
        workdir: "./tests/integration"
        implementations: [{
            script: "pytest"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "test e2e"
        workdir: "./tests/e2e"
        implementations: [{
            script: "cypress run"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Build in Subdirectory

```cue
{
    name: "build"
    workdir: "./src"
    implementations: [{
        script: """
            # Now in ./src
            go build -o ../bin/app ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Best Practices

1. **Use relative paths**: More portable across machines
2. **Forward slashes**: Cross-platform compatible
3. **Root level for defaults**: Override where needed
4. **Keep paths short**: Easier to understand

## Next Steps

- [Interpreters](./interpreters) - Use non-shell interpreters
- [Platform-Specific](./platform-specific) - Per-platform implementations
