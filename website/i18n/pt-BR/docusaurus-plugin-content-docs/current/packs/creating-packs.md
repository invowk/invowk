---
sidebar_position: 2
---

# Creating Packs

Learn how to create, structure, and organize packs for your commands.

## Quick Create

Use `pack create` to scaffold a new pack:

```bash
# Simple pack
invowk pack create mytools

# RDNS naming
invowk pack create com.company.devtools

# In specific directory
invowk pack create mytools --path /path/to/packs

# With scripts directory
invowk pack create mytools --scripts
```

## Generated Structure

Basic pack:
```
mytools.invkpack/
└── invkfile.cue
```

With `--scripts`:
```
mytools.invkpack/
├── invkfile.cue
└── scripts/
```

## Template Invkfile

The generated `invkfile.cue`:

```cue
group: "mytools"
version: "1.0"
description: "Commands for mytools"

commands: [
    {
        name: "hello"
        description: "Say hello"
        implementations: [
            {
                script: """
                    echo "Hello from mytools!"
                    """
                target: {
                    runtimes: [{name: "native"}]
                }
            }
        ]
    }
]
```

## Manual Creation

You can also create packs manually:

```bash
mkdir mytools.invkpack
touch mytools.invkpack/invkfile.cue
```

## Adding Scripts

### Inline vs External

Choose based on complexity:

```cue
commands: [
    // Simple: inline script
    {
        name: "quick"
        implementations: [{
            script: "echo 'Quick task'"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    
    // Complex: external script
    {
        name: "complex"
        implementations: [{
            script: "scripts/complex-task.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### Script Organization

```
mytools.invkpack/
├── invkfile.cue
└── scripts/
    ├── build.sh           # Main scripts
    ├── deploy.sh
    ├── test.sh
    └── lib/               # Shared utilities
        ├── logging.sh
        └── validation.sh
```

### Script Paths

Always use forward slashes:

```cue
// Good
script: "scripts/build.sh"
script: "scripts/lib/logging.sh"

// Bad - will fail on some platforms
script: "scripts\\build.sh"

// Bad - escapes pack directory
script: "../outside.sh"
```

## Environment Files

Include `.env` files in your pack:

```
mytools.invkpack/
├── invkfile.cue
├── .env                   # Default config
├── .env.example           # Template for users
└── scripts/
```

Reference them:

```cue
env: {
    files: [".env"]
}
```

## Documentation

Include a README for users:

```
mytools.invkpack/
├── invkfile.cue
├── README.md              # Usage instructions
├── CHANGELOG.md           # Version history
└── scripts/
```

## Real-World Examples

### Build Tools Pack

```
com.company.buildtools.invkpack/
├── invkfile.cue
├── scripts/
│   ├── build-go.sh
│   ├── build-node.sh
│   └── build-python.sh
├── templates/
│   ├── Dockerfile.go
│   ├── Dockerfile.node
│   └── Dockerfile.python
└── README.md
```

```cue
group: "com.company.buildtools"
version: "1.0"
description: "Standardized build tools"

commands: [
    {
        name: "go"
        description: "Build Go project"
        implementations: [{
            script: "scripts/build-go.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "node"
        description: "Build Node.js project"
        implementations: [{
            script: "scripts/build-node.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    },
    {
        name: "python"
        description: "Build Python project"
        implementations: [{
            script: "scripts/build-python.sh"
            target: {runtimes: [{name: "native"}]}
        }]
    }
]
```

### DevOps Pack

```
org.devops.k8s.invkpack/
├── invkfile.cue
├── scripts/
│   ├── deploy.sh
│   ├── rollback.sh
│   └── status.sh
├── manifests/
│   ├── deployment.yaml
│   └── service.yaml
└── .env.example
```

## Best Practices

1. **Use RDNS naming**: Prevents conflicts with other packs
2. **Keep scripts focused**: One task per script
3. **Include documentation**: README with usage examples
4. **Version your pack**: Use semantic versioning
5. **Forward slashes only**: Cross-platform compatibility
6. **Validate before sharing**: Run `pack validate --deep`

## Validating Your Pack

Before sharing, validate:

```bash
invowk pack validate mytools.invkpack --deep
```

See [Validating](./validating) for details.

## Next Steps

- [Validating](./validating) - Ensure pack integrity
- [Distributing](./distributing) - Share your pack
