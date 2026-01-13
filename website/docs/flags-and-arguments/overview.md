---
sidebar_position: 1
---

# Flags and Arguments Overview

Commands can accept user input through **flags** (named options) and **arguments** (positional values). Both are passed to your scripts as environment variables.

## Quick Comparison

| Feature | Flags | Arguments |
|---------|-------|-----------|
| Syntax | `--name=value` or `--name value` | Positional: `value1 value2` |
| Order | Any order | Must follow position order |
| Boolean | Supported (`--verbose`) | Not supported |
| Named access | `INVOWK_FLAG_NAME` | `INVOWK_ARG_NAME` |
| Multiple values | No | Yes (variadic) |

## Example Command

```cue
{
    name: "deploy"
    description: "Deploy to an environment"
    
    // Flags - named options
    flags: [
        {name: "dry-run", type: "bool", default_value: "false"},
        {name: "replicas", type: "int", default_value: "1"},
    ]
    
    // Arguments - positional values
    args: [
        {name: "environment", required: true},
        {name: "services", variadic: true},
    ]
    
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENVIRONMENT"
            echo "Replicas: $INVOWK_FLAG_REPLICAS"
            echo "Dry run: $INVOWK_FLAG_DRY_RUN"
            echo "Services: $INVOWK_ARG_SERVICES"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

Usage:
```bash
invowk cmd myproject deploy production api web --dry-run --replicas=3
```

## Flags

Flags are named options with `--name=value` syntax:

```cue
flags: [
    {name: "verbose", type: "bool", short: "v"},
    {name: "output", type: "string", short: "o", default_value: "./dist"},
    {name: "count", type: "int", default_value: "1"},
]
```

```bash
# Long form
invowk cmd myproject build --verbose --output=./build --count=5

# Short form
invowk cmd myproject build -v -o=./build
```

Key features:
- Support short aliases (`-v` for `--verbose`)
- Typed values (string, bool, int, float)
- Optional or required
- Default values
- Regex validation

See [Flags](./flags) for details.

## Arguments

Arguments are positional values after the command name:

```cue
args: [
    {name: "source", required: true},
    {name: "destination", default_value: "./output"},
    {name: "files", variadic: true},
]
```

```bash
invowk cmd myproject copy ./src ./dest file1.txt file2.txt
```

Key features:
- Position-based (order matters)
- Required or optional
- Default values
- Last argument can be variadic (accept multiple values)
- Typed values (string, int, float - but not bool)

See [Positional Arguments](./positional-arguments) for details.

## Environment Variable Access

Both flags and arguments are available as environment variables:

### Flag Variables

Prefix: `INVOWK_FLAG_`

| Flag Name | Environment Variable |
|-----------|---------------------|
| `verbose` | `INVOWK_FLAG_VERBOSE` |
| `dry-run` | `INVOWK_FLAG_DRY_RUN` |
| `output-file` | `INVOWK_FLAG_OUTPUT_FILE` |

### Argument Variables

Prefix: `INVOWK_ARG_`

| Argument Name | Environment Variable |
|---------------|---------------------|
| `environment` | `INVOWK_ARG_ENVIRONMENT` |
| `file-path` | `INVOWK_ARG_FILE_PATH` |

### Variadic Arguments

Additional variables for multiple values:

| Variable | Description |
|----------|-------------|
| `INVOWK_ARG_FILES` | Space-separated values |
| `INVOWK_ARG_FILES_COUNT` | Number of values |
| `INVOWK_ARG_FILES_1` | First value |
| `INVOWK_ARG_FILES_2` | Second value |

## Shell Positional Parameters

Arguments are also available as shell positional parameters:

```cue
{
    name: "greet"
    args: [
        {name: "first-name"},
        {name: "last-name"},
    ]
    implementations: [{
        script: """
            # Using environment variables
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            
            # Or using positional parameters
            echo "Hello, $1 $2!"
            
            # All arguments
            echo "All: $@"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Mixing Flags and Arguments

Flags can appear anywhere on the command line:

```bash
# All equivalent
invowk cmd myproject deploy production --dry-run api web
invowk cmd myproject deploy --dry-run production api web
invowk cmd myproject deploy production api web --dry-run
```

## Reserved Flags

Some flag names are reserved by Invowk:

- `env-file` / `-e` - Load environment from file
- `env-var` / `-E` - Set environment variable

Don't use these names for your flags.

## Help Output

Flags and arguments appear in command help:

```bash
invowk cmd myproject deploy --help
```

```
Usage:
  invowk cmd myproject deploy <environment> [services]... [flags]

Arguments:
  environment          (required) - Target environment
  services             (optional) (variadic) - Services to deploy

Flags:
      --dry-run          Perform a dry run (default: false)
  -n, --replicas int     Number of replicas (default: 1)
  -h, --help             help for deploy
```

## Next Steps

- [Flags](./flags) - Named options with `--flag=value` syntax
- [Positional Arguments](./positional-arguments) - Value-based input
