---
sidebar_position: 3
---

# Virtual Runtime

The **virtual** runtime uses Invowk's built-in POSIX-compatible shell interpreter (powered by [mvdan/sh](https://github.com/mvdan/sh)). It provides consistent shell behavior across all platforms without requiring an external shell.

## How It Works

When you run a command with the virtual runtime, Invowk:

1. Parses the script using the built-in shell parser
2. Executes it in an embedded POSIX-like environment
3. Provides core utilities (echo, test, etc.) built-in

## Basic Usage

```cue
{
    name: "build"
    implementations: [{
        script: """
            echo "Building..."
            go build -o bin/app ./...
            echo "Done!"
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

```bash
invowk cmd myproject build --runtime virtual
```

## Cross-Platform Consistency

The virtual runtime behaves identically on Linux, macOS, and Windows:

```cue
{
    name: "setup"
    implementations: [{
        script: """
            # This works the same everywhere!
            if [ -d "node_modules" ]; then
                echo "Dependencies already installed"
            else
                echo "Installing dependencies..."
                npm install
            fi
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

No more "works on my machine" for shell scripts!

## Built-in Utilities

The virtual shell includes core POSIX utilities:

| Utility | Description |
|---------|-------------|
| `echo` | Print text |
| `printf` | Formatted output |
| `test` / `[` | Conditionals |
| `true` / `false` | Exit with 0/1 |
| `pwd` | Print working directory |
| `cd` | Change directory |
| `read` | Read input |
| `export` | Set environment variables |

### Extended Utilities (u-root)

When enabled in config, additional utilities are available:

```cue
// In your config file
virtual_shell: {
    enable_uroot_utils: true
}
```

This adds utilities like:
- `cat`, `head`, `tail`
- `grep`, `sed`, `awk`
- `ls`, `cp`, `mv`, `rm`
- `mkdir`, `rmdir`
- And many more

## POSIX Shell Features

The virtual shell supports standard POSIX constructs:

### Variables

```cue
script: """
    NAME="World"
    echo "Hello, $NAME!"
    
    # Parameter expansion
    echo "${NAME:-default}"
    echo "${#NAME}"  # Length
    """
```

### Conditionals

```cue
script: """
    if [ "$ENV" = "production" ]; then
        echo "Production mode"
    elif [ "$ENV" = "staging" ]; then
        echo "Staging mode"
    else
        echo "Development mode"
    fi
    """
```

### Loops

```cue
script: """
    # For loop
    for file in *.go; do
        echo "Processing $file"
    done
    
    # While loop
    count=0
    while [ $count -lt 5 ]; do
        echo "Count: $count"
        count=$((count + 1))
    done
    """
```

### Functions

```cue
script: """
    greet() {
        echo "Hello, $1!"
    }
    
    greet "World"
    greet "Invowk"
    """
```

### Subshells and Command Substitution

```cue
script: """
    # Command substitution
    current_date=$(date +%Y-%m-%d)
    echo "Today is $current_date"
    
    # Subshell
    (cd /tmp && echo "In temp: $(pwd)")
    echo "Still in: $(pwd)"
    """
```

## Calling External Commands

The virtual shell can call external commands installed on your system:

```cue
script: """
    # Calls the real 'go' binary
    go version
    
    # Calls the real 'git' binary
    git status
    """
```

External commands are found using the system's PATH.

## Environment Variables

Environment variables work the same as in native:

```cue
{
    name: "build"
    env: {
        vars: {
            BUILD_MODE: "release"
        }
    }
    implementations: [{
        script: """
            echo "Building in $BUILD_MODE mode"
            go build -ldflags="-s -w" ./...
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

## Flags and Arguments

Access flags and arguments the same way:

```cue
{
    name: "greet"
    args: [{name: "name", default_value: "World"}]
    implementations: [{
        script: """
            # Using environment variable
            echo "Hello, $INVOWK_ARG_NAME!"
            
            # Or positional parameter
            echo "Hello, $1!"
            """
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

## Limitations

### No Interpreter Support

The virtual runtime **cannot** use non-shell interpreters:

```cue
// This will NOT work with virtual runtime!
{
    name: "bad-example"
    implementations: [{
        script: """
            #!/usr/bin/env python3
            print("This won't work!")
            """
        target: {
            runtimes: [{
                name: "virtual"
                interpreter: "python3"  // ERROR: Not supported
            }]
        }
    }]
}
```

For Python, Ruby, or other interpreters, use the native or container runtime.

### Bash-Specific Features

Some bash-specific features are not available:

```cue
// These won't work in virtual runtime:
script: """
    # Bash arrays (use $@ instead)
    declare -a arr=(1 2 3)  # Not supported
    
    # Bash-specific parameter expansion
    ${var^^}  # Uppercase - not supported
    ${var,,}  # Lowercase - not supported
    
    # Process substitution
    diff <(cmd1) <(cmd2)  # Not supported
    """
```

Stick to POSIX-compatible constructs for virtual runtime.

## Dependency Validation

Dependencies are validated against the virtual shell's capabilities:

```cue
{
    name: "build"
    depends_on: {
        tools: [
            // These will be checked in the virtual shell environment
            {alternatives: ["go"]},
            {alternatives: ["git"]}
        ]
    }
    implementations: [{
        script: "go build ./..."
        target: {
            runtimes: [{name: "virtual"}]
        }
    }]
}
```

## Advantages

- **Consistency**: Same behavior on Linux, macOS, and Windows
- **No shell dependency**: Works even if system shell is unavailable
- **Portability**: Scripts work across all platforms
- **Built-in utilities**: Core utilities always available
- **Faster startup**: No shell process to spawn

## When to Use Virtual

- **Cross-platform scripts**: When the same script must work everywhere
- **CI/CD pipelines**: Consistent behavior across build agents
- **Simple shell scripts**: When you don't need bash-specific features
- **Embedded environments**: When external shells aren't available

## Configuration

Configure the virtual shell in your Invowk config file:

```cue
// ~/.config/invowk/config.cue (Linux)
// ~/Library/Application Support/invowk/config.cue (macOS)
// %APPDATA%\invowk\config.cue (Windows)

virtual_shell: {
    // Enable additional utilities from u-root
    enable_uroot_utils: true
}
```

## Next Steps

- [Native Runtime](./native) - For full shell access
- [Container Runtime](./container) - For isolated execution
