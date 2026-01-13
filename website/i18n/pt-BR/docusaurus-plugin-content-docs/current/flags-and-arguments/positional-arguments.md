---
sidebar_position: 3
---

# Positional Arguments

Positional arguments are values passed by position after the command name. They're ideal for required inputs where order is natural (like `source` and `destination`).

## Defining Arguments

```cue
{
    name: "copy"
    args: [
        {
            name: "source"
            description: "Source file or directory"
            required: true
        },
        {
            name: "destination"
            description: "Destination path"
            required: true
        }
    ]
    implementations: [...]
}
```

Usage:
```bash
invowk cmd myproject copy ./src ./dest
```

## Argument Properties

| Property | Required | Description |
|----------|----------|-------------|
| `name` | Yes | Argument name (alphanumeric, hyphens, underscores) |
| `description` | Yes | Help text |
| `type` | No | `string`, `int`, `float` (default: `string`) |
| `default_value` | No | Default if not provided |
| `required` | No | Must be provided (can't have default) |
| `validation` | No | Regex pattern for value |
| `variadic` | No | Accept multiple values (last arg only) |

## Types

### String (Default)

```cue
{name: "filename", description: "File to process", type: "string"}
```

### Integer

```cue
{name: "count", description: "Number of items", type: "int", default_value: "10"}
```

```bash
invowk cmd myproject generate 5
```

### Float

```cue
{name: "ratio", description: "Scaling ratio", type: "float", default_value: "1.0"}
```

```bash
invowk cmd myproject scale 0.5
```

Note: Boolean type is **not supported** for arguments. Use flags for boolean options.

## Required vs Optional

### Required Arguments

```cue
args: [
    {name: "input", description: "Input file", required: true},
    {name: "output", description: "Output file", required: true},
]
```

```bash
# Error: missing required argument
invowk cmd myproject convert input.txt
# Error: argument 'output' is required

# Success
invowk cmd myproject convert input.txt output.txt
```

### Optional Arguments

```cue
args: [
    {name: "input", description: "Input file", required: true},
    {name: "format", description: "Output format", default_value: "json"},
]
```

```bash
# Uses default format (json)
invowk cmd myproject parse input.txt

# Override format
invowk cmd myproject parse input.txt yaml
```

### Ordering Rule

Required arguments must come before optional arguments:

```cue
// Good
args: [
    {name: "input", required: true},      // Required first
    {name: "output", required: true},     // Required second
    {name: "format", default_value: "json"}, // Optional last
]

// Bad - will cause validation error
args: [
    {name: "format", default_value: "json"}, // Optional can't come first
    {name: "input", required: true},
]
```

## Variadic Arguments

The last argument can accept multiple values:

```cue
{
    name: "process"
    args: [
        {name: "output", description: "Output file", required: true},
        {name: "inputs", description: "Input files", variadic: true},
    ]
    implementations: [{
        script: """
            echo "Output: $INVOWK_ARG_OUTPUT"
            echo "Inputs: $INVOWK_ARG_INPUTS"
            echo "Count: $INVOWK_ARG_INPUTS_COUNT"
            
            for i in $(seq 1 $INVOWK_ARG_INPUTS_COUNT); do
                eval "file=\$INVOWK_ARG_INPUTS_$i"
                echo "Processing: $file"
            done
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

```bash
invowk cmd myproject process out.txt a.txt b.txt c.txt
# Output: out.txt
# Inputs: a.txt b.txt c.txt
# Count: 3
# Processing: a.txt
# Processing: b.txt
# Processing: c.txt
```

### Variadic Environment Variables

| Variable | Description |
|----------|-------------|
| `INVOWK_ARG_INPUTS` | Space-joined values |
| `INVOWK_ARG_INPUTS_COUNT` | Number of values |
| `INVOWK_ARG_INPUTS_1` | First value |
| `INVOWK_ARG_INPUTS_2` | Second value |
| ... | And so on |

## Validation Patterns

Validate argument values with regex:

```cue
args: [
    {
        name: "environment"
        description: "Target environment"
        required: true
        validation: "^(dev|staging|prod)$"
    },
    {
        name: "version"
        description: "Version number"
        validation: "^[0-9]+\\.[0-9]+\\.[0-9]+$"
    }
]
```

```bash
# Valid
invowk cmd myproject deploy prod 1.2.3

# Invalid
invowk cmd myproject deploy production
# Error: argument 'environment' value 'production' does not match pattern '^(dev|staging|prod)$'
```

## Accessing in Scripts

### Environment Variables

```cue
{
    name: "greet"
    args: [
        {name: "first-name", required: true},
        {name: "last-name", default_value: "User"},
    ]
    implementations: [{
        script: """
            echo "Hello, $INVOWK_ARG_FIRST_NAME $INVOWK_ARG_LAST_NAME!"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Naming Convention

| Argument Name | Environment Variable |
|---------------|---------------------|
| `name` | `INVOWK_ARG_NAME` |
| `file-path` | `INVOWK_ARG_FILE_PATH` |
| `outputDir` | `INVOWK_ARG_OUTPUTDIR` |

### Shell Positional Parameters

Arguments are also available as `$1`, `$2`, etc.:

```cue
{
    name: "copy"
    args: [
        {name: "source", required: true},
        {name: "dest", required: true},
    ]
    implementations: [{
        script: """
            # Using environment variables
            cp "$INVOWK_ARG_SOURCE" "$INVOWK_ARG_DEST"
            
            # Or positional parameters
            cp "$1" "$2"
            
            # All arguments
            echo "Args: $@"
            echo "Count: $#"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Shell Compatibility

| Shell | Positional Access |
|-------|-------------------|
| bash, sh, zsh | `$1`, `$2`, `$@`, `$#` |
| PowerShell | `$args[0]`, `$args[1]` |
| virtual runtime | `$1`, `$2`, `$@`, `$#` |
| container | `$1`, `$2`, `$@`, `$#` |

## Real-World Examples

### File Processing

```cue
{
    name: "convert"
    description: "Convert file format"
    args: [
        {
            name: "input"
            description: "Input file"
            required: true
        },
        {
            name: "output"
            description: "Output file"
            required: true
        },
        {
            name: "format"
            description: "Output format"
            default_value: "json"
            validation: "^(json|yaml|toml|xml)$"
        }
    ]
    implementations: [{
        script: """
            echo "Converting $1 to $2 as $3"
            ./converter --input="$1" --output="$2" --format="$3"
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Multi-File Operation

```cue
{
    name: "compress"
    description: "Compress files into archive"
    args: [
        {
            name: "archive"
            description: "Output archive name"
            required: true
            validation: "\\.(zip|tar\\.gz|tgz)$"
        },
        {
            name: "files"
            description: "Files to compress"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            if [ -z "$INVOWK_ARG_FILES" ]; then
                echo "No files specified!"
                exit 1
            fi
            
            # Use the space-separated list
            tar -czvf "$INVOWK_ARG_ARCHIVE" $INVOWK_ARG_FILES
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

### Deployment

```cue
{
    name: "deploy"
    description: "Deploy services"
    args: [
        {
            name: "env"
            description: "Target environment"
            required: true
            validation: "^(dev|staging|prod)$"
        },
        {
            name: "replicas"
            description: "Number of replicas"
            type: "int"
            default_value: "1"
        },
        {
            name: "services"
            description: "Services to deploy"
            variadic: true
        }
    ]
    implementations: [{
        script: """
            echo "Deploying to $INVOWK_ARG_ENV with $INVOWK_ARG_REPLICAS replicas"
            
            if [ -n "$INVOWK_ARG_SERVICES" ]; then
                for i in $(seq 1 $INVOWK_ARG_SERVICES_COUNT); do
                    eval "service=\$INVOWK_ARG_SERVICES_$i"
                    echo "Deploying $service..."
                    kubectl scale deployment/$service --replicas=$INVOWK_ARG_REPLICAS
                done
            else
                echo "Deploying all services..."
                kubectl scale deployment --all --replicas=$INVOWK_ARG_REPLICAS
            fi
            """
        target: {runtimes: [{name: "native"}]}
    }]
}
```

## Mixing with Flags

Flags can appear anywhere; arguments are positional:

```bash
# All equivalent
invowk cmd myproject deploy prod 3 --dry-run
invowk cmd myproject deploy --dry-run prod 3
invowk cmd myproject deploy prod --dry-run 3
```

Arguments are extracted in order, regardless of flag positions.

## Arguments vs Subcommands

A command cannot have both arguments and subcommands. If a command has subcommands, arguments are ignored:

```
Warning: command has both args and subcommands!

Command 'deploy' defines positional arguments but also has subcommands.
Subcommands take precedence; positional arguments will be ignored.
```

Choose one approach:
- Use arguments for simple commands
- Use subcommands for complex command hierarchies

## Best Practices

1. **Required args first**: Follow ordering rules
2. **Use meaningful names**: `source` and `dest` not `arg1` and `arg2`
3. **Validate when possible**: Catch errors early
4. **Document with descriptions**: Help users understand
5. **Prefer flags for optional values**: Easier to understand

## Next Steps

- [Flags](./flags) - For named optional values
- [Environment](../environment/overview) - Configure command environment
