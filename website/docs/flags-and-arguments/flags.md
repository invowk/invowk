---
sidebar_position: 2
---

# Flags

Flags are named options passed with `--name=value` syntax. They're ideal for optional settings, boolean switches, and configuration that doesn't follow a strict order.

## Defining Flags

```cue
{
    name: "deploy"
    flags: [
        {
            name: "environment"
            description: "Target environment"
            required: true
        },
        {
            name: "dry-run"
            description: "Simulate without changes"
            type: "bool"
            default_value: "false"
        },
        {
            name: "replicas"
            description: "Number of replicas"
            type: "int"
            default_value: "1"
        }
    ]
    implementations: [...]
}
```

## Flag Properties

| Property | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Flag name (alphanumeric, hyphens, underscores) |
| `description` | Yes | Help text |
| `type` | No | `string`, `bool`, `int`, `float` (default: `string`) |
| `default_value` | No | Default if not provided |
| `required` | No | Must be provided (can't have default) |
| `short` | No | Single-letter alias |
| `validation` | No | Regex pattern for value |

## Types

### String (Default)

```cue
{name: "message", description: "Custom message", type: "string"}
// or simply
{name: "message", description: "Custom message"}
```

```bash
invowk cmd myproject run --message="Hello World"
```

### Boolean

```cue
{name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"}
```

```bash
# Enable
invowk cmd myproject run --verbose
invowk cmd myproject run --verbose=true

# Disable (explicit)
invowk cmd myproject run --verbose=false
```

Boolean flags only accept `true` or `false`.

### Integer

```cue
{name: "count", description: "Number of iterations", type: "int", default_value: "5"}
```

```bash
invowk cmd myproject run --count=10
invowk cmd myproject run --count=-1  # Negative allowed
```

### Float

```cue
{name: "threshold", description: "Confidence threshold", type: "float", default_value: "0.95"}
```

```bash
invowk cmd myproject run --threshold=0.8
invowk cmd myproject run --threshold=1.5e-3  # Scientific notation
```

## Required vs Optional

### Required Flags

```cue
{
    name: "target"
    description: "Deployment target"
    required: true  // Must be provided
}
```

```bash
# Error: missing required flag
invowk cmd myproject deploy
# Error: flag 'target' is required

# Success
invowk cmd myproject deploy --target=production
```

Required flags cannot have a `default_value`.

### Optional Flags

```cue
{
    name: "timeout"
    description: "Request timeout in seconds"
    type: "int"
    default_value: "30"  // Used if not provided
}
```

```bash
# Uses default (30)
invowk cmd myproject request

# Override
invowk cmd myproject request --timeout=60
```

## Short Aliases

Add single-letter shortcuts:

```cue
flags: [
    {name: "verbose", description: "Verbose output", type: "bool", short: "v"},
    {name: "output", description: "Output file", short: "o"},
    {name: "force", description: "Force overwrite", type: "bool", short: "f"},
]
```

```bash
# Long form
invowk cmd myproject build --verbose --output=./dist --force

# Short form
invowk cmd myproject build -v -o=./dist -f

# Mixed
invowk cmd myproject build -v --output=./dist -f
```

## Validation Patterns

Validate flag values with regex:

```cue
flags: [
    {
        name: "env"
        description: "Environment name"
        validation: "^(dev|staging|prod)$"
        default_value: "dev"
    },
    {
        name: "version"
        description: "Semantic version"
        validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
    }
]
```

```bash
# Valid
invowk cmd myproject deploy --env=prod --version=1.2.3

# Invalid - fails before execution
invowk cmd myproject deploy --env=production
# Error: flag 'env' value 'production' does not match required pattern '^(dev|staging|prod)$'
```

## Accessing in Scripts

Flags are available as `INVOWK_FLAG_*` environment variables:

```cue
{
    name: "deploy"
    flags: [
        {name: "env", description: "Environment", required: true},
        {name: "dry-run", description: "Dry run", type: "bool", default_value: "false"},
        {name: "replica-count", description: "Replicas", type: "int", default_value: "1"},
    ]
    implementations: [{
        script: """
            echo "Environment: $INVOWK_FLAG_ENV"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Replicas: $INVOWK_FLAG_REPLICA_COUNT"
            
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                echo "Would deploy $INVOWK_FLAG_REPLICA_COUNT replicas to $INVOWK_FLAG_ENV"
            else
                ./deploy.sh "$INVOWK_FLAG_ENV" "$INVOWK_FLAG_REPLICA_COUNT"
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Naming Convention

| Flag Name | Environment Variable |
|-----------|---------------------|
| `env` | `INVOWK_FLAG_ENV` |
| `dry-run` | `INVOWK_FLAG_DRY_RUN` |
| `output-file` | `INVOWK_FLAG_OUTPUT_FILE` |
| `retryCount` | `INVOWK_FLAG_RETRYCOUNT` |

Hyphens become underscores, uppercase.

## Real-World Examples

### Build Command

```cue
{
    name: "build"
    description: "Build the application"
    flags: [
        {name: "mode", description: "Build mode", validation: "^(debug|release)$", default_value: "debug"},
        {name: "output", description: "Output directory", short: "o", default_value: "./build"},
        {name: "verbose", description: "Verbose output", type: "bool", short: "v"},
        {name: "parallel", description: "Parallel jobs", type: "int", short: "j", default_value: "4"},
    ]
    implementations: [{
        script: """
            mkdir -p "$INVOWK_FLAG_OUTPUT"
            
            VERBOSE=""
            if [ "$INVOWK_FLAG_VERBOSE" = "true" ]; then
                VERBOSE="-v"
            fi
            
            go build $VERBOSE -o "$INVOWK_FLAG_OUTPUT/app" ./...
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Deploy Command

```cue
{
    name: "deploy"
    description: "Deploy to cloud"
    flags: [
        {
            name: "env"
            description: "Target environment"
            short: "e"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "version"
            description: "Version to deploy"
            short: "v"
            validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
        },
        {
            name: "dry-run"
            description: "Simulate deployment"
            type: "bool"
            short: "n"
            default_value: "false"
        },
        {
            name: "timeout"
            description: "Deployment timeout (seconds)"
            type: "int"
            default_value: "300"
        }
    ]
    implementations: [{
        script: """
            echo "Deploying version ${INVOWK_FLAG_VERSION:-latest} to $INVOWK_FLAG_ENV"
            
            ARGS="--timeout=$INVOWK_FLAG_TIMEOUT"
            if [ "$INVOWK_FLAG_DRY_RUN" = "true" ]; then
                ARGS="$ARGS --dry-run"
            fi
            
            ./scripts/deploy.sh "$INVOWK_FLAG_ENV" $ARGS
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Flag Syntax Variations

All these work:

```bash
# Equals sign
--output=./dist

# Space separator
--output ./dist

# Short with equals
-o=./dist

# Short with value
-o ./dist

# Boolean toggle (enables)
--verbose
-v

# Boolean explicit
--verbose=true
--verbose=false
```

## Reserved Flags

Don't use these names - they're reserved by Invowk:

| Flag | Short | Description |
|------|-------|-------------|
| `env-file` | `e` | Load environment from file |
| `env-var` | `E` | Set environment variable |
| `help` | `h` | Show help |
| `runtime` | - | Override runtime |

## Best Practices

1. **Use descriptive names**: `--output-dir` not `--od`
2. **Provide defaults when sensible**: Reduce required inputs
3. **Add validation for constrained values**: Fail fast on invalid input
4. **Use short aliases for common flags**: `-v`, `-o`, `-f`
5. **Boolean flags should default to false**: Opt-in behavior

## Next Steps

- [Positional Arguments](./positional-arguments) - For ordered, required values
- [Environment](../environment/overview) - Configure command environment
