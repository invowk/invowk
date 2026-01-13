---
sidebar_position: 4
---

# Command Dependencies

Command dependencies ensure that other Invowk commands run successfully before your command executes.

## Basic Usage

```cue
{
    name: "deploy"
    depends_on: {
        commands: [
            {alternatives: ["myproject build"]}
        ]
    }
    implementations: [...]
}
```

When you run `deploy`, Invowk first runs `build`. If `build` fails, `deploy` won't run.

## Full Command Names

Always use the **full group-prefixed name**:

```cue
group: "myproject"

commands: [
    {
        name: "build"
        implementations: [...]
    },
    {
        name: "test"
        depends_on: {
            commands: [
                // Must include group prefix "myproject"
                {alternatives: ["myproject build"]}
            ]
        }
        implementations: [...]
    }
]
```

## Alternatives (OR Semantics)

Specify alternatives when any command will work:

```cue
depends_on: {
    commands: [
        // Either a debug OR release build
        {alternatives: ["myproject build debug", "myproject build release"]},
        
        // Either unit OR integration tests
        {alternatives: ["myproject test unit", "myproject test integration"]},
    ]
}
```

The dependency is satisfied if **any** alternative has run successfully.

## Multiple Command Requirements

Each entry is an AND requirement:

```cue
depends_on: {
    commands: [
        // Need build AND test AND lint
        {alternatives: ["myproject build"]},
        {alternatives: ["myproject test"]},
        {alternatives: ["myproject lint"]},
    ]
}
```

All three commands must succeed (though alternatives within each are OR).

## Command Chains

Build complex workflows with command chains:

```cue
group: "myproject"

commands: [
    // Base command
    {
        name: "clean"
        implementations: [{
            script: "rm -rf build/"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Depends on clean
    {
        name: "build"
        depends_on: {
            commands: [{alternatives: ["myproject clean"]}]
        }
        implementations: [{
            script: "go build -o build/app ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Depends on build
    {
        name: "test"
        depends_on: {
            commands: [{alternatives: ["myproject build"]}]
        }
        implementations: [{
            script: "go test ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Depends on test (which depends on build, which depends on clean)
    {
        name: "release"
        depends_on: {
            commands: [{alternatives: ["myproject test"]}]
        }
        implementations: [{
            script: "./scripts/release.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

Running `release` executes: `clean` → `build` → `test` → `release`

## Cross-Invkfile Dependencies

Commands can depend on commands from other invkfiles:

```cue
// In frontend/invkfile.cue
group: "frontend"

commands: [
    {
        name: "build"
        depends_on: {
            commands: [
                // Depends on a command from the shared invkfile
                {alternatives: ["shared generate-types"]}
            ]
        }
        implementations: [...]
    }
]
```

```cue
// In shared/invkfile.cue
group: "shared"

commands: [
    {
        name: "generate-types"
        implementations: [...]
    }
]
```

## Real-World Examples

### CI/CD Pipeline

```cue
group: "myapp"

commands: [
    {name: "lint", implementations: [...]},
    {name: "test unit", implementations: [...]},
    {name: "test integration", implementations: [...]},
    {name: "build", implementations: [...]},
    
    {
        name: "ci"
        description: "Run full CI pipeline"
        depends_on: {
            commands: [
                {alternatives: ["myapp lint"]},
                {alternatives: ["myapp test unit"]},
                {alternatives: ["myapp test integration"]},
                {alternatives: ["myapp build"]},
            ]
        }
        implementations: [{
            script: "echo 'CI passed!'"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Build Variants

```cue
group: "myapp"

commands: [
    {
        name: "build debug"
        implementations: [{
            script: "go build -gcflags='all=-N -l' -o build/app-debug ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "build release"
        implementations: [{
            script: "go build -ldflags='-s -w' -o build/app ./..."
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    {
        name: "deploy"
        depends_on: {
            commands: [
                // Must have run either debug or release build
                {alternatives: ["myapp build debug", "myapp build release"]}
            ]
        }
        implementations: [{
            script: "./scripts/deploy.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Microservices

```cue
group: "monorepo"

commands: [
    {
        name: "api build"
        implementations: [...]
    },
    {
        name: "web build"
        implementations: [...]
    },
    {
        name: "deploy all"
        depends_on: {
            commands: [
                {alternatives: ["monorepo api build"]},
                {alternatives: ["monorepo web build"]},
            ]
        }
        implementations: [{
            script: "./scripts/deploy-all.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

## Execution Flow

When a command with dependencies runs:

```
invowk cmd myproject deploy
       │
       ▼
┌──────────────────┐
│ Check all deps   │
│ (commands list)  │
└────────┬─────────┘
         │
    ┌────┴────┐
    ▼         ▼
┌───────┐ ┌───────┐
│ build │ │ test  │
└───┬───┘ └───┬───┘
    │         │
    └────┬────┘
         │
         ▼
    ┌─────────┐
    │ deploy  │
    └─────────┘
```

Dependencies run in the order specified. If any fails, execution stops.

## Runtime Inheritance

Dependent commands use their own runtime settings:

```cue
{
    name: "build"
    implementations: [{
        script: "go build ./..."
        target: {runtimes: [{name: "container", image: "golang:1.21"}]}
    }]
}

{
    name: "deploy"
    depends_on: {
        commands: [{alternatives: ["myproject build"]}]
    }
    implementations: [{
        script: "./scripts/deploy.sh"
        target: {runtimes: [{name: "native"}]}
    }]
}
```

When you run `deploy`:
1. `build` runs in a container
2. `deploy` runs natively

Each command uses its own configuration.

## Best Practices

1. **Use full names**: Always include the group prefix
2. **Keep chains shallow**: Deep chains can be hard to debug
3. **Use alternatives wisely**: For truly interchangeable commands
4. **Consider parallelism**: Independent commands could run in parallel (future feature)

## Next Steps

- [Capabilities](./capabilities) - Check system capabilities
- [Environment Variables](./env-vars) - Check for required env vars
